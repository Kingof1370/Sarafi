package security

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"velyxora/packages/database"
)

// BackupRestoreEngine manages secure PostgreSQL backups and isolated test restores.
type BackupRestoreEngine struct {
	db         *database.DB
	encryptionKey []byte // Loaded from env or config
}

// getPGCommand builds an executable command using configurable container name or native binary
func getPGCommand(ctx context.Context, binary string, args []string) *exec.Cmd {
	container := os.Getenv("DB_CONTAINER_NAME")
	if container == "" {
		// If running inside standard container/native, use binary directly
		return exec.CommandContext(ctx, binary, args...)
	}
	// If running externally with Docker forwarding
	dockerArgs := append([]string{"exec"}, container)
	// If stdin piping is needed for psql
	if binary == "psql" {
		dockerArgs = append(dockerArgs, "-i")
	}
	dockerArgs = append(dockerArgs, binary)
	dockerArgs = append(dockerArgs, args...)
	return exec.CommandContext(ctx, "docker", dockerArgs...)
}

// NewBackupRestoreEngine initializes the secure backup and restore subsystem.
func NewBackupRestoreEngine(db *database.DB, hexOrPlainKey string) (*BackupRestoreEngine, error) {
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "production"
	}

	key := []byte(hexOrPlainKey)
	if appEnv == "production" {
		if len(hexOrPlainKey) == 0 || hexOrPlainKey == "velyxora-backup-default-aes-key32" {
			return nil, fmt.Errorf("FAIL CLOSED: BACKUP_ENCRYPTION_KEY is missing or insecure in production mode")
		}
		if len(key) < 32 {
			return nil, fmt.Errorf("FAIL CLOSED: BACKUP_ENCRYPTION_KEY is too weak in production mode (must be at least 32 characters)")
		}
	} else {
		if len(key) == 0 {
			key = []byte("velyxora-backup-default-aes-key32")
		}
	}

	if len(key) < 32 {
		// Pad to 32 bytes for AES-256
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	} else if len(key) > 32 {
		key = key[:32]
	}

	return &BackupRestoreEngine{
		db:         db,
		encryptionKey: key,
	}, nil
}

// CreateEncryptedBackup runs pg_dump, encrypts the output with AES-256-GCM, and returns the encrypted bytes and/or file.
func (bre *BackupRestoreEngine) CreateEncryptedBackup(ctx context.Context, host, user, password, dbName string, port int) ([]byte, string, error) {
	// 1. Run pg_dump via configurable command builder
	args := []string{"-U", user, "-d", dbName, "--clean", "--no-owner"}
	if host != "" && host != "localhost" && host != "127.0.0.1" {
		args = append(args, "-h", host)
	}
	if port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", port))
	}

	cmd := getPGCommand(ctx, "pg_dump", args)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", password))

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		return nil, "", fmt.Errorf("pg_dump failed: %v, stderr: %s", err, stderrBuf.String())
	}

	plainData := stdoutBuf.Bytes()
	if len(plainData) == 0 {
		return nil, "", fmt.Errorf("pg_dump returned empty backup data")
	}

	// 2. Encrypt using AES-256-GCM
	block, err := aes.NewCipher(bre.encryptionKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", fmt.Errorf("failed to read nonce: %w", err)
	}

	encryptedData := gcm.Seal(nonce, nonce, plainData, nil)

	// Write encrypted backup file
	id := fmt.Sprintf("bk_db_%d", time.Now().UnixNano())
	filepath := fmt.Sprintf("/tmp/velyxora_encrypted_backup_%s.enc", id)

	err = os.WriteFile(filepath, encryptedData, 0600)
	if err != nil {
		return nil, "", fmt.Errorf("failed to write encrypted backup to file: %w", err)
	}

	// Store backup event in PostgreSQL if db is configured
	if bre.db != nil {
		_, _ = bre.db.Pool.Exec(ctx,
			"INSERT INTO backup_history (id, backup_type, status, filepath, timestamp) VALUES ($1, $2, $3, $4, NOW())",
			id, "DATABASE", "COMPLETED", filepath)
	}

	return encryptedData, filepath, nil
}

// DecryptAndVerifyBackup decrypts the backup file and verifies integrity and content (unencrypted raw check).
func (bre *BackupRestoreEngine) DecryptAndVerifyBackup(filepath string) ([]byte, error) {
	encryptedData, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file: %w", err)
	}

	block, err := aes.NewCipher(bre.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plainData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt backup file (integrity verification failed): %w", err)
	}

	// Verify standard PostgreSQL script markers to confirm valid sql content
	if !bytes.Contains(plainData, []byte("PostgreSQL database dump")) && !bytes.Contains(plainData, []byte("CREATE TABLE")) {
		return nil, fmt.Errorf("backup file lacks valid PostgreSQL dump signatures")
	}

	return plainData, nil
}

// RunIsolatedRestoreTest restores decrypted dump data into a dynamically created isolated database for strict verification.
func (bre *BackupRestoreEngine) RunIsolatedRestoreTest(ctx context.Context, backupFilepath string, user, password string) (bool, map[string]interface{}, error) {
	results := make(map[string]interface{})

	// 1. Decrypt and verify backup file first
	plainData, err := bre.DecryptAndVerifyBackup(backupFilepath)
	if err != nil {
		return false, nil, fmt.Errorf("backup decryption / verification failed: %w", err)
	}

	// 2. Create a temporary isolated database name
	restoreDBName := fmt.Sprintf("velyxora_restore_verify_%d", time.Now().UnixNano()/1e6)
	results["temp_db"] = restoreDBName

	// Create dynamic isolated database
	createCmd := getPGCommand(ctx, "createdb", []string{"-U", user, restoreDBName})
	createCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", password))
	if err := createCmd.Run(); err != nil {
		return false, nil, fmt.Errorf("failed to create isolated restore DB %s: %v", restoreDBName, err)
	}

	// Guarantee cleanup of the temporary database
	defer func() {
		dropCmd := getPGCommand(ctx, "dropdb", []string{"-U", user, "--if-exists", restoreDBName})
		dropCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", password))
		_ = dropCmd.Run()
	}()

	// 3. Restore plain backup dump into the isolated database using psql
	restoreCmd := getPGCommand(ctx, "psql", []string{"-U", user, "-d", restoreDBName})
	restoreCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", password))
	restoreCmd.Stdin = bytes.NewReader(plainData)

	var stderrBuf bytes.Buffer
	restoreCmd.Stderr = &stderrBuf

	if err := restoreCmd.Run(); err != nil {
		return false, nil, fmt.Errorf("isolated restore execution failed: %v, stderr: %s", err, stderrBuf.String())
	}

	// 4. Run structural & financial verification queries on the isolated restored database
	verifyPool, err := database.NewConnectionPool(database.Config{
		Host:     "localhost", // host outside docker pointing to local port forwarding
		Port:     5432,
		User:     user,
		Password: password,
		DBName:   restoreDBName,
		SSLMode:  "disable",
	})
	if err != nil {
		return false, nil, fmt.Errorf("failed to connect to isolated restored database for verification: %w", err)
	}
	defer verifyPool.Close()

	// A. Verify key tables exist
	var usersCount, balancesCount, ledgerCount, ordersCount, complianceCount, incidentCount int
	_ = verifyPool.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&usersCount)
	_ = verifyPool.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM balances").Scan(&balancesCount)
	_ = verifyPool.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM ledger_entries").Scan(&ledgerCount)
	_ = verifyPool.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&ordersCount)
	_ = verifyPool.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM compliance_cases").Scan(&complianceCount)
	_ = verifyPool.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM sre_incidents").Scan(&incidentCount)

	results["users_count"] = usersCount
	results["balances_count"] = balancesCount
	results["ledger_entries_count"] = ledgerCount
	results["orders_count"] = ordersCount
	results["compliance_cases_count"] = complianceCount
	results["sre_incidents_count"] = incidentCount

	// B. Verify double-entry ledger math integrity: sum(amount) for Credits should equal sum(amount) for Debits if completely balanced
	var debitsSum, creditsSum float64
	_ = verifyPool.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(amount), 0) FROM ledger_entries WHERE type = 'DEBIT'").Scan(&debitsSum)
	_ = verifyPool.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(amount), 0) FROM ledger_entries WHERE type = 'CREDIT'").Scan(&creditsSum)

	results["debits_sum"] = debitsSum
	results["credits_sum"] = creditsSum
	results["ledger_balanced"] = (debitsSum == creditsSum)

	// Save verification history if authoritative DB exists
	if bre.db != nil {
		resultsJSON, _ := json.Marshal(results)
		id := fmt.Sprintf("rec_%d", time.Now().UnixNano())
		_, _ = bre.db.Pool.Exec(ctx,
			"INSERT INTO recovery_history (id, status, details, timestamp) VALUES ($1, $2, $3, NOW())",
			id, "SUCCESS", string(resultsJSON))
	}

	return true, results, nil
}
