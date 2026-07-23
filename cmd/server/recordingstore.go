package main

import (
	"context"
	"database/sql"
	"time"
)

type recordingRow struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	CallID    string    `json:"call_id"`
	Peer      string    `json:"peer"`
	Direction string    `json:"direction"`
	Duration  int       `json:"duration"`
	FilePath  string    `json:"file_path"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}

type recordingStore struct{ db *sql.DB }

func newRecordingStore(ctx context.Context, db *sql.DB) (*recordingStore, error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS recordings (
		id         TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		call_id    TEXT NOT NULL,
		peer       TEXT NOT NULL DEFAULT '',
		direction  TEXT NOT NULL DEFAULT 'outbound',
		duration   INTEGER NOT NULL DEFAULT 0,
		file_path  TEXT NOT NULL,
		file_size  INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	if err != nil {
		return nil, err
	}
	_, err = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_recordings_session ON recordings (session_id)`)
	if err != nil {
		return nil, err
	}
	return &recordingStore{db: db}, nil
}

func (s *recordingStore) insert(ctx context.Context, r *recordingRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO recordings (id, session_id, call_id, peer, direction, duration, file_path, file_size, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		r.ID, r.SessionID, r.CallID, r.Peer, r.Direction, r.Duration, r.FilePath, r.FileSize, r.CreatedAt)
	return err
}

func (s *recordingStore) listBySession(ctx context.Context, sessionID string) ([]recordingRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, call_id, peer, direction, duration, file_path, file_size, created_at
		 FROM recordings WHERE session_id = $1 ORDER BY created_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []recordingRow
	for rows.Next() {
		var r recordingRow
		if err := rows.Scan(&r.ID, &r.SessionID, &r.CallID, &r.Peer, &r.Direction, &r.Duration, &r.FilePath, &r.FileSize, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *recordingStore) listAll(ctx context.Context) ([]recordingRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, call_id, peer, direction, duration, file_path, file_size, created_at
		 FROM recordings ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []recordingRow
	for rows.Next() {
		var r recordingRow
		if err := rows.Scan(&r.ID, &r.SessionID, &r.CallID, &r.Peer, &r.Direction, &r.Duration, &r.FilePath, &r.FileSize, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *recordingStore) get(ctx context.Context, id string) (*recordingRow, error) {
	var r recordingRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, session_id, call_id, peer, direction, duration, file_path, file_size, created_at
		 FROM recordings WHERE id = $1`, id).Scan(
		&r.ID, &r.SessionID, &r.CallID, &r.Peer, &r.Direction, &r.Duration, &r.FilePath, &r.FileSize, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
