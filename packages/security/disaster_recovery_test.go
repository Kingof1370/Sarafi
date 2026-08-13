package security

import (
	"context"
	"encoding/json"
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

func TestBackupRestoreEngineEnvelopeEncryption(t *testing.T) {
	bre, err := NewBackupRestoreEngine(nil, "velyxora-backup-key-for-test-32bytes")
	if err != nil {
		t.Fatalf("Failed to create BackupRestoreEngine: %v", err)
	}

	km, err := NewKeyManager(nil, "", "velyxora-kms-kek-key-for-test-32bytes")
	if err != nil {
		t.Fatalf("Failed to create KeyManager: %v", err)
	}
	bre.SetKeyManager(km)

	// Since we don't have pg_dump running in tests without active DB, let's test DecryptAndVerifyBackup directly
	// by writing mock raw SQL data, encrypting it manually using the envelope encrypted backup structure,
	// and verifying that DecryptAndVerifyBackup decrypts it perfectly.

	sqlDump := []byte("PostgreSQL database dump -- CREATE TABLE users (id SERIAL PRIMARY KEY);")

	// Let's create an envelope encrypted backup manually
	filepath := "/tmp/velyxora_envelope_test_backup.enc"
	defer os.Remove(filepath)

	// 1. Generate DEK
	dek := []byte("backup-dek-key-for-test-32bytes-")
	encSQL, err := aesGCMEncrypt(dek, sqlDump)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	// 2. Encrypt DEK with KMS KEK
	encDEK, err := km.KMS.Encrypt(context.Background(), dek)
	if err != nil {
		t.Fatalf("failed to encrypt DEK: %v", err)
	}

	envelope := EnvelopeEncryptedBackup{
		EncryptedData: encSQL,
		EncryptedDEK:  encDEK,
	}

	data, err := jsonMarshal(envelope)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	if err := os.WriteFile(filepath, data, 0600); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Verify DecryptAndVerifyBackup can parse, decrypt DEK via KMS, decrypt SQL with DEK, and verify standard signature
	decrypted, err := bre.DecryptAndVerifyBackup(filepath)
	if err != nil {
		t.Fatalf("DecryptAndVerifyBackup failed: %v", err)
	}

	if string(decrypted) != string(sqlDump) {
		t.Errorf("Decrypted SQL data mismatch: got '%s', expected '%s'", string(decrypted), string(sqlDump))
	}
}

// Simple JSON marshalling helper to avoid extra imports
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
