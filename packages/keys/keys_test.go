package keys

import (
	"testing"
)

func TestKeyLifecycleAndRotations(t *testing.T) {
	km := NewKeyManager(nil)

	// 1. Generate local standard key
	key, err := km.GenerateNewKey(TypeEd25519, false)
	if err != nil {
		t.Fatalf("GenerateNewKey failed: %v", err)
	}

	if key.Status != StatusActive || key.Version != 1 {
		t.Errorf("Expected version 1 active, got status %s version %d", key.Status, key.Version)
	}

	// 2. Key Rotation
	newKey, err := km.RotateKey(key.ID)
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}

	if key.Status != StatusRotated {
		t.Errorf("Expected old key rotated, got status %s", key.Status)
	}

	if newKey.Status != StatusActive || newKey.Version != 2 {
		t.Errorf("Expected new key version 2 active, got status %s version %d", newKey.Status, newKey.Version)
	}

	// 3. Key Revocation
	revKey, err := km.RevokeKey(newKey.ID)
	if err != nil {
		t.Fatalf("RevokeKey failed: %v", err)
	}

	if revKey.Status != StatusRevoked {
		t.Errorf("Expected revoked status, got %s", revKey.Status)
	}
}

func TestThresholdSignatureRequestQueue(t *testing.T) {
	km := NewKeyManager(nil)

	rawData := []byte("v_sig_payload_99")
	req := km.CreateSignatureRequest(rawData, 2) // Require 2-of-3 approvals

	if req.Status != "PENDING" || req.RequiredApprovals != 2 {
		t.Errorf("Expected PENDING and 2 required approvals")
	}

	// 1st approval
	req, err := km.ApproveSignature(req.ID, "admin_user_1", []byte("partial_sig_1"))
	if err != nil {
		t.Fatalf("ApproveSignature failed: %v", err)
	}

	if req.Status != "PENDING" || req.CurrentApprovals != 1 {
		t.Errorf("Expected status still PENDING at 1/2 approvals, got %s", req.Status)
	}

	// 2nd approval (threshold achieved)
	req, err = km.ApproveSignature(req.ID, "admin_user_2", []byte("partial_sig_2"))
	if err != nil {
		t.Fatalf("ApproveSignature failed on 2nd approval: %v", err)
	}

	if req.Status != "COMPLETED" || req.CurrentApprovals != 2 {
		t.Errorf("Expected status COMPLETED on threshold achievement, got %s", req.Status)
	}
}
