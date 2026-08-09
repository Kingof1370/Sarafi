package tests

import (
	"context"
	"os"
	"testing"

	"velyxora/packages/database"
	"velyxora/packages/security"
)

func TestP0014_DisasterRecovery_EndToEnd(t *testing.T) {
	// Initialize Database Pool Connection
	db, err := database.NewConnectionPool(database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "velyxora",
		Password: "super-secure-db-password-123",
		DBName:   "velyxora",
		SSLMode:  "disable",
	})
	if err != nil {
		t.Skipf("PostgreSQL server not online, skipping test: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Initialize secure Backup/Restore Engine using a dedicated backup key
	os.Setenv("BACKUP_ENCRYPTION_KEY", "velyxora-extremely-secure-aes256-backup-key-10014")
	defer os.Unsetenv("BACKUP_ENCRYPTION_KEY")

	bre, err := security.NewBackupRestoreEngine(db, os.Getenv("BACKUP_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("Failed to instantiate BackupRestoreEngine: %v", err)
	}

	// 1. Trigger encrypted backup generation
	t.Log("Generating encrypted pg_dump backup archive...")
	_, filepath, err := bre.CreateEncryptedBackup(ctx, "localhost", "velyxora", "super-secure-db-password-123", "velyxora", 5432)
	if err != nil {
		t.Fatalf("Encrypted backup creation failed: %v", err)
	}
	defer os.Remove(filepath)

	// Confirm file has been successfully written on disk
	info, err := os.Stat(filepath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("Encrypted backup file was empty or missing: %v", err)
	}
	t.Logf("Secure backup archive generated successfully (Size: %d bytes). Filepath: %s", info.Size(), filepath)

	// 2. Validate backup integrity (decrypting archive and asserting pg SQL structure)
	t.Log("Verifying encrypted backup file integrity and signatures...")
	plainData, err := bre.DecryptAndVerifyBackup(filepath)
	if err != nil {
		t.Fatalf("Backup decryption and signature verification failed: %v", err)
	}
	if len(plainData) == 0 {
		t.Fatal("Decrypted backup output was completely empty")
	}
	t.Log("Backup integrity validated successfully.")

	// 3. Run transactional isolated restore test inside dynamic temporary PostgreSQL database
	t.Log("Testing full database restore inside isolated temporary database instance...")
	success, results, err := bre.RunIsolatedRestoreTest(ctx, filepath, "velyxora", "super-secure-db-password-123")
	if err != nil {
		t.Fatalf("Isolated restore verification failed: %v", err)
	}

	if !success {
		t.Fatal("Restore verification returned negative success result")
	}

	// 4. Assert financial ledger math and record counts are fully recovered
	t.Logf("Restore results asserting...: %+v", results)
	if results["temp_db"] == "" {
		t.Error("Isolated database name was empty")
	}

	if results["ledger_balanced"] != true {
		t.Error("Double-entry ledger is not balanced after full database restoration")
	}

	// 5. Test concurrent single-writer leader coordination with PostgreSQL advisory locks
	t.Log("Verifying single-writer advisory locks...")
	lockID := int64(14011400)
	alm1 := database.NewAdvisoryLockManager(db, lockID)
	alm2 := database.NewAdvisoryLockManager(db, lockID)

	acq1, err := alm1.TryLock(ctx)
	if err != nil || !acq1 {
		t.Fatalf("First writer failed to acquire advisory lock: %v", err)
	}

	acq2, err := alm2.TryLock(ctx)
	if err != nil {
		t.Fatalf("Second writer TryLock check failed with error: %v", err)
	}
	if acq2 {
		t.Fatal("Second writer successfully acquired advisory lock simultaneously (violates single-writer isolation)")
	}

	// Release locks
	_, _ = alm1.Unlock(ctx)
	_, _ = alm2.Unlock(ctx)
	t.Log("Single-writer leader coordination verified successfully.")
}
