package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"velyxora/packages/deposits"
)

// TestDepositEngineConcurrency verifies thread-safe parallel deposit validations
func TestDepositEngineConcurrency(t *testing.T) {
	de := deposits.NewDepositEngine()
	ctx := context.Background()

	concurrencyLimit := 50
	var wg sync.WaitGroup
	wg.Add(concurrencyLimit)

	for i := 0; i < concurrencyLimit; i++ {
		go func(idx int) {
			defer wg.Done()
			tx := &deposits.BlockchainTx{
				TxHash:      fmt.Sprintf("0xhash_concurrent_%d", idx),
				Network:     "Ethereum",
				Asset:       "ETH",
				Amount:      1.5,
				Sender:      "0xSender",
				Receiver:    "0xReceiver",
				BlockNumber: int64(1000 + idx),
				Timestamp:   time.Now(),
			}
			_, _ = de.ValidateAndDetectIngress(ctx, tx, fmt.Sprintf("usr_%d", idx))
		}(i)
	}

	wg.Wait()

	list := de.ListDeposits()
	if len(list) != concurrencyLimit {
		t.Errorf("Expected %d processed deposits, got %d", concurrencyLimit, len(list))
	}
}

// TestDepositReplayAttackblocking confirms that duplicate transaction hashes are blocked
func TestDepositReplayAttackblocking(t *testing.T) {
	de := deposits.NewDepositEngine()
	ctx := context.Background()

	tx := &deposits.BlockchainTx{
		TxHash:      "0xreplay_hash_111",
		Network:     "Bitcoin",
		Asset:       "BTC",
		Amount:      0.25,
		Sender:      "Sender",
		Receiver:    "Receiver",
		BlockNumber: 450,
		Timestamp:   time.Now(),
	}

	// 1. Success ingress
	_, err := de.ValidateAndDetectIngress(ctx, tx, "usr_1")
	if err != nil {
		t.Fatalf("Failed standard ingress: %v", err)
	}

	// 2. Reject double spending replay attack
	_, errReplay := de.ValidateAndDetectIngress(ctx, tx, "usr_2")
	if errReplay == nil {
		t.Error("Replay Protection failure: duplicate transaction hash was accepted")
	}
}

// BenchmarkDepositProcessing measures speed of validation and ingress
func BenchmarkDepositProcessing(b *testing.B) {
	de := deposits.NewDepositEngine()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx := &deposits.BlockchainTx{
			TxHash:      fmt.Sprintf("0xbench_hash_%d", i),
			Network:     "Solana",
			Asset:       "SOL",
			Amount:      10.0,
			Sender:      "Sender",
			Receiver:    "Receiver",
			BlockNumber: int64(i),
			Timestamp:   time.Now(),
		}
		_, _ = de.ValidateAndDetectIngress(ctx, tx, "usr_bench")
	}
}

// BenchmarkConfirmationProcessing measures speed of block height updates
func BenchmarkConfirmationProcessing(b *testing.B) {
	de := deposits.NewDepositEngine()
	ctx := context.Background()

	tx := &deposits.BlockchainTx{
		TxHash:      "0xbench_hash_conf",
		Network:     "Ethereum",
		Asset:       "ETH",
		Amount:      1.5,
		Sender:      "Sender",
		Receiver:    "Receiver",
		BlockNumber: 1000,
		Timestamp:   time.Now(),
	}
	_, _ = de.ValidateAndDetectIngress(ctx, tx, "usr_bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = de.IncrementConfirmations(ctx, tx.TxHash, int64(1000+i))
	}
}
