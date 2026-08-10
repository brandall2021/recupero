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
	ClientID  string    `json:"client_id"`
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
		client_id  TEXT NOT NULL DEFAULT '',
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
	_, _ = db.ExecContext(ctx, `ALTER TABLE recordings ADD COLUMN IF NOT EXISTS client_id TEXT NOT NULL DEFAULT ''`)
	_, err = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_recordings_session ON recordings (session_id)`)
	if err != nil {
		return nil, err
	}
	_, err = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_recordings_client ON recordings (client_id)`)
	if err != nil {
		return nil, err
	}
	return &recordingStore{db: db}, nil
}

func (s *recordingStore) insert(ctx context.Context, r *recordingRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO recordings (id, session_id, call_id, client_id, peer, direction, duration, file_path, file_size, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		r.ID, r.SessionID, r.CallID, r.ClientID, r.Peer, r.Direction, r.Duration, r.FilePath, r.FileSize, r.CreatedAt)
	return err
}

const recCols = `id, session_id, call_id, client_id, peer, direction, duration, file_path, file_size, created_at`

func scanRecording(sc interface{ Scan(...any) error }) (recordingRow, error) {
	var r recordingRow
	err := sc.Scan(&r.ID, &r.SessionID, &r.CallID, &r.ClientID, &r.Peer, &r.Direction, &r.Duration, &r.FilePath, &r.FileSize, &r.CreatedAt)
	return r, err
}

func (s *recordingStore) listBySession(ctx context.Context, sessionID, clientID string) ([]recordingRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+recCols+` FROM recordings WHERE session_id = $1 AND (client_id = $2 OR $2 = '')
		 ORDER BY created_at DESC`, sessionID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []recordingRow
	for rows.Next() {
		r, err := scanRecording(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *recordingStore) listAll(ctx context.Context) ([]recordingRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+recCols+` FROM recordings ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []recordingRow
	for rows.Next() {
		r, err := scanRecording(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *recordingStore) get(ctx context.Context, id string) (*recordingRow, error) {
	var r recordingRow
	err := s.db.QueryRowContext(ctx,
		`SELECT `+recCols+` FROM recordings WHERE id = $1`, id).Scan(
		&r.ID, &r.SessionID, &r.CallID, &r.ClientID, &r.Peer, &r.Direction, &r.Duration, &r.FilePath, &r.FileSize, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// getScoped returns a recording only when it belongs to the given session and
// client, enforcing tenant isolation.
func (s *recordingStore) getScoped(ctx context.Context, id, sessionID, clientID string) (*recordingRow, error) {
	var r recordingRow
	err := s.db.QueryRowContext(ctx,
		`SELECT `+recCols+` FROM recordings
		 WHERE id = $1 AND session_id = $2 AND (client_id = $3 OR $3 = '')`, id, sessionID, clientID).Scan(
		&r.ID, &r.SessionID, &r.CallID, &r.ClientID, &r.Peer, &r.Direction, &r.Duration, &r.FilePath, &r.FileSize, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
