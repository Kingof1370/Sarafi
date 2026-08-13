package engine

import (
	"encoding/json"
	"testing"
	"time"
	"velyxora/packages/types"
)

func TestCanonicalOrderSchemaLosslessSerialization(t *testing.T) {
	// 1. Construct a fully configured types.Order with advanced, algorithmic fields
	originalOrder := types.Order{
		ID:            "ord_canonical_test_123",
		UserID:        "usr_alice_789",
		Symbol:        "BTC-USDT",
		Side:          types.SideBuy,
		Type:          types.TypeLimit,
		Price:         99500.50,
		Quantity:      1.5,
		FilledQty:     0.25,
		Status:        types.StatusNew,
		TimeInForce:   "FOK",
		PostOnly:      true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ClientOrderID: "cli_client_id_abc",
		ExternalRefID: "ext_reference_ref_def",
		ExecutionID:   "exec_leg_xyz",
		CorrelationID: "corr_id_tracing_jwt",
		StopPrice:     99000.0,
		TrailingDelta: 0.02,
		IcebergSize:   0.1,
		ReduceOnly:    true,
	}

	// 2. Serialize into JSON (simulating Kafka border serialization)
	payload, err := json.Marshal(originalOrder)
	if err != nil {
		t.Fatalf("Failed to serialize canonical Order struct: %v", err)
	}

	// 3. Deserialize back into types.Order
	var deserializedOrder types.Order
	if err := json.Unmarshal(payload, &deserializedOrder); err != nil {
		t.Fatalf("Failed to deserialize canonical Order struct: %v", err)
	}

	// 4. Assert that ALL advanced and standard fields are preserved without any pruning or data loss
	if deserializedOrder.ID != originalOrder.ID {
		t.Errorf("Mismatch in ID: expected %s, got %s", originalOrder.ID, deserializedOrder.ID)
	}
	if deserializedOrder.ClientOrderID != originalOrder.ClientOrderID {
		t.Errorf("Mismatch in ClientOrderID: expected %s, got %s", originalOrder.ClientOrderID, deserializedOrder.ClientOrderID)
	}
	if deserializedOrder.ExternalRefID != originalOrder.ExternalRefID {
		t.Errorf("Mismatch in ExternalRefID: expected %s, got %s", originalOrder.ExternalRefID, deserializedOrder.ExternalRefID)
	}
	if deserializedOrder.ExecutionID != originalOrder.ExecutionID {
		t.Errorf("Mismatch in ExecutionID: expected %s, got %s", originalOrder.ExecutionID, deserializedOrder.ExecutionID)
	}
	if deserializedOrder.CorrelationID != originalOrder.CorrelationID {
		t.Errorf("Mismatch in CorrelationID: expected %s, got %s", originalOrder.CorrelationID, deserializedOrder.CorrelationID)
	}
	if deserializedOrder.StopPrice != originalOrder.StopPrice {
		t.Errorf("Mismatch in StopPrice: expected %f, got %f", originalOrder.StopPrice, deserializedOrder.StopPrice)
	}
	if deserializedOrder.TrailingDelta != originalOrder.TrailingDelta {
		t.Errorf("Mismatch in TrailingDelta: expected %f, got %f", originalOrder.TrailingDelta, deserializedOrder.TrailingDelta)
	}
	if deserializedOrder.IcebergSize != originalOrder.IcebergSize {
		t.Errorf("Mismatch in IcebergSize: expected %f, got %f", originalOrder.IcebergSize, deserializedOrder.IcebergSize)
	}
	if deserializedOrder.ReduceOnly != originalOrder.ReduceOnly {
		t.Errorf("Mismatch in ReduceOnly: expected %v, got %v", originalOrder.ReduceOnly, deserializedOrder.ReduceOnly)
	}
	if deserializedOrder.PostOnly != originalOrder.PostOnly {
		t.Errorf("Mismatch in PostOnly: expected %v, got %v", originalOrder.PostOnly, deserializedOrder.PostOnly)
	}
	if deserializedOrder.TimeInForce != originalOrder.TimeInForce {
		t.Errorf("Mismatch in TimeInForce: expected %s, got %s", originalOrder.TimeInForce, deserializedOrder.TimeInForce)
	}

	// 5. Convert to engine.AdvancedOrder and verify fields map correctly inside matching engine consumer
	advOrder := &AdvancedOrder{
		ID:            deserializedOrder.ID,
		UserID:        deserializedOrder.UserID,
		Symbol:        deserializedOrder.Symbol,
		Side:          string(deserializedOrder.Side),
		Type:          string(deserializedOrder.Type),
		Price:         deserializedOrder.Price,
		Quantity:      deserializedOrder.Quantity,
		FilledQty:     deserializedOrder.FilledQty,
		Status:        StatusOMS_Created,
		TimeInForce:   TimeInForce(deserializedOrder.TimeInForce),
		CreatedAt:     deserializedOrder.CreatedAt,
		UpdatedAt:     deserializedOrder.UpdatedAt,
		ClientOrderID: deserializedOrder.ClientOrderID,
		ExternalRefID: deserializedOrder.ExternalRefID,
		ExecutionID:   deserializedOrder.ExecutionID,
		CorrelationID: deserializedOrder.CorrelationID,
		StopPrice:     deserializedOrder.StopPrice,
		TrailingDelta: deserializedOrder.TrailingDelta,
		IcebergSize:   deserializedOrder.IcebergSize,
		ReduceOnly:    deserializedOrder.ReduceOnly,
		PostOnly:      deserializedOrder.PostOnly,
	}

	if advOrder.ClientOrderID != originalOrder.ClientOrderID {
		t.Errorf("AdvancedOrder conversion lost ClientOrderID")
	}
	if advOrder.StopPrice != originalOrder.StopPrice {
		t.Errorf("AdvancedOrder conversion lost StopPrice")
	}
	if advOrder.ReduceOnly != originalOrder.ReduceOnly {
		t.Errorf("AdvancedOrder conversion lost ReduceOnly")
	}
}
