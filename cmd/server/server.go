package main

import (
	"context"
	"database/sql"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type server struct {
	broker    *Broker
	sessions  *SessionManager
	auth      *authStore
	clients   *clientStore
	log       *slog.Logger
	staticDir string
}

func openDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return db, nil
}

func newServer(ctx context.Context, databaseURL, staticDir string, maxCalls int, log *slog.Logger) (*server, error) {
	db, err := openDB(databaseURL)
	if err != nil {
		return nil, err
	}
	container := sqlstore.NewWithDB(db, "postgres", waLog.Noop)
	if err := container.Upgrade(ctx); err != nil {
		return nil, err
	}
	clientSt, err := newClientStore(ctx, db)
	if err != nil {
		return nil, err
	}
	store, err := newSessionStore(ctx, db)
	if err != nil {
		return nil, err
	}
	recStore, err := newRecordingStore(ctx, db)
	if err != nil {
		return nil, err
	}
	authSt, err := newAuthStore(ctx, db)
	if err != nil {
		return nil, err
	}
	if err := authSt.Seed(ctx); err != nil {
		log.Warn("user seed failed", "err", err)
	}

	waLogger := waLog.Noop
	if log.Enabled(ctx, slog.LevelDebug) {
		waLogger = waLog.Stdout("WA", "INFO", true)
	}

	broker := NewBroker()
	mgr := newSessionManager(ctx, container, broker, store, recStore, clientSt, waLogger, log, maxCalls)
	broker.SnapshotFn = mgr.snapshotEvents

	return &server{broker: broker, sessions: mgr, auth: authSt, clients: clientSt, log: log, staticDir: staticDir}, nil
}
