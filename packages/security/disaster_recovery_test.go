package security

import (
	"context"
	"os"
	"testing"
	"velyxora/packages/database"
)

func TestBackupRestoreEngine(t *testing.T) {
	// Initialize Connection to Database inside docker sandbox
	db, err := database.NewConnectionPool(database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "velyxora",
		Password: "super-secure-db-password-123",
		DBName:   "velyxora",
		SSLMode:  "disable",
	})
	if err != nil {
		t.Skipf("Postgres DB not running, skipping test: %v", err)
	}
	defer db.Close()

	bre, err := NewBackupRestoreEngine(db, "velyxora-dedicated-backup-aes-key")
	if err != nil {
		t.Fatalf("Failed to create BackupRestoreEngine: %v", err)
	}

	ctx := context.Background()

	// 1. Create encrypted backup
	encrypted, filepath, err := bre.CreateEncryptedBackup(ctx, "localhost", "velyxora", "super-secure-db-password-123", "velyxora", 5432)
	if err != nil {
		t.Fatalf("CreateEncryptedBackup failed: %v", err)
	}
	defer os.Remove(filepath)

	if len(encrypted) == 0 {
		t.Error("Encrypted backup payload is empty")
	}

	// 2. Run Isolated Restore Verification Test
	success, results, err := bre.RunIsolatedRestoreTest(ctx, filepath, "velyxora", "super-secure-db-password-123")
	if err != nil {
		t.Fatalf("RunIsolatedRestoreTest failed: %v", err)
	}

	if !success {
		t.Error("Restore verification was not successful")
	}

	if results["temp_db"] == "" {
		t.Error("Temporary restore DB name was not returned")
	}

	t.Logf("Restore verification results: %+v", results)
}
