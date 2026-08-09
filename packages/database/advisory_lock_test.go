package database

import (
	"context"
	"testing"
)

func TestAdvisoryLockManager(t *testing.T) {
	db, err := NewConnectionPool(Config{
		Host:     "localhost",
		Port:     5432,
		User:     "velyxora",
		Password: "super-secure-db-password-123",
		DBName:   "velyxora",
		SSLMode:  "disable",
	})
	if err != nil {
		t.Skipf("Postgres DB not running, skipping: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	lockID := int64(100140)

	alm1 := NewAdvisoryLockManager(db, lockID)
	alm2 := NewAdvisoryLockManager(db, lockID)

	// 1. First writer acquires lock
	acquired, err := alm1.TryLock(ctx)
	if err != nil {
		t.Fatalf("alm1 TryLock failed: %v", err)
	}
	if !acquired {
		t.Error("alm1 failed to acquire free advisory lock")
	}

	// 2. Second writer fails to acquire lock (enforces single-writer behavior)
	acquired2, err := alm2.TryLock(ctx)
	if err != nil {
		t.Fatalf("alm2 TryLock failed: %v", err)
	}
	if acquired2 {
		t.Error("alm2 successfully acquired locked advisory lock (violates single-writer protection)")
	}

	// 3. First writer unlocks
	released, err := alm1.Unlock(ctx)
	if err != nil {
		t.Fatalf("alm1 Unlock failed: %v", err)
	}
	if !released {
		t.Error("alm1 failed to release its advisory lock")
	}

	// 4. Second writer can now acquire lock
	acquired3, err := alm2.TryLock(ctx)
	if err != nil {
		t.Fatalf("alm2 TryLock third check failed: %v", err)
	}
	if !acquired3 {
		t.Error("alm2 failed to acquire released advisory lock")
	}

	// Cleanup
	_, _ = alm2.Unlock(ctx)
}
