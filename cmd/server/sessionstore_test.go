package main

import (
	"context"
	"database/sql"
	"errors"
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

func TestSessionStoreListHandlesNullClientID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	db.ExecContext(ctx, "DELETE FROM sessions")
	db.ExecContext(ctx, "DELETE FROM users")
	db.ExecContext(ctx, "DELETE FROM clients")

	if _, err := newAuthStore(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := newClientStore(ctx, db); err != nil {
		t.Fatal(err)
	}
	store, err := newSessionStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	// legacy row created before the multitenant migration: client_id is NULL
	if _, err := db.ExecContext(ctx,
		`ALTER TABLE sessions ALTER COLUMN client_id DROP NOT NULL`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO sessions (id, name, jid, token_hash, status, created_at, updated_at)
		 VALUES ($1, 'Legacy', '5511999999999:1@s.whatsapp.net', '', 'connected', now(), now())`,
		"00000000-0000-0000-0000-000000000099",
	); err != nil {
		t.Fatal(err)
	}

	rows, err := store.list(ctx)
	if err != nil {
		t.Fatalf("list should not fail on NULL client_id: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "00000000-0000-0000-0000-000000000099" || rows[0].ClientID != "" {
		t.Fatalf("unexpected rows for legacy session: %+v", rows)
	}

	if err := store.delete(ctx, rows[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestSessionStoreAssignOrphan(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

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
	clientA, err := clients.createWithAdmin(ctx, "Acme", "acme", 5, "acme@test.com", "Acme Admin", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	clientB, err := clients.createWithAdmin(ctx, "Beta", "beta", 5, "beta@test.com", "Beta Admin", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	clientC, err := clients.createWithAdmin(ctx, "Gamma", "gamma", 1, "gamma@test.com", "Gamma Admin", "secret123")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(ctx,
		`ALTER TABLE sessions ALTER COLUMN client_id DROP NOT NULL`,
	); err != nil {
		t.Fatal(err)
	}
	orphanID := "00000000-0000-0000-0000-0000000000aa"
	if _, err := db.ExecContext(ctx,
		`INSERT INTO sessions (id, name, jid, token_hash, status, created_at, updated_at)
		 VALUES ($1, 'Legacy', '5511999999999:1@s.whatsapp.net', '', 'connected', now(), now())`,
		orphanID,
	); err != nil {
		t.Fatal(err)
	}

	orphans, err := store.listOrphaned(ctx)
	if err != nil {
		t.Fatalf("listOrphaned: %v", err)
	}
	if len(orphans) != 1 || orphans[0].ID != orphanID {
		t.Fatalf("expected one orphan, got %+v", orphans)
	}

	if err := store.setClient(ctx, orphanID, clientA); err != nil {
		t.Fatalf("assign to A: %v", err)
	}
	rows, err := store.list(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].ClientID != clientA {
		t.Fatalf("expected client A after assign, got %q", rows[0].ClientID)
	}
	if orphans, _ = store.listOrphaned(ctx); len(orphans) != 0 {
		t.Fatalf("expected no orphans after assign, got %+v", orphans)
	}

	if _, _, err := store.createWithLimit(ctx, clientB, "Legacy"); err != nil {
		t.Fatalf("create session in B: %v", err)
	}
	if err := store.setClient(ctx, orphanID, clientB); !errors.Is(err, ErrSessionClientNameTaken) {
		t.Fatalf("expected name conflict, got %v", err)
	}

	if _, _, err := store.createWithLimit(ctx, clientC, "Only Slot"); err != nil {
		t.Fatalf("create session in C: %v", err)
	}
	if err := store.setClient(ctx, orphanID, clientC); !errors.Is(err, ErrSessionLimitReached) {
		t.Fatalf("expected limit reached, got %v", err)
	}

	if err := store.setClient(ctx, orphanID, clientB); !errors.Is(err, ErrSessionClientNameTaken) {
		t.Fatalf("expected name conflict on B reassign, got %v", err)
	}
	if err := store.setClient(ctx, orphanID, clientA); err != nil {
		t.Fatalf("reassign orphan to A: %v", err)
	}
	if rows, _ = store.list(ctx); rows[0].ClientID != clientA {
		t.Fatalf("expected client A after reassign, got %q", rows[0].ClientID)
	}

	if err := store.setClient(ctx, "does-not-exist", clientA); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected session not found, got %v", err)
	}
	if err := store.setClient(ctx, orphanID, "00000000-0000-0000-0000-00000000dead"); !errors.Is(err, ErrClientNotFound) {
		t.Fatalf("expected client not found, got %v", err)
	}
}
