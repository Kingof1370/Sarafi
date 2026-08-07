package assets

import (
	"context"
	"testing"
)

func TestAssetRegistry(t *testing.T) {
	ar := NewAssetRegistry()

	ast := &Asset{
		Symbol:      "BTC",
		Name:        "Bitcoin",
		Type:        TypeNativeCoin,
		Precision:   8,
		BaseNetwork: "Bitcoin",
		IsActive:    true,
	}

	meta := &AssetMetadata{
		Symbol:      "BTC",
		Description: "Digital Gold",
		Website:     "bitcoin.org",
	}

	perm := &AssetPermission{
		Symbol:             "BTC",
		CanDeposit:         true,
		CanWithdraw:        true,
		CanTrade:           true,
		MinDepositAmount:   0.001,
		MinWithdrawAmount:  0.002,
		MaxDailyWithdrawal: 5.0,
		WithdrawalFee:      0.0005,
	}

	err := ar.RegisterAsset(ast, meta, perm)
	if err != nil {
		t.Fatalf("RegisterAsset failed: %v", err)
	}

	// Fetch validations
	fetchedAst, err := ar.GetAsset("BTC")
	if err != nil {
		t.Errorf("GetAsset failed: %v", err)
	} else if fetchedAst.Name != "Bitcoin" {
		t.Errorf("Expected Bitcoin name, got %s", fetchedAst.Name)
	}

	fetchedMeta, err := ar.GetMetadata("BTC")
	if err != nil {
		t.Errorf("GetMetadata failed: %v", err)
	} else if fetchedMeta.Description != "Digital Gold" {
		t.Errorf("Expected Digital Gold description, got %s", fetchedMeta.Description)
	}

	fetchedPerm, err := ar.GetPermissions("BTC")
	if err != nil {
		t.Errorf("GetPermissions failed: %v", err)
	} else if !fetchedPerm.CanTrade {
		t.Errorf("Expected trade capability to be enabled")
	}

	assetsList := ar.ListAssets(context.Background())
	if len(assetsList) != 1 {
		t.Errorf("Expected assets list size 1, got %d", len(assetsList))
	}
}

func TestAssetRegistryErrors(t *testing.T) {
	ar := NewAssetRegistry()
	_, err := ar.GetAsset("USDT")
	if err == nil {
		t.Error("Expected error fetching unregistered asset, got nil")
	}
}
