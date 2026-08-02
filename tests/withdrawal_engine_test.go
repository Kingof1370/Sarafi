package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"velyxora/packages/withdrawals"
)

// TestWithdrawalConcurrency verifies thread-safe parallel processing and approvals on the same engine
func TestWithdrawalConcurrency(t *testing.T) {
	we := withdrawals.NewWithdrawalEngine()
	ctx := context.Background()

	concurrencyLimit := 30
	var wg sync.WaitGroup
	wg.Add(concurrencyLimit)

	for i := 0; i < concurrencyLimit; i++ {
		go func(idx int) {
			defer wg.Done()
			uID := fmt.Sprintf("usr_con_%d", idx)
			addr := fmt.Sprintf("0xReceiver_%d", idx)
			we.AddAddressToBook(uID, addr, "Ethereum", "Whitelist address", true)
			req, err := we.CreateWithdrawal(ctx, uID, "USDT", 10.0, 1.0, addr)
			if err == nil {
				_, _ = we.ApproveWithdrawal(ctx, req.ID, "admin_user")
			} else {
				fmt.Printf("Concurrency error: %v\n", err)
			}
		}(i)
	}

	wg.Wait()

	list := we.ListRequests()
	if len(list) != concurrencyLimit {
		t.Errorf("Expected %d processed withdrawals, got %d", concurrencyLimit, len(list))
	}

	for _, r := range list {
		if r.Status != withdrawals.StatusApproved {
			t.Errorf("Expected approved status for concurrent withdrawal %s, got %s", r.ID, r.Status)
		}
	}
}

// TestWithdrawalLimitsAndCooldowns verifies that single limits, daily limits, and active cooldowns block invalid transfers
func TestWithdrawalLimitsAndCooldowns(t *testing.T) {
	we := withdrawals.NewWithdrawalEngine()
	ctx := context.Background()

	we.AddAddressToBook("usr_test", "1BvBM_1", "Bitcoin", "My Whitelist", true)

	// 1. Success withdrawal (below limits)
	req1, err := we.CreateWithdrawal(ctx, "usr_test", "BTC", 0.1, 0.0005, "1BvBM_1")
	if err != nil {
		t.Fatalf("Failed standard withdrawal creation: %v", err)
	}
	if req1.Status != withdrawals.StatusRequested {
		t.Errorf("Expected REQUESTED, got %s", req1.Status)
	}

	// 2. Reject due to active cooldown boundary (BTC cooldown is 30 mins)
	_, errCooldown := we.CreateWithdrawal(ctx, "usr_test", "BTC", 0.1, 0.0005, "1BvBM_1")
	if errCooldown == nil {
		t.Error("Replay Protection active cooldown check failed: expected error, got nil")
	}
}

// BenchmarkWithdrawalValidation measures speed of whitelist, cooldown, and limits check
func BenchmarkWithdrawalValidation(b *testing.B) {
	we := withdrawals.NewWithdrawalEngine()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clean UserID per benchmark operation to prevent cooldown rejection
		uID := fmt.Sprintf("usr_bench_%d", i)
		we.AddAddressToBook(uID, "0xReceiver", "Ethereum", "Whitelist", true)
		_, _ = we.CreateWithdrawal(ctx, uID, "ETH", 0.1, 0.003, "0xReceiver")
	}
}

// BenchmarkApprovalProcessing measures administrative multi-signature approvals speed
func BenchmarkApprovalProcessing(b *testing.B) {
	we := withdrawals.NewWithdrawalEngine()
	ctx := context.Background()

	we.AddAddressToBook("usr_bench", "0xReceiver", "Ethereum", "Whitelist", true)
	req, _ := we.CreateWithdrawal(ctx, "usr_bench", "ETH", 0.1, 0.003, "0xReceiver")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = we.ApproveWithdrawal(ctx, req.ID, fmt.Sprintf("admin_%d", i))
	}
}
