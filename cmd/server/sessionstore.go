package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
)

type sessionRow struct {
	ID        string
	Name      string
	JID       string
	Token     string
	Webhook   string
}

type sessionStore struct{ db *sql.DB }

func newSessionStore(ctx context.Context, db *sql.DB) (*sessionStore, error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS sessions (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		jid         TEXT,
		token       TEXT,
		webhook_url TEXT
	)`)
	if err != nil {
		return nil, err
	}
	// Migration: add token/webhook_url columns if missing (for existing installs)
	_, _ = db.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS token TEXT`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS webhook_url TEXT`)
	return &sessionStore{db: db}, nil
}

func (s *sessionStore) list(ctx context.Context) ([]sessionRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, COALESCE(jid, ''), COALESCE(token, ''), COALESCE(webhook_url, '') FROM sessions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sessionRow
	for rows.Next() {
		var r sessionRow
		if err := rows.Scan(&r.ID, &r.Name, &r.JID, &r.Token, &r.Webhook); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *sessionStore) insert(ctx context.Context, id, name, token string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (id, name, jid, token) VALUES ($1, $2, NULL, $3)`, id, name, token)
	return err
}

func (s *sessionStore) setJID(ctx context.Context, id, jid string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET jid = $1 WHERE id = $2`, jid, id)
	return err
}

func (s *sessionStore) setToken(ctx context.Context, id, token string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET token = $1 WHERE id = $2`, token, id)
	return err
}

func (s *sessionStore) setWebhook(ctx context.Context, id, url string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET webhook_url = $1 WHERE id = $2`, url, id)
	return err
}

func (s *sessionStore) delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

func newSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func newSessionToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}
