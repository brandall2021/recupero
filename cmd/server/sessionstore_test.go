package main

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/wacalls_test?sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSessionStoreRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	// clean slate
	db.ExecContext(ctx, "DELETE FROM sessions")

	st, err := newSessionStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	id := newSessionID()
	if len(id) != 32 {
		t.Fatalf("session id should be 32 hex chars, got %d", len(id))
	}
	if err := st.insert(ctx, id, "Account A"); err != nil {
		t.Fatal(err)
	}

	rows, err := st.list(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != id || rows[0].Name != "Account A" || rows[0].JID != "" {
		t.Fatalf("unexpected rows after insert: %+v", rows)
	}

	if err := st.setJID(ctx, id, "5511999999999:1@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	rows, _ = st.list(ctx)
	if rows[0].JID != "5511999999999:1@s.whatsapp.net" {
		t.Fatalf("jid not persisted: %+v", rows[0])
	}

	if err := st.delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	rows, _ = st.list(ctx)
	if len(rows) != 0 {
		t.Fatalf("expected empty after delete, got %+v", rows)
	}
}
