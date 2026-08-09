package database

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"sync"
)

// AdvisoryLockManager provides distributed single-writer leader coordination using Postgres session advisory locks.
// Because pg_try_advisory_lock is session-scoped, different connections in a connection pool are treated as different sessions.
// Thus, we acquire a dedicated connection from the pool and hold it for the duration of the lock.
type AdvisoryLockManager struct {
	db     *DB
	lockID int64
	conn   pgx.Tx
	mu     sync.Mutex
}

// NewAdvisoryLockManager initializes advisory lock coordination on a specific numerical lock identifier.
func NewAdvisoryLockManager(db *DB, lockID int64) *AdvisoryLockManager {
	return &AdvisoryLockManager{
		db:     db,
		lockID: lockID,
	}
}

// TryLock attempts to acquire the transactional session-level exclusive advisory lock. Returns true if acquired.
func (alm *AdvisoryLockManager) TryLock(ctx context.Context) (bool, error) {
	if alm.db == nil {
		return true, nil // Standalone memory mode bypass
	}

	alm.mu.Lock()
	defer alm.mu.Unlock()

	if alm.conn != nil {
		return true, nil // Already holding the lock
	}

	// Begin a transaction to pin a single database connection/session
	tx, err := alm.db.Pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin lock transaction: %w", err)
	}

	var acquired bool
	err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", alm.lockID).Scan(&acquired)
	if err != nil {
		_ = tx.Rollback(ctx)
		return false, fmt.Errorf("failed to query pg_try_advisory_xact_lock: %w", err)
	}

	if !acquired {
		_ = tx.Rollback(ctx)
		return false, nil
	}

	alm.conn = tx
	return true, nil
}

// Unlock releases the exclusive session-level advisory lock by committing/rolling back the transaction pinning the connection.
func (alm *AdvisoryLockManager) Unlock(ctx context.Context) (bool, error) {
	if alm.db == nil {
		return true, nil // Standalone memory mode bypass
	}

	alm.mu.Lock()
	defer alm.mu.Unlock()

	if alm.conn == nil {
		return true, nil // Lock is already released/not held
	}

	err := alm.conn.Rollback(ctx)
	alm.conn = nil

	if err != nil {
		return false, fmt.Errorf("failed to release transactional advisory lock: %w", err)
	}

	return true, nil
}
