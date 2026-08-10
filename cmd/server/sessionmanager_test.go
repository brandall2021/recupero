package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func newTestManager(t *testing.T) *SessionManager {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/wacalls_test?sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	// clean slate
	db.ExecContext(ctx, "DELETE FROM sessions")
	db.ExecContext(ctx, "DELETE FROM users")
	db.ExecContext(ctx, "DELETE FROM clients")
	db.ExecContext(ctx, "DELETE FROM whatsmeow_device")

	container := sqlstore.NewWithDB(db, "postgres", waLog.Noop)
	if err := container.Upgrade(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := newAuthStore(ctx, db); err != nil {
		t.Fatal(err)
	}
	clientSt, err := newClientStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	store, err := newSessionStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	recStore, err := newRecordingStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	return newSessionManager(ctx, container, NewBroker(), store, recStore, clientSt, waLog.Noop, slog.Default(), 0)
}

func TestSessionManagerRegistry(t *testing.T) {
	m := newTestManager(t)

	// create a client and seed a couple of sessions directly in the store
	clientID, err := m.clients.createWithAdmin(m.appCtx, "Acme", "acme", 5, "acme@test.com", "Acme Admin", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Account A", "Account B"} {
		id, token, err := m.store.createWithLimit(m.appCtx, clientID, name)
		if err != nil {
			t.Fatal(err)
		}
		client := whatsmeow.NewClient(m.container.NewDevice(), waLog.Noop)
		s := newSession(m, id, clientID, name, hashToken(token), "", client)
		m.register(s)
	}

	infos := m.infos()
	if len(infos) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(infos))
	}
	if infos[0].Name != "Account A" || infos[1].Name != "Account B" {
		t.Fatalf("registration order not preserved: %+v", infos)
	}
	if infos[0].Paired {
		t.Fatal("unconnected session should not report paired")
	}
	if !infos[0].TokenConfigured {
		t.Fatal("session should report tokenConfigured")
	}
	if infos[0].ClientID != clientID {
		t.Fatalf("session client mismatch: got %s want %s", infos[0].ClientID, clientID)
	}

	m.unregister(infos[1].ID)
	if _, ok := m.Get(infos[1].ID); ok {
		t.Fatal("session b should be gone after unregister")
	}
	if len(m.infos()) != 1 {
		t.Fatal("expected 1 session after unregister")
	}
}

func TestSessionLimitEnforced(t *testing.T) {
	m := newTestManager(t)
	clientID, err := m.clients.createWithAdmin(m.appCtx, "Limit Co", "limitco", 2, "limit@test.com", "Limit Admin", "secret123")
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if _, _, err := m.store.createWithLimit(m.appCtx, clientID, fmt.Sprintf("S%d", i+1)); err != nil {
			t.Fatalf("session %d should be created: %v", i+1, err)
		}
	}
	if _, _, err := m.store.createWithLimit(m.appCtx, clientID, "S3"); err == nil {
		t.Fatal("expected session_limit_reached on third session")
	} else if err != ErrSessionLimitReached {
		t.Fatalf("expected ErrSessionLimitReached, got %v", err)
	}
	// raising the limit allows more
	if err := m.clients.updateLimitsGuarded(m.appCtx, clientID, 3); err != nil {
		t.Fatalf("raise limit: %v", err)
	}
	if _, _, err := m.store.createWithLimit(m.appCtx, clientID, "S3"); err != nil {
		t.Fatalf("session 3 should now fit: %v", err)
	}

	// lowering below usage is refused
	if err := m.clients.updateLimitsGuarded(m.appCtx, clientID, 1); err != ErrLimitBelowUsage {
		t.Fatalf("expected ErrLimitBelowUsage, got %v", err)
	}
}
