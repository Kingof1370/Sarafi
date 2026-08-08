package security

import (
	"context"
	"os"
	"testing"
	"time"

	"velyxora/packages/database"
)

func TestDisasterRecoverySuite(t *testing.T) {
	// Initialize a transient test database context or bypass safely
	cfg := database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres_password",
		DBName:   "velyxora_test",
		SSLMode:  "disable",
	}

	db, err := database.NewConnectionPool(cfg)
	if err != nil {
		t.Logf("PostgreSQL is unavailable in this test context, using standalone mocks or skipping integration. Error: %v", err)
		// We can still run pure mock test verification if database is down
	}

	tempDir, err := os.MkdirTemp("", "velyxora-backups-test")
	if err != nil {
		t.Fatalf("Failed to create temporary backup test directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine, err := NewDREngine(db, tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize Disaster Recovery Engine: %v", err)
	}

	ctx := context.Background()

	// 1. Operational Mode Test
	t.Run("Disaster Recovery System Modes", func(t *testing.T) {
		status, err := engine.GetSystemStatus(ctx)
		if err != nil {
			t.Fatalf("Failed to get system status: %v", err)
		}

		if status.Mode != ModeNormal {
			t.Errorf("Expected initial state to be NORMAL, got %s", status.Mode)
		}

		err = engine.SetSystemMode(ctx, ModeEmergency, true)
		if err != nil {
			t.Fatalf("Failed to update system mode: %v", err)
		}

		status, _ = engine.GetSystemStatus(ctx)
		if status.Mode != ModeEmergency || !status.IncidentActive {
			t.Errorf("Failed to update operational mode to EMERGENCY")
		}

		// Reset
		_ = engine.SetSystemMode(ctx, ModeNormal, false)
	})

	// 2. Distributed Locks / Leader Election Test
	t.Run("Distributed Coordination Locks", func(t *testing.T) {
		lockKey := "cron_scheduled_backup_job"
		owner1 := "worker-node-alpha"
		owner2 := "worker-node-beta"

		acquired, err := engine.AcquireDistributedLock(ctx, lockKey, owner1, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Lock acquisition failed: %v", err)
		}
		if !acquired {
			t.Fatalf("Expected owner1 to acquire the lock successfully")
		}

		// Attempt to acquire from owner2 while lock is active
		acquired2, _ := engine.AcquireDistributedLock(ctx, lockKey, owner2, 100*time.Millisecond)
		if acquired2 {
			t.Errorf("Owner2 should have been blocked from acquiring an active lock")
		}

		// Wait for TTL expiration
		time.Sleep(120 * time.Millisecond)

		acquired3, _ := engine.AcquireDistributedLock(ctx, lockKey, owner2, 100*time.Millisecond)
		if !acquired3 {
			t.Errorf("Owner2 should have acquired the lock after TTL expiration")
		}

		_ = engine.ReleaseDistributedLock(ctx, lockKey, owner2)
	})

	// 3. Backup & Restore Tests (Only if live DB exists)
	if db != nil {
		t.Run("Cryptographic Backups & Integrity Restore Workflow", func(t *testing.T) {
			// Clear first
			_, _ = db.Pool.Exec(ctx, "DELETE FROM ledger_entries")
			_, _ = db.Pool.Exec(ctx, "DELETE FROM balances")
			_, _ = db.Pool.Exec(ctx, "DELETE FROM users")

			// Seed user and balance
			_, _ = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, is_mfa_enabled, mfa_secret, status, role) VALUES ('usr_01', 'audit@test.com', 'pwhash', false, '', 'ACTIVE', 'ADMIN')")
			_, _ = db.Pool.Exec(ctx, "INSERT INTO balances (user_id, asset, available, locked, pending, reserved, total) VALUES ('usr_01', 'USDT', 1500.0, 0.0, 0.0, 0.0, 1500.0)")
			_, _ = db.Pool.Exec(ctx, "INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ('led_01', 'tx_1', 'usr_01', 'USDT', 'CREDIT', 1500.0, 'Initial', NOW())")

			// 1. Create Backup
			bak, err := engine.CreateBackup(ctx, "FULL")
			if err != nil {
				t.Fatalf("Failed to create database backup: %v", err)
			}

			if bak.Status != "SUCCESSFUL" {
				t.Fatalf("Expected backup status to be SUCCESSFUL, got %s", bak.Status)
			}

			// Verify file created
			if _, err := os.Stat(bak.Filepath); os.IsNotExist(err) {
				t.Fatalf("Backup output file does not exist: %s", bak.Filepath)
			}

			// 2. Validate Backup
			valid, err := engine.ValidateBackup(ctx, bak.ID)
			if err != nil || !valid {
				t.Fatalf("Backup verification failed: %v", err)
			}

			// 3. Destructive change simulation
			_, _ = db.Pool.Exec(ctx, "UPDATE balances SET available = 0.0, total = 0.0 WHERE user_id = 'usr_01'")

			// 4. Unauthorized Restore Attempt (Must fail safely)
			_, err = engine.RestoreBackup(ctx, bak.ID, "")
			if err == nil {
				t.Fatalf("Expected automatic unauthorized restore to be rejected")
			}

			// 5. Authorized Restore (Must succeed)
			rst, err := engine.RestoreBackup(ctx, bak.ID, "sys_admin_mfa_verified")
			if err != nil {
				t.Fatalf("Authorized restore failed: %v", err)
			}

			if rst.Status != "SUCCESSFUL" {
				t.Fatalf("Expected restore status to be SUCCESSFUL")
			}

			// 6. Post-restore Verification Checks
			var resAvailable float64
			err = db.Pool.QueryRow(ctx, "SELECT available FROM balances WHERE user_id = 'usr_01' AND asset = 'USDT'").Scan(&resAvailable)
			if err != nil {
				t.Fatalf("Failed to scan restored balance: %v", err)
			}

			if resAvailable != 1500.0 {
				t.Errorf("Expected restored balance to be 1500.0, got %f", resAvailable)
			}

			// 7. Balance Reconciliation Check
			issues, err := engine.ReconstructAndReconcileBalances(ctx)
			if err != nil {
				t.Fatalf("Balance reconstruction reconciliation run failed: %v", err)
			}

			if len(issues) > 0 {
				t.Errorf("Expected 0 accounting reconciliation issues, got %d: %v", len(issues), issues)
			}
		})
	}
}
