package engine

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDurableSettlementQueueOutboxPersistence(t *testing.T) {
	// Initialize SettlementEngine with a mock/nil database to verify standalone in-memory outbox capabilities
	se := NewSettlementEngine(nil)

	exec := &Execution{
		TradeID:        "trade_durable_123",
		Symbol:         "BTC-USDT",
		Price:          95000.0,
		Quantity:       0.5,
		BuyerID:        "usr_buyer_alice",
		SellerID:       "usr_seller_bob",
		BuyerFee:       10.0,
		SellerFee:      5.0,
		BuyerFeeAsset:  "USDT",
		SellerFeeAsset: "USDT",
	}

	job := se.QueueSettlement(exec, "BTC", "USDT")

	if job.ID != "set_job_trade_durable_123" {
		t.Errorf("Unexpected SettlementJob ID: got %s", job.ID)
	}
	if job.Status != SettlePending {
		t.Errorf("Expected initial state PENDING, got %s", job.Status)
	}

	// Serialize/Deserialize verification to ensure full outbox payload persistence compatibility
	payloadBytes, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Failed to serialize SettlementJob: %v", err)
	}

	var deserializedJob SettlementJob
	if err := json.Unmarshal(payloadBytes, &deserializedJob); err != nil {
		t.Fatalf("Failed to deserialize SettlementJob: %v", err)
	}

	if deserializedJob.ID != job.ID {
		t.Errorf("Deserialized ID mismatch: got %s", deserializedJob.ID)
	}
	if deserializedJob.Exec.TradeID != exec.TradeID {
		t.Errorf("Deserialized nested Exec TradeID mismatch: got %s", deserializedJob.Exec.TradeID)
	}

	// Verify ProcessQueue clears the in-memory fallback successfully
	success, fail := se.ProcessQueue(context.Background())
	if success != 1 || fail != 0 {
		t.Errorf("Expected 1 successful and 0 failed processing, got success=%d, fail=%d", success, fail)
	}

	if se.GetPendingJobsCount() != 0 {
		t.Errorf("Expected pending jobs count to be 0, got %d", se.GetPendingJobsCount())
	}
	if se.GetSettledCount() != 1 {
		t.Errorf("Expected settled count to be 1, got %d", se.GetSettledCount())
	}
}
