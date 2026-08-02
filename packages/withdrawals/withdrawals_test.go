package withdrawals

import (
	"context"
	"testing"
)

func TestWithdrawalCreationAndApproval(t *testing.T) {
	we := NewWithdrawalEngine()
	ctx := context.Background()

	// 1. Rejects unregistered whitelists
	_, errBk := we.CreateWithdrawal(ctx, "usr_101", "BTC", 0.1, 0.0, "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")
	if errBk == nil {
		t.Error("Expected error because address is not whitelisted, got nil")
	}

	// Add to book
	we.AddAddressToBook("usr_101", "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", "Bitcoin", "My BTC Whitelist", true)

	// 2. Accept whitelisted address
	req, err := we.CreateWithdrawal(ctx, "usr_101", "BTC", 0.1, 0.0, "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")
	if err != nil {
		t.Fatalf("Failed to create standard withdrawal: %v", err)
	}

	if req.Status != StatusRequested {
		t.Errorf("Expected status REQUESTED, got %s", req.Status)
	}

	// 3. Reject single limit boundary violations
	_, errLimit := we.CreateWithdrawal(ctx, "usr_101", "BTC", 1.5, 0.0, "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2") // Max single limit: 0.5
	if errLimit == nil {
		t.Error("Expected limit boundary error, got nil")
	}

	// 4. Admin approvals
	req, err = we.ApproveWithdrawal(ctx, req.ID, "admin_1")
	if err != nil {
		t.Fatalf("ApproveWithdrawal failed: %v", err)
	}

	if req.Status != StatusApproved {
		t.Errorf("Expected StatusApproved, got %s", req.Status)
	}

	// 5. Broadcast payload
	req, err = we.BroadcastTransaction(ctx, req.ID, "raw_hex_data_abc")
	if err != nil {
		t.Fatalf("BroadcastTransaction failed: %v", err)
	}

	if req.Status != StatusBroadcast || req.TxHash == "" {
		t.Errorf("Expected StatusBroadcast and valid TxHash")
	}

	// 6. Finalization
	req, err = we.FinalizeWithdrawal(ctx, req.ID)
	if err != nil {
		t.Fatalf("FinalizeWithdrawal failed: %v", err)
	}

	if req.Status != StatusCompleted {
		t.Errorf("Expected StatusCompleted, got %s", req.Status)
	}
}

func TestWithdrawalCancellations(t *testing.T) {
	we := NewWithdrawalEngine()
	ctx := context.Background()

	we.AddAddressToBook("usr_202", "0xReceiver", "Ethereum", "EVM destination", true)

	req, _ := we.CreateWithdrawal(ctx, "usr_202", "ETH", 1.5, 0.005, "0xReceiver")

	req, err := we.CancelWithdrawal(ctx, req.ID)
	if err != nil {
		t.Fatalf("CancelWithdrawal failed: %v", err)
	}

	if req.Status != StatusRejected || req.Message != "Canceled by user" {
		t.Errorf("Expected StatusRejected, got %s", req.Status)
	}
}
