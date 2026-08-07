package database

import (
	"context"
	"testing"
)

// Since we may not have an active Postgres server in standard unit tests,
// we will write a unit test structure. For full verification we can test Config format parsing.
func TestDBConnectionAndMigrations(t *testing.T) {
	ctx := context.Background()

	// Connect to real docker-compose postgres running locally on port 5432
	db, err := NewConnectionPool(Config{
		Host:     "localhost",
		Port:     5432,
		User:     "velyxora",
		Password: "super-secure-db-password-123",
		DBName:   "velyxora",
		SSLMode:  "disable",
	})
	if err != nil {
		t.Skipf("Skipping real DB test; database not reachable: %v", err)
		return
	}
	defer db.Close()

	err = db.Ping(ctx)
	if err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	err = RunMigrations(ctx, db)
	if err != nil {
		t.Fatalf("Failed to execute migrations: %v", err)
	}

	// Verify that the table brute_force_lockouts and others exist by querying them
	var exists bool
	err = db.Pool.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'user_sessions')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to query information_schema: %v", err)
	}
	if !exists {
		t.Error("Expected table user_sessions to exist after migrations run")
	}

	err = db.Pool.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'user_api_keys')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to query information_schema: %v", err)
	}
	if !exists {
		t.Error("Expected table user_api_keys to exist after migrations run")
	}
}
