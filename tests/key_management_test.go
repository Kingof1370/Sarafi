package tests

import (
	"fmt"
	"sync"
	"testing"

	"velyxora/packages/keys"
)

// TestKeyManagerConcurrency verifies parallel key creation and concurrent multi-sig approvals
func TestKeyManagerConcurrency(t *testing.T) {
	km := keys.NewKeyManager(nil)

	concurrencyLimit := 30
	var wg sync.WaitGroup
	wg.Add(concurrencyLimit)

	for i := 0; i < concurrencyLimit; i++ {
		go func(idx int) {
			defer wg.Done()
			_, _ = km.GenerateNewKey(keys.TypeECDSA, false)
		}(i)
	}

	wg.Wait()

	list := km.ListKeys()
	if len(list) != concurrencyLimit {
		t.Errorf("Expected %d created keys, got %d", concurrencyLimit, len(list))
	}
}

// TestThresholdSignatureConcurrency verifies parallel admin approvals under heavy concurrent load on same signature request
func TestThresholdSignatureConcurrency(t *testing.T) {
	km := keys.NewKeyManager(nil)

	req := km.CreateSignatureRequest([]byte("tx_payload_data"), 20) // Require 20 approvals

	concurrencyLimit := 30
	var wg sync.WaitGroup
	wg.Add(concurrencyLimit)

	for i := 0; i < concurrencyLimit; i++ {
		go func(idx int) {
			defer wg.Done()
			adminID := fmt.Sprintf("admin_user_%d", idx)
			_, _ = km.ApproveSignature(req.ID, adminID, []byte(fmt.Sprintf("partial_sig_%d", idx)))
		}(i)
	}

	wg.Wait()

	if req.Status != "COMPLETED" || req.CurrentApprovals < 20 {
		t.Errorf("Expected status COMPLETED on threshold achievement, got %s, approvals %d", req.Status, req.CurrentApprovals)
	}
}

// BenchmarkKeyGeneration measures performance of local standard keygen
func BenchmarkKeyGeneration(b *testing.B) {
	km := keys.NewKeyManager(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = km.GenerateNewKey(keys.TypeECDSA, false)
	}
}

// BenchmarkSignatureCreation measures latency to register a signature request and add partial signatures
func BenchmarkSignatureCreation(b *testing.B) {
	km := keys.NewKeyManager(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := km.CreateSignatureRequest([]byte("payload"), 1)
		_, _ = km.ApproveSignature(req.ID, "admin", []byte("sig"))
	}
}

// BenchmarkKeyLookup measures speed of registry key queries
func BenchmarkKeyLookup(b *testing.B) {
	km := keys.NewKeyManager(nil)
	for i := 0; i < 1000; i++ {
		_, _ = km.GenerateNewKey(keys.TypeEd25519, false)
	}

	keysList := km.ListKeys()
	targetID := keysList[0].ID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = km.LookupKey(targetID)
	}
}
