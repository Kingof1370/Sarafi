# Prompt: Canonical Order Schema & Kafka Contract Preservation (PART-05)

## Objective
Remediet the schema mismatch and loss of advanced order attributes across Kafka borders. Currently, complex orders configured with advanced execution parameters (such as `TimeInForce`, `StopPrice`, `IcebergSize`, `PostOnly`, `ReduceOnly`, and `ClientOrderID`) lose these properties when sent through Kafka. This is because they are cast into basic `types.Order` structs and then parsed back as basic `types.Order` inside the Matching Engine. Align and unify the transaction pipeline with a canonical schema representation.

## Scope
- Modify `packages/types/types.go` to support all advanced execution fields.
- Align schema definitions across `apps/backend/main.go`, `apps/matching-engine/main.go`, and internal matching engine types.
- Ensure proper serialization and deserialization over Kafka brokers without dropping data.

## Requirements & Specifications

### 1. Canonical Order Struct Alignment
- Establish `types.Order` as the absolute canonical schema model.
- Add and map all necessary trading parameters inside `types.Order` directly:
  - `TimeInForce` (e.g., GTC, IOC, FOK)
  - `StopPrice` (for STOP, STOP_LIMIT, TAKE_PROFIT, and TAKE_PROFIT_LIMIT)
  - `IcebergSize` (for hidden execution limits)
  - `PostOnly` (for strict maker execution)
  - `ReduceOnly` (for state-reducing derivatives/positions)
  - `ClientOrderID` (for client tracking/tracking)
- Remove intermediate conversions that prune or drop fields during transmission.

### 2. Lossless Serialization and Event Contracts
- Ensure that the REST Gateway correctly parses the complete advanced order JSON payload, populates the aligned canonical struct, and serializes it in its entirety into Kafka's `velyxora-orders` queue.
- Inside the Matching Engine, ensure that the Kafka event consumer parses the full JSON payload back into the exact matching engine implementation structure (`AdvancedOrder`) with 100% preservation of all advanced attributes.

### 3. Engine Processing Capability
- The matching engine must be able to utilize these preserved properties (like `PostOnly` or `TimeInForce = FOK`) directly based on the fields transmitted via Kafka, instead of fallback defaults.

## Testing Strategy
- Create serialization and schema alignment unit tests:
  - Write a test that serializes an advanced order (containing custom stop prices, post-only boolean flag, and a specific client order ID) into Kafka event JSON, deserializes it back, and asserts that all attributes remain identical and untruncated.
  - Verify that the matching engine correctly receives and enforces `PostOnly` or `FOK` execution limits on orders received via the Kafka consumer loop.
