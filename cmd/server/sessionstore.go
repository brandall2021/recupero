package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	sessionStatusPending      = "pending"
	sessionStatusPairing      = "pairing"
	sessionStatusConnected    = "connected"
	sessionStatusDisconnected = "disconnected"
	sessionStatusDisabled     = "disabled"

	tokenPrefix = "wcs_live_"
)

var (
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionLimitReached = errors.New("session_limit_reached")
	ErrSessionNameTaken    = errors.New("session name already in use")
	ErrSessionDisabled     = errors.New("session disabled")
	ErrClientNotActive     = errors.New("client not active")
)

type sessionRow struct {
	ID              string
	ClientID        string
	Name            string
	JID             string
	TokenHash       string
	Status          string
	PhoneNumber     string
	Webhook         string
	TokenCreatedAt  *time.Time
	TokenLastUsedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type sessionStore struct{ db *sql.DB }

func newSessionStore(ctx context.Context, db *sql.DB) (*sessionStore, error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS sessions (
		id          TEXT PRIMARY KEY,
		client_id   UUID NOT NULL,
		name        TEXT NOT NULL,
		jid         TEXT,
		token_hash  TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'pending',
		phone_number TEXT NOT NULL DEFAULT '',
		webhook_url TEXT,
		token_created_at TIMESTAMPTZ,
		token_last_used_at TIMESTAMPTZ,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT sessions_status_check CHECK (status IN ('pending','pairing','connected','disconnected','disabled')),
		CONSTRAINT sessions_client_fk FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE
	)`)
	if err != nil {
		return nil, err
	}
	// Migration: add columns if missing (for existing installs)
	_, _ = db.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS client_id UUID`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS token_hash TEXT NOT NULL DEFAULT ''`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending'`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS phone_number TEXT NOT NULL DEFAULT ''`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS token_created_at TIMESTAMPTZ`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS token_last_used_at TIMESTAMPTZ`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`)
	_, _ = db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS uq_sessions_client_name ON sessions (client_id, name) WHERE client_id IS NOT NULL`)
	return &sessionStore{db: db}, nil
}

const sessionCols = `id, client_id, name, COALESCE(jid, ''), COALESCE(token_hash, ''), status, COALESCE(phone_number, ''), COALESCE(webhook_url, ''), token_created_at, token_last_used_at, created_at, updated_at`

func scanSessionRow(sc interface{ Scan(...any) error }) (sessionRow, error) {
	var r sessionRow
	var clientID, jid, tokenHash, phone, webhook sql.NullString
	var tokenCreated, tokenLastUsed sql.NullTime
	err := sc.Scan(&r.ID, &clientID, &r.Name, &jid, &tokenHash, &r.Status, &phone, &webhook, &tokenCreated, &tokenLastUsed, &r.CreatedAt, &r.UpdatedAt)
	r.ClientID = clientID.String
	r.JID = jid.String
	r.TokenHash = tokenHash.String
	r.PhoneNumber = phone.String
	r.Webhook = webhook.String
	if tokenCreated.Valid {
		r.TokenCreatedAt = &tokenCreated.Time
	}
	if tokenLastUsed.Valid {
		r.TokenLastUsedAt = &tokenLastUsed.Time
	}
	return r, err
}

func (s *sessionStore) list(ctx context.Context) ([]sessionRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+sessionCols+` FROM sessions ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sessionRow
	for rows.Next() {
		r, err := scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *sessionStore) listByClient(ctx context.Context, clientID string) ([]sessionRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+sessionCols+` FROM sessions WHERE client_id = $1 ORDER BY created_at`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sessionRow
	for rows.Next() {
		r, err := scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *sessionStore) get(ctx context.Context, id string) (*sessionRow, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+sessionCols+` FROM sessions WHERE id = $1`, id)
	r, err := scanSessionRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &r, nil
}

// createWithLimit inserts a session for a client inside a transaction,
// enforcing the client's max_sessions (counting non-disabled sessions). On
// success it returns the session id and the plaintext token (shown only once).
func (s *sessionStore) createWithLimit(ctx context.Context, clientID, name string) (string, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	var maxSessions int
	err = tx.QueryRowContext(ctx,
		`SELECT max_sessions FROM clients WHERE id = $1 FOR UPDATE`, clientID).Scan(&maxSessions)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrClientNotFound
		}
		return "", "", err
	}

	var used int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE client_id = $1 AND status <> 'disabled'`, clientID).Scan(&used)
	if err != nil {
		return "", "", err
	}
	if used >= maxSessions {
		return "", "", ErrSessionLimitReached
	}

	id := uuid.New().String()
	token, tokenHash := newSessionToken()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sessions (id, client_id, name, jid, token_hash, status, webhook_url, token_created_at, created_at, updated_at)
		 VALUES ($1, $2, $3, NULL, $4, 'pending', NULL, $5, $6, $6)`,
		id, clientID, name, tokenHash, now, now); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return "", "", ErrSessionNameTaken
		}
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return id, token, nil
}

func (s *sessionStore) setJID(ctx context.Context, id, jid string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET jid = $1, updated_at = NOW() WHERE id = $2`, jid, id)
	return err
}

func (s *sessionStore) setPhoneNumber(ctx context.Context, id, phone string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET phone_number = $1, updated_at = NOW() WHERE id = $2`, phone, id)
	return err
}

func (s *sessionStore) setStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}

// rotateToken replaces the session token hash and returns the new plaintext
// token (shown only once). The previous token becomes invalid immediately.
func (s *sessionStore) rotateToken(ctx context.Context, id string) (string, error) {
	token, tokenHash := newSessionToken()
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET token_hash = $1, token_created_at = $2, updated_at = $2 WHERE id = $3`,
		tokenHash, now, id)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", ErrSessionNotFound
	}
	return token, nil
}

func (s *sessionStore) touchToken(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET token_last_used_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *sessionStore) setWebhook(ctx context.Context, id, url string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET webhook_url = $1, updated_at = NOW() WHERE id = $2`, url, id)
	return err
}

func (s *sessionStore) delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

func newSessionID() string {
	return uuid.New().String()
}

// newSessionToken generates a crypto-secure token with the wcs_live_ prefix and
// returns it alongside its SHA-256 hash (only the hash is persisted).
func newSessionToken() (token, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token = tokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func tokenPrefixOf(token string) string {
	if len(token) <= 8 {
		return ""
	}
	return token[:8]
}
