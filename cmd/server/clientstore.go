package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrClientNotFound   = errors.New("client not found")
	ErrLimitBelowUsage  = errors.New("limit_below_current_usage")
	ErrSlugTaken        = errors.New("slug already in use")
	ErrClientEmailTaken = errors.New("email already in use")
)

type clientRow struct {
	ID           string
	Name         string
	Slug         string
	Status       string
	MaxSessions  int
	UsedSessions int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type clientStore struct{ db *sql.DB }

func newClientStore(ctx context.Context, db *sql.DB) (*clientStore, error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS clients (
		id           UUID PRIMARY KEY,
		name         TEXT NOT NULL,
		slug         TEXT NOT NULL UNIQUE,
		status       TEXT NOT NULL DEFAULT 'active',
		max_sessions INTEGER NOT NULL DEFAULT 1,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

		CONSTRAINT clients_status_check
			CHECK (status IN ('active', 'suspended', 'disabled')),

		CONSTRAINT clients_max_sessions_check
			CHECK (max_sessions >= 0)
	)`)
	if err != nil {
		return nil, err
	}
	return &clientStore{db: db}, nil
}

func (s *clientStore) list(ctx context.Context) ([]clientRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.name, c.slug, c.status, c.max_sessions, c.created_at, c.updated_at,
		        (SELECT COUNT(*) FROM sessions x WHERE x.client_id = c.id AND x.status <> 'disabled') AS used
		 FROM clients c ORDER BY c.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []clientRow
	for rows.Next() {
		var r clientRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Slug, &r.Status, &r.MaxSessions, &r.CreatedAt, &r.UpdatedAt, &r.UsedSessions); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *clientStore) get(ctx context.Context, id string) (*clientRow, error) {
	var r clientRow
	err := s.db.QueryRowContext(ctx,
		`SELECT c.id, c.name, c.slug, c.status, c.max_sessions, c.created_at, c.updated_at,
		        (SELECT COUNT(*) FROM sessions x WHERE x.client_id = c.id AND x.status <> 'disabled') AS used
		 FROM clients c WHERE c.id = $1`, id).
		Scan(&r.ID, &r.Name, &r.Slug, &r.Status, &r.MaxSessions, &r.CreatedAt, &r.UpdatedAt, &r.UsedSessions)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrClientNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *clientStore) getBySlug(ctx context.Context, slug string) (*clientRow, error) {
	var r clientRow
	err := s.db.QueryRowContext(ctx,
		`SELECT c.id, c.name, c.slug, c.status, c.max_sessions, c.created_at, c.updated_at,
		        (SELECT COUNT(*) FROM sessions x WHERE x.client_id = c.id AND x.status <> 'disabled') AS used
		 FROM clients c WHERE c.slug = $1`, slug).
		Scan(&r.ID, &r.Name, &r.Slug, &r.Status, &r.MaxSessions, &r.CreatedAt, &r.UpdatedAt, &r.UsedSessions)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrClientNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *clientStore) insert(ctx context.Context, id, name, slug string, maxSessions int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO clients (id, name, slug, max_sessions) VALUES ($1, $2, $3, $4)`,
		id, name, slug, maxSessions)
	return err
}

func (s *clientStore) update(ctx context.Context, id, name string, maxSessions int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE clients SET name = $1, max_sessions = $2, updated_at = NOW() WHERE id = $3`,
		name, maxSessions, id)
	return err
}

func (s *clientStore) setStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE clients SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}

func (s *clientStore) delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM clients WHERE id = $1`, id)
	return err
}

// createWithAdmin inserts a client and its client_admin user in a single
// transaction. The client is created active. Returns the client id.
func (s *clientStore) createWithAdmin(ctx context.Context, name, slug string, maxSessions int, adminEmail, adminName, adminPassword string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	clientID := uuid.New().String()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO clients (id, name, slug, max_sessions) VALUES ($1, $2, $3, $4)`,
		clientID, name, slug, maxSessions); err != nil {
		if isUniqueViolation(err) {
			return "", ErrSlugTaken
		}
		return "", err
	}
	if err := insertAdminUserTx(ctx, tx, adminEmail, adminName, adminPassword, clientID); err != nil {
		if isUniqueViolation(err) {
			return "", ErrClientEmailTaken
		}
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return clientID, nil
}

// updateLimitsGuarded raises/lowers max_sessions in a transaction, refusing to
// set it below the number of currently enabled sessions. On refusal it returns
// ErrLimitBelowUsage.
func (s *clientStore) updateLimitsGuarded(ctx context.Context, clientID string, newMax int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Lock the client row to serialize concurrent limit changes on it, then
	// count enabled sessions (FOR UPDATE is not allowed on aggregate queries).
	var unused int
	if err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM clients WHERE id = $1 FOR UPDATE`, clientID).Scan(&unused); err != nil {
		return err
	}
	var used int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE client_id = $1 AND status <> 'disabled'`,
		clientID).Scan(&used)
	if err != nil {
		return err
	}
	if newMax < used {
		return ErrLimitBelowUsage
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE clients SET max_sessions = $1, updated_at = NOW() WHERE id = $2`, newMax, clientID); err != nil {
		return err
	}
	return tx.Commit()
}

func insertAdminUserTx(ctx context.Context, tx *sql.Tx, email, name, password, clientID string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO users (email, name, password, role, client_id) VALUES ($1, $2, $3, $4, $5)`,
		strings.TrimSpace(strings.ToLower(email)), strings.TrimSpace(name), string(hash), roleClientAdmin, clientID)
	if err != nil {
		return err
	}
	return nil
}

func (s *clientStore) exists(ctx context.Context, id string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM clients WHERE id = $1`, id).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isUniqueViolation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint") || strings.Contains(msg, "SQLSTATE 23505")
}

func slugify(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_', r == '.':
			b.WriteByte('-')
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = uuid.NewString()[:8]
	}
	return out
}
