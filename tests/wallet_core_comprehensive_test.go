package tests

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"velyxora/packages/address"
	"velyxora/packages/assets"
	"velyxora/packages/blockchain"
	"velyxora/packages/wallet"
)

// TestWalletCoreConcurrency verifies thread-safe parallel processing and blockchain address allocations
func TestWalletCoreConcurrency(t *testing.T) {
	wm := wallet.NewWalletManager()

	// Provision warm operational wallet and a receiver hot wallet
	_, _ = wm.ProvisionWallet("wal_source", "usr_src", wallet.TypeOperational)
	_, _ = wm.ProvisionWallet("wal_dest", "usr_dst", wallet.TypeHot)

	_ = wm.UpdateBalance("wal_source", "USDT", 10000.0, 0, 0, 0)
	_ = wm.UpdateBalance("wal_dest", "USDT", 0, 0, 0, 0)

	_ = wm.SetLimit("wal_source", 500.0, 10000.0)

	var wg sync.WaitGroup
	concurrencyLimit := 20
	wg.Add(concurrencyLimit)

	// Launch 20 concurrent transfers of 50.0 USDT each (total 1000.0 USDT)
	for i := 0; i < concurrencyLimit; i++ {
		go func(idx int) {
			defer wg.Done()
			txID := fmt.Sprintf("tx_concurrent_%d", idx)
			_, _ = wm.InitiateTransfer(txID, "wal_source", "wal_dest", "USDT", 50.0, "usr_src")
		}(i)
	}

	wg.Wait()

	srcW, _ := wm.GetWallet("wal_source")
	dstW, _ := wm.GetWallet("wal_dest")

	if srcW.Balances["USDT"].Available != 9000.0 {
		t.Errorf("Expected remaining source balance to be 9000.0, got %f", srcW.Balances["USDT"].Available)
	}

	if dstW.Balances["USDT"].Available != 1000.0 {
		t.Errorf("Expected destination balance to be 1000.0, got %f", dstW.Balances["USDT"].Available)
	}

	// Verify all audit logs are correct and chained properly
	logs := wm.ListAuditLogs()
	if len(logs) < 40 { // Wallet provisioning + updates + transfers * 2 (debit and credit entries)
		t.Errorf("Expected substantial audit logs, got %d", len(logs))
	}

	// Validate entire hash chain integrity
	var prev *any // Can be compared easily by chaining loop
	_ = prev
	for idx, log := range logs {
		if idx > 0 {
			ok, err := log.ValidateIntegrity(logs[idx-1])
			if err != nil || !ok {
				t.Fatalf("Audit chain integrity broken at log index %d: %v", idx, err)
			}
		}
	}
}

// TestAssetRegistryAndPermissions verifies that the Assets package rejects unlawful actions
func TestAssetRegistryAndPermissions(t *testing.T) {
	ar := assets.NewAssetRegistry()

	ast := &assets.Asset{
		Symbol:      "BTC",
		Name:        "Bitcoin",
		Type:        assets.TypeNativeCoin,
		Precision:   8,
		BaseNetwork: "Bitcoin",
		IsActive:    true,
	}

	perm := &assets.AssetPermission{
		Symbol:             "BTC",
		CanDeposit:         true,
		CanWithdraw:        false, // Withdrawals restricted for security validation
		CanTrade:           true,
		MinDepositAmount:   0.001,
		MinWithdrawAmount:  0.002,
		MaxDailyWithdrawal: 10.0,
		WithdrawalFee:      0.0005,
	}

	_ = ar.RegisterAsset(ast, nil, perm)

	p, err := ar.GetPermissions("BTC")
	if err != nil {
		t.Fatalf("Failed to fetch permissions: %v", err)
	}

	if p.CanWithdraw {
		t.Error("Asset permission restriction failed: expected withdrawals to be forbidden")
	}
}

// TestAddressRegistryAndOwnership verifies HD key address registry lookup and verification
func TestAddressRegistryAndOwnership(t *testing.T) {
	reg := address.NewAddressRegistry()
	blockchainReg := blockchain.NewAdapterRegistry()
	ethAdapter, _ := blockchainReg.Get("Ethereum")

	privKey := []byte("deterministic-private-key-phrase")
	pubKey := ethAdapter.DerivePublicKey(privKey)
	addressString, _ := ethAdapter.GenerateAddress(privKey)

	rec := &address.AddressRecord{
		UserID:         "usr_concurrency_test",
		Network:        "Ethereum",
		Address:        addressString,
		PublicKey:      pubKey,
		DerivationPath: "m/44'/60'/0'/0/0",
		Status:         address.StatusAllocated,
		CreatedAt:      time.Now(),
	}

	_ = reg.RegisterAddress(rec, nil)

	// Validate lookup
	fetched, err := reg.LookupByAddress(addressString)
	if err != nil || fetched.UserID != "usr_concurrency_test" {
		t.Errorf("Address lookup mismatch")
	}

	// Verify cryptographic signature ownership validation
	challenge := []byte("challenge_string_abc_123")
	sig, _ := ethAdapter.SignTransaction(privKey, challenge)

	ok, err := reg.VerifyOwnership(ethAdapter, addressString, challenge, sig)
	if err != nil || !ok {
		t.Errorf("Address verification failed: %v", err)
	}
}

// BenchmarkWalletLookup measures wallet resolution speeds under load
func BenchmarkWalletLookup(b *testing.B) {
	wm := wallet.NewWalletManager()
	for i := 0; i < 1000; i++ {
		wID := fmt.Sprintf("wal_%d", i)
		_, _ = wm.ProvisionWallet(wID, "usr_bench", wallet.TypeHot)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wID := fmt.Sprintf("wal_%d", i%1000)
		_, _ = wm.GetWallet(wID)
	}
}

// BenchmarkAddressGeneration measures performance of blockchain address derivations
func BenchmarkAddressGeneration(b *testing.B) {
	registry := blockchain.NewAdapterRegistry()
	adapter, _ := registry.Get("Ethereum")
	seed := []byte("deterministic-bench-32-byte-seed")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.GenerateAddress(seed)
	}
}

// BenchmarkBalanceCalculation measures available balance math updates
func BenchmarkBalanceCalculation(b *testing.B) {
	wm := wallet.NewWalletManager()
	_, _ = wm.ProvisionWallet("wal_bench", "usr_bench", wallet.TypeHot)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = wm.UpdateBalance("wal_bench", "BTC", 1.0, 0, 0, 0)
	}
}
