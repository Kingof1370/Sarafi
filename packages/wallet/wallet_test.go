package wallet

import (
	"testing"
)

func TestWalletProvisionAndBalances(t *testing.T) {
	wm := NewWalletManager()

	w, err := wm.ProvisionWallet("wal_hot_01", "usr_01", TypeHot)
	if err != nil {
		t.Fatalf("ProvisionWallet failed: %v", err)
	}

	if w.ID != "wal_hot_01" || w.Type != TypeHot {
		t.Errorf("Mismatch in provisioned wallet values")
	}

	// Update balance segment
	err = wm.UpdateBalance("wal_hot_01", "BTC", 2.5, 0, 0, 0)
	if err != nil {
		t.Fatalf("UpdateBalance failed: %v", err)
	}

	bal := w.Balances["BTC"]
	if bal.Available != 2.5 || bal.Total != 2.5 {
		t.Errorf("Expected balance 2.5, got available %f total %f", bal.Available, bal.Total)
	}

	// Verify negative balance segment is rejected
	err = wm.UpdateBalance("wal_hot_01", "BTC", -5.0, 0, 0, 0)
	if err == nil {
		t.Error("Expected error when updating to negative balance, got nil")
	}
}

func TestOperationalLimits(t *testing.T) {
	wm := NewWalletManager()

	// Provision source and destination
	_, _ = wm.ProvisionWallet("wal_op_01", "usr_01", TypeOperational)
	_, _ = wm.ProvisionWallet("wal_hot_02", "usr_02", TypeHot)

	_ = wm.UpdateBalance("wal_op_01", "USDT", 1000.0, 0, 0, 0)
	_ = wm.UpdateBalance("wal_hot_02", "USDT", 0, 0, 0, 0)

	// Set limits
	err := wm.SetLimit("wal_op_01", 100.0, 500.0)
	if err != nil {
		t.Fatalf("SetLimit failed: %v", err)
	}

	// Try single limit violation
	_, err = wm.InitiateTransfer("tx_01", "wal_op_01", "wal_hot_02", "USDT", 150.0, "usr_01")
	if err == nil {
		t.Error("Expected error due to single transaction limit, got nil")
	}

	// Try valid transfer
	ok, err := wm.InitiateTransfer("tx_02", "wal_op_01", "wal_hot_02", "USDT", 80.0, "usr_01")
	if err != nil {
		t.Fatalf("InitiateTransfer failed: %v", err)
	}
	if !ok {
		t.Error("Transfer failed to execute")
	}

	// Try daily limit violation with cumulative transfers
	_, _ = wm.InitiateTransfer("tx_03", "wal_op_01", "wal_hot_02", "USDT", 80.0, "usr_01")
	_, _ = wm.InitiateTransfer("tx_04", "wal_op_01", "wal_hot_02", "USDT", 80.0, "usr_01")
	_, _ = wm.InitiateTransfer("tx_05", "wal_op_01", "wal_hot_02", "USDT", 80.0, "usr_01")
	_, _ = wm.InitiateTransfer("tx_06", "wal_op_01", "wal_hot_02", "USDT", 80.0, "usr_01")
	_, _ = wm.InitiateTransfer("tx_07", "wal_op_01", "wal_hot_02", "USDT", 80.0, "usr_01")

	// Next transfer must violate cumulative limit (spent 80 * 6 = 480, next 80 makes it 560 > 500)
	_, err = wm.InitiateTransfer("tx_08", "wal_op_01", "wal_hot_02", "USDT", 80.0, "usr_01")
	if err == nil {
		t.Error("Expected error due to cumulative daily spending limit, got nil")
	}
}

func TestColdAndTreasuryTransferAuthorizations(t *testing.T) {
	wm := NewWalletManager()

	_, _ = wm.ProvisionWallet("wal_cold", "usr_cold", TypeCold)
	_, _ = wm.ProvisionWallet("wal_treasury", "usr_treasury", TypeTreasury)
	_, _ = wm.ProvisionWallet("wal_hot", "usr_hot", TypeHot)

	_ = wm.UpdateBalance("wal_cold", "ETH", 10.0, 0, 0, 0)
	_ = wm.UpdateBalance("wal_treasury", "ETH", 10.0, 0, 0, 0)

	// 1. Cold Wallet direct transfer should fail
	_, err := wm.InitiateTransfer("tx_cold_01", "wal_cold", "wal_hot", "ETH", 1.0, "usr_cold")
	if err == nil {
		t.Error("Expected cold transfer to be rejected directly, got nil")
	}

	// 2. Treasury transfer with insufficient approvals should fail
	_, err = wm.InitiateTransfer("tx_tr_01", "wal_treasury", "wal_hot", "ETH", 2.0, "usr_treasury")
	if err == nil {
		t.Error("Expected treasury transfer to fail due to insufficient approvals, got nil")
	}

	// Approve by Admin 1 and Admin 2
	wm.ApproveTransfer("tx_tr_01", "admin_01")
	wm.ApproveTransfer("tx_tr_01", "admin_02")

	// Treasury transfer should now succeed
	ok, err := wm.InitiateTransfer("tx_tr_01", "wal_treasury", "wal_hot", "ETH", 2.0, "usr_treasury")
	if err != nil {
		t.Fatalf("Treasury transfer failed after correct approvals: %v", err)
	}
	if !ok {
		t.Error("Treasury transfer failed")
	}
}
