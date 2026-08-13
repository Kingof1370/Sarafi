package main

import (
	"context"
	"testing"
	"time"

	"velyxora/packages/logger"
)

func TestWalletWorkerInitAndStop(t *testing.T) {
	log := logger.NewLogger(logger.Config{
		Level:       "INFO",
		Format:      "JSON",
		ServiceName: "wallet-service-test",
	})

	worker := NewWalletWorker(nil, log)
	if worker == nil {
		t.Fatal("Expected NewWalletWorker to return non-nil value")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Run the worker loop in the background and stop it shortly after
	done := make(chan struct{})
	go func() {
		worker.Start(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success, stopped cleanly
	case <-time.After(1 * time.Second):
		t.Error("WalletWorker did not shut down within 1 second after context cancel")
	}
}

func TestWalletWorkerNilDbSafety(t *testing.T) {
	log := logger.NewLogger(logger.Config{
		Level:       "INFO",
		Format:      "JSON",
		ServiceName: "wallet-service-test",
	})

	worker := NewWalletWorker(nil, log)

	// Test that scanning blocks with nil db exits gracefully without any panic
	err := worker.ScanBlocks(context.Background())
	if err != nil {
		t.Errorf("Expected nil error when scanning with nil db, got: %v", err)
	}

	// Test that confirming deposits with nil db exits gracefully without any panic
	err = worker.ConfirmDeposits(context.Background())
	if err != nil {
		t.Errorf("Expected nil error when confirming with nil db, got: %v", err)
	}
}
