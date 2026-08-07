package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// RecoveryMetrics tracks system reliability indicators
type RecoveryMetrics struct {
	TotalBackupsCreated   int64     `json:"total_backups_created"`
	LastBackupTime        time.Time `json:"last_backup_time"`
	TotalRestoresExecuted int64     `json:"total_restores_executed"`
	SuccessfulRestores    int64     `json:"successful_restores"`
	RecoverySuccessRate   float64   `json:"recovery_success_rate"`
}

// DRSupervisor monitors backup integrity, health status, and emits alerts
type DRSupervisor struct {
	mu           sync.RWMutex
	backupMgr    *BackupManager
	metrics      RecoveryMetrics
	recoveryLogs []string
	alerts       []string
}

// NewDRSupervisor initializes the Disaster Recovery Supervisor platform
func NewDRSupervisor(bm *BackupManager) *DRSupervisor {
	return &DRSupervisor{
		backupMgr:    bm,
		recoveryLogs: make([]string, 0),
		alerts:       make([]string, 0),
	}
}

// VerifyBackupBlock checks the cryptographic integrity of an encrypted backup file block
func (ds *DRSupervisor) VerifyBackupBlock(backupID string) (bool, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.backupMgr.mu.RLock()
	encrypted, exists := ds.backupMgr.rawStorage[backupID]
	meta := ds.backupMgr.backups[backupID]
	ds.backupMgr.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("backup ID %s does not exist", backupID)
	}

	// 1. Re-compute SHA-256 Checksum on the encrypted byte block
	hash := sha256.Sum256(encrypted)
	computedChecksum := hex.EncodeToString(hash[:])

	if meta.Checksum != computedChecksum {
		// Log severe alarm
		alertMsg := fmt.Sprintf("[CRITICAL SEVERITY ALARM] Cryptographic tamper detected on Backup Block %s!", backupID)
		ds.alerts = append(ds.alerts, alertMsg)
		meta.Verified = false
		return false, errors.New(alertMsg)
	}

	// 2. Try to decrypt the block to verify key decryption path matches
	_, err := ds.backupMgr.DecryptData(encrypted)
	if err != nil {
		alertMsg := fmt.Sprintf("[CRITICAL SEVERITY ALARM] Decryption key mismatch on Backup Block %s: %v", backupID, err)
		ds.alerts = append(ds.alerts, alertMsg)
		meta.Verified = false
		return false, errors.New(alertMsg)
	}

	meta.Verified = true
	ds.recoveryLogs = append(ds.recoveryLogs, fmt.Sprintf("[%s] Verified backup block %s successfully.", time.Now().Format(time.RFC3339), backupID))
	return true, nil
}

// RecordRestoreSession tracks successful or failed restore metrics
func (ds *DRSupervisor) RecordRestoreSession(success bool) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.metrics.TotalRestoresExecuted++
	if success {
		ds.metrics.SuccessfulRestores++
	} else {
		ds.alerts = append(ds.alerts, fmt.Sprintf("[ALARM] System restore session failed on execution at %s", time.Now().Format(time.RFC3339)))
	}

	if ds.metrics.TotalRestoresExecuted > 0 {
		ds.metrics.RecoverySuccessRate = float64(ds.metrics.SuccessfulRestores) / float64(ds.metrics.TotalRestoresExecuted)
	}
}

// IncrementBackupCount updates total backup parameters
func (ds *DRSupervisor) IncrementBackupCount() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.metrics.TotalBackupsCreated++
	ds.metrics.LastBackupTime = time.Now()
}

// GetMetrics returns active metrics reports
func (ds *DRSupervisor) GetMetrics() RecoveryMetrics {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.metrics
}

// ListAlerts returns active alarms
func (ds *DRSupervisor) ListAlerts() []string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.alerts
}

// ListLogs returns system verification histories
func (ds *DRSupervisor) ListLogs() []string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.recoveryLogs
}
