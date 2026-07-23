package main

import (
	"os"
	"testing"
)

func TestOpenDB(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/wacalls_test?sslmode=disable"
	}
	db, err := openDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if got := db.Stats().MaxOpenConnections; got != 25 {
		t.Fatalf("expected pool capped to 25 connections, got %d", got)
	}

	var version string
	if err := db.QueryRow("SELECT version()").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version == "" {
		t.Fatal("expected a PostgreSQL version string")
	}
}
