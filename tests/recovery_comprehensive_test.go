package tests

import (
	"fmt"
	"testing"
	"time"

	"velyxora/packages/recovery"
)

func TestComprehensiveDisasterRecoveryAndPITR(t *testing.T) {
	bm, err := recovery.NewBackupManager()
	if err != nil {
		t.Fatalf("failed to create backup manager: %v", err)
	}

	re := recovery.NewRestoreEngine(bm)
	ds := recovery.NewDRSupervisor(bm)

	baseState := &recovery.BackupState{
		DatabaseTables: map[string]int{"balances": 500},
		WalletState:    map[string]float64{"wal_warm": 50000.0},
		Configs:        map[string]string{"STATUS": "ONLINE"},
		Secrets:        map[string]string{"HSM_PIN": "hashed_hsm_pin"},
	}

	// 1. Create secure encrypted full backup
	meta, err := bm.CreateBackup(recovery.BackupFull, baseState)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}
	ds.IncrementBackupCount()

	// Verify backup checksum is valid initially
	ok, err := ds.VerifyBackupBlock(meta.ID)
	if !ok || err != nil {
		t.Fatalf("VerifyBackupBlock failed: %v", err)
	}

	// 2. Append sequence journal updates
	now := time.Now()
	re.AppendJournalLog(&recovery.JournalRecord{
		Sequence:  1,
		Timestamp: now.Add(1 * time.Second),
		Type:      "TABLE_ROW_INSERT",
		Payload:   `{"table": "balances", "rows": 25}`,
	})
	re.AppendJournalLog(&recovery.JournalRecord{
		Sequence:  2,
		Timestamp: now.Add(2 * time.Second),
		Type:      "WALLET_BALANCE_ADJUST",
		Payload:   `{"wallet_id": "wal_warm", "delta": -1200.5}`,
	})
	re.AppendJournalLog(&recovery.JournalRecord{
		Sequence:  3,
		Timestamp: now.Add(3 * time.Second),
		Type:      "CONFIG_UPDATE",
		Payload:   `{"key": "STATUS", "val": "MAINTENANCE"}`,
	})

	// 3. Restore to Point-In-Time at now + 2.5 seconds (capturing rows insert and wallet debit, but status remains ONLINE)
	targetTime := now.Add(2 * time.Second + 500 * time.Millisecond)
	recovered, err := re.RestoreToPointInTime(meta.ID, targetTime)
	if err != nil {
		t.Fatalf("RestoreToPointInTime failed: %v", err)
	}

	ds.RecordRestoreSession(true)

	// Validate restored balances row count (500 + 25 = 525)
	if recovered.DatabaseTables["balances"] != 525 {
		t.Errorf("expected balances row count to be 525, got %d", recovered.DatabaseTables["balances"])
	}

	// Validate warm wallet balance (50000.0 - 1200.5 = 48799.5)
	if recovered.WalletState["wal_warm"] != 48799.5 {
		t.Errorf("expected wal_warm balance to be 48799.5, got %.1f", recovered.WalletState["wal_warm"])
	}

	// Validate status config is still ONLINE (not replayed CONFIG_UPDATE)
	if recovered.Configs["STATUS"] != "ONLINE" {
		t.Errorf("expected config status to remain ONLINE, got %s", recovered.Configs["STATUS"])
	}

	// Validate metrics are recorded correctly
	metrics := ds.GetMetrics()
	if metrics.TotalBackupsCreated != 1 || metrics.TotalRestoresExecuted != 1 || metrics.SuccessfulRestores != 1 {
		t.Errorf("invalid supervisor metrics recorded: %+v", metrics)
	}

	fmt.Printf("[INTEGRATION_OK] Successfully validated Point-in-Time Disaster Recovery on %s\n", targetTime.Format(time.RFC3339))
}
