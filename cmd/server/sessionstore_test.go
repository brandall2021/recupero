package main

import (
	"context"
	"database/sql"
	"os"
	"strings"
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

func TestTokenFormatAndHashing(t *testing.T) {
	token, hash := newSessionToken()
	if !strings.HasPrefix(token, tokenPrefix) {
		t.Fatalf("token should start with %q, got %q", tokenPrefix, token)
	}
	if len(token) < len(tokenPrefix)+16 {
		t.Fatal("token too short; should carry >= 128 bits of entropy")
	}
	if hash != hashToken(token) {
		t.Fatal("hashToken should be deterministic")
	}
	if hash == "" || hashToken(token+"x") == hash {
		t.Fatal("hash should not collide across tokens")
	}
}

func TestSessionStoreRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	// clean slate
	db.ExecContext(ctx, "DELETE FROM sessions")
	db.ExecContext(ctx, "DELETE FROM users")
	db.ExecContext(ctx, "DELETE FROM clients")

	if _, err := newAuthStore(ctx, db); err != nil {
		t.Fatal(err)
	}
	clients, err := newClientStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	store, err := newSessionStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	clientID, err := clients.createWithAdmin(ctx, "Acme", "acme", 5, "acme@test.com", "Acme Admin", "secret123")
	if err != nil {
		t.Fatal(err)
	}

	id, token, err := store.createWithLimit(ctx, clientID, "Account A")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "00000000") && len(id) != 36 {
		t.Fatalf("session id should be a UUID, got %q", id)
	}

	rows, err := store.list(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != id || rows[0].Name != "Account A" || rows[0].ClientID != clientID {
		t.Fatalf("unexpected rows after insert: %+v", rows)
	}
	// token must never be persisted in plaintext
	if rows[0].TokenHash == token || rows[0].TokenHash != hashToken(token) {
		t.Fatalf("token hash mismatch: stored=%q", rows[0].TokenHash)
	}

	if err := store.setJID(ctx, id, "5511999999999:1@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	rows, _ = store.list(ctx)
	if rows[0].JID != "5511999999999:1@s.whatsapp.net" {
		t.Fatalf("jid not persisted: %+v", rows[0])
	}

	// token rotation invalidates the old hash
	oldHash := rows[0].TokenHash
	newToken, err := store.rotateToken(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	row, err := store.get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if row.TokenHash == oldHash {
		t.Fatal("token hash should change after rotation")
	}
	if hashToken(newToken) != row.TokenHash {
		t.Fatal("rotated token hash mismatch")
	}

	if err := store.delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	rows, _ = store.list(ctx)
	if len(rows) != 0 {
		t.Fatalf("expected empty after delete, got %+v", rows)
	}
}
