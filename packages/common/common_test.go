package common

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"velyxora/packages/types"
)

// TestMockCommonSetup checks basic context pass-through
func TestMockCommonSetup(t *testing.T) {
	ctx := context.Background()
	_ = ctx
}

func TestVersionedKafkaWalletSchemas(t *testing.T) {
	// 1. Verify WalletCreatedEvent
	created := types.WalletCreatedEvent{
		Version:   "v1",
		WalletID:  "wal_hot_101",
		UserID:    "usr_202",
		Type:      "HOT",
		Timestamp: time.Now(),
	}

	raw, err := json.Marshal(created)
	if err != nil {
		t.Fatalf("Failed to marshal WalletCreatedEvent: %v", err)
	}

	var parsedCreated types.WalletCreatedEvent
	err = json.Unmarshal(raw, &parsedCreated)
	if err != nil {
		t.Fatalf("Failed to unmarshal WalletCreatedEvent: %v", err)
	}

	if parsedCreated.Version != "v1" || parsedCreated.WalletID != "wal_hot_101" {
		t.Errorf("Unexpected unmarshaled payload values")
	}

	// 2. Verify BalanceUpdatedEvent
	balUpdated := types.BalanceUpdatedEvent{
		Version:   "v1",
		WalletID:  "wal_hot_101",
		Asset:     "BTC",
		Available: 1.5,
		Locked:    0.5,
		Reserved:  0.0,
		Pending:   0.0,
		Total:     2.0,
		Timestamp: time.Now(),
	}

	rawBal, err := json.Marshal(balUpdated)
	if err != nil {
		t.Fatalf("Failed to marshal BalanceUpdatedEvent: %v", err)
	}

	var parsedBal types.BalanceUpdatedEvent
	err = json.Unmarshal(rawBal, &parsedBal)
	if err != nil {
		t.Fatalf("Failed to unmarshal BalanceUpdatedEvent: %v", err)
	}

	if parsedBal.Available != 1.5 || parsedBal.Total != 2.0 {
		t.Errorf("Unexpected unmarshaled Balance fields")
	}
}
