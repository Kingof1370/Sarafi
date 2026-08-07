package recovery

import (
	"bytes"
	"testing"
	"time"
)

func TestBackupAndRestoreEngine(t *testing.T) {
	bm, err := NewBackupManager()
	if err != nil {
		t.Fatalf("NewBackupManager failed: %v", err)
	}

	state := &BackupState{
		DatabaseTables: map[string]int{"users": 10, "orders": 50},
		WalletState:    map[string]float64{"wal_1": 1.5, "wal_2": 10.0},
		Configs:        map[string]string{"PORT": "8080"},
		Secrets:        map[string]string{"SECRET_ID": "hashed_secret_val"},
	}

	// 1. Create secure encrypted backup snapshot
	meta, err := bm.CreateBackup(BackupFull, state)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if meta.Type != BackupFull || meta.Size == 0 {
		t.Errorf("invalid backup metadata parameters: type=%s, size=%d", meta.Type, meta.Size)
	}

	// 2. Fetch and verify decrypted backup block
	data, err := bm.GetBackupData(meta.ID)
	if err != nil {
		t.Fatalf("GetBackupData failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected decrypted data payload to be populated")
	}

	// 3. Initialize Restore Engine and Replay logs
	re := NewRestoreEngine(bm)

	// Append a few sequential journal transactions (PITR logs)
	now := time.Now()
	re.AppendJournalLog(&JournalRecord{
		Sequence:  1,
		Timestamp: now.Add(1 * time.Minute),
		Type:      "TABLE_ROW_INSERT",
		Payload:   `{"table": "orders", "rows": 15}`,
	})
	re.AppendJournalLog(&JournalRecord{
		Sequence:  2,
		Timestamp: now.Add(2 * time.Minute),
		Type:      "WALLET_BALANCE_ADJUST",
		Payload:   `{"wallet_id": "wal_1", "delta": 2.5}`,
	})
	re.AppendJournalLog(&JournalRecord{
		Sequence:  3,
		Timestamp: now.Add(3 * time.Minute), // past target point-in-time
		Type:      "CONFIG_UPDATE",
		Payload:   `{"key": "PORT", "val": "9000"}`,
	})

	// Restore to point-in-time at +2.5 minutes (which includes log 1 and 2, but excludes log 3)
	recovered, err := re.RestoreToPointInTime(meta.ID, now.Add(2*time.Minute+30*time.Second))
	if err != nil {
		t.Fatalf("RestoreToPointInTime failed: %v", err)
	}

	// Verify order rows incremented (50 + 15 = 65)
	if recovered.DatabaseTables["orders"] != 65 {
		t.Errorf("expected orders table rows to be 65, got %d", recovered.DatabaseTables["orders"])
	}

	// Verify wallet balance adjusted (1.5 + 2.5 = 4.0)
	if recovered.WalletState["wal_1"] != 4.0 {
		t.Errorf("expected wal_1 balance to be 4.0, got %.1f", recovered.WalletState["wal_1"])
	}

	// Verify config NOT updated to 9000 (remains 8080)
	if recovered.Configs["PORT"] != "8080" {
		t.Errorf("expected config PORT to remain 8080, got %s", recovered.Configs["PORT"])
	}
}

func TestDRSupervisor(t *testing.T) {
	bm, _ := NewBackupManager()
	ds := NewDRSupervisor(bm)

	state := &BackupState{
		DatabaseTables: map[string]int{"users": 5},
	}
	meta, _ := bm.CreateBackup(BackupFull, state)

	// Verify normal block checks
	ok, err := ds.VerifyBackupBlock(meta.ID)
	if !ok || err != nil {
		t.Fatalf("VerifyBackupBlock failed: %v", err)
	}

	// Simulate Tampering
	bm.mu.Lock()
	bm.rawStorage[meta.ID] = []byte("malicious tampered ciphertext block bytes")
	bm.mu.Unlock()

	ok, err = ds.VerifyBackupBlock(meta.ID)
	if !ok && err != nil {
		t.Logf("PASSED: successfully caught backup block tampering and raised critical alarm: %v", err)
	} else {
		t.Error("expected supervisor block check to detect tampering and throw alarm error")
	}

	if len(ds.ListAlerts()) == 0 {
		t.Error("expected critical severity alarm logged in the supervisor alerts list")
	}
}

func TestAESGCMSymmetricBlockSafety(t *testing.T) {
	bm, _ := NewBackupManager()
	payload := []byte("highly_confidential_secret_parameters_99")

	enc, err := bm.EncryptData(payload)
	if err != nil {
		t.Fatalf("EncryptData failed: %v", err)
	}

	dec, err := bm.DecryptData(enc)
	if err != nil {
		t.Fatalf("DecryptData failed: %v", err)
	}

	if !bytes.Equal(payload, dec) {
		t.Error("decrypted bytes mismatch original plaintext payload")
	}
}
