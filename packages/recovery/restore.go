package recovery

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// JournalRecord represents a single transactional record appended to sequential journal files
type JournalRecord struct {
	Sequence  int64     `json:"sequence"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`      // e.g. LEDGER_CREDIT, LEDGER_DEBIT, WALLET_LOCK
	Payload   string    `json:"payload"`   // JSON-serialized change data
}

// RestoreEngine handles recovery operations and point-in-time sequential journal replaying
type RestoreEngine struct {
	mu            sync.RWMutex
	backupMgr     *BackupManager
	journals      []*JournalRecord
	recoveredState *BackupState
}

// NewRestoreEngine initializes the Enterprise Restore & Recovery Engine
func NewRestoreEngine(bm *BackupManager) *RestoreEngine {
	return &RestoreEngine{
		backupMgr: bm,
		journals:  make([]*JournalRecord, 0),
	}
}

// AppendJournalLog registers a transaction journal log sequentially
func (re *RestoreEngine) AppendJournalLog(record *JournalRecord) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.journals = append(re.journals, record)
}

// RestoreToPointInTime recovers a base backup state and replays journals up to a specific timestamp
func (re *RestoreEngine) RestoreToPointInTime(backupID string, targetTime time.Time) (*BackupState, error) {
	re.mu.Lock()
	defer re.mu.Unlock()

	// 1. Retrieve and decrypt the base encrypted backup
	decryptedBytes, err := re.backupMgr.GetBackupData(backupID)
	if err != nil {
		return nil, fmt.Errorf("failed to load base backup: %w", err)
	}

	var baseState BackupState
	if err := json.Unmarshal(decryptedBytes, &baseState); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted backup state: %w", err)
	}

	// Make copies of structures to avoid reference mutation during replay
	restoredTables := make(map[string]int)
	for k, v := range baseState.DatabaseTables {
		restoredTables[k] = v
	}
	restoredWallets := make(map[string]float64)
	for k, v := range baseState.WalletState {
		restoredWallets[k] = v
	}
	restoredConfigs := make(map[string]string)
	for k, v := range baseState.Configs {
		restoredConfigs[k] = v
	}
	restoredSecrets := make(map[string]string)
	for k, v := range baseState.Secrets {
		restoredSecrets[k] = v
	}

	// 2. Filter and replay sequential journals up to targetTime
	replayCount := 0
	for _, jr := range re.journals {
		// Stop if journal is after target point-in-time
		if jr.Timestamp.After(targetTime) {
			break
		}

		// Replay actions dynamically
		switch jr.Type {
		case "TABLE_ROW_INSERT":
			var payload struct {
				Table string `json:"table"`
				Rows  int    `json:"rows"`
			}
			if err := json.Unmarshal([]byte(jr.Payload), &payload); err == nil {
				restoredTables[payload.Table] += payload.Rows
			}
		case "WALLET_BALANCE_ADJUST":
			var payload struct {
				WalletID string  `json:"wallet_id"`
				Delta    float64 `json:"delta"`
			}
			if err := json.Unmarshal([]byte(jr.Payload), &payload); err == nil {
				restoredWallets[payload.WalletID] += payload.Delta
			}
		case "CONFIG_UPDATE":
			var payload struct {
				Key string `json:"key"`
				Val string `json:"val"`
			}
			if err := json.Unmarshal([]byte(jr.Payload), &payload); err == nil {
				restoredConfigs[payload.Key] = payload.Val
			}
		}
		replayCount++
	}

	finalState := &BackupState{
		DatabaseTables: restoredTables,
		WalletState:    restoredWallets,
		Configs:        restoredConfigs,
		Secrets:        restoredSecrets,
	}

	re.recoveredState = finalState
	fmt.Printf("[PITR RECOVERY] Successfully replayed %d journals up to %s\n", replayCount, targetTime.Format(time.RFC3339))

	return finalState, nil
}

// GetRecoveredState returns the last restored state
func (re *RestoreEngine) GetRecoveredState() *BackupState {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.recoveredState
}
