# PART 05 Remediation: Canonical Order Schema & Kafka Contract Preservation

Remediation was fully performed and verified to resolve **Issue 5 (Dropped Advanced Order Attributes)**.

## 1. Objective
Align order schema models and prevent loss of advanced algorithmic execution properties (such as StopPrice, PostOnly, IcebergSize, ReduceOnly, and ClientOrderID) over the boundaries of Apache Kafka brokers.

## 2. Remediation Details & Implementation
- Modified `packages/types/types.go`: Added all advanced and algorithmic fields directly to the standard canonical `types.Order` struct.
- Modified `apps/matching-engine/main.go`: Updated conversion logic from `types.Order` to `engine.AdvancedOrder` inside the Kafka order consumer routine to map and preserve all newly introduced fields.
- Modified `apps/backend/oms_handlers.go`: Updated both `handleCreateOrder` and `handleReplaceOrder` handler methods to preserve and populate these advanced fields inside the `types.Order` representation prior to serialization and submission into Kafka queues.

## 3. Verification & Testing
Created `apps/matching-engine/engine/oms_canonical_schema_test.go` and executed `TestCanonicalOrderSchemaLosslessSerialization` to explicitly verify that serializing a completely configured advanced order payload, transmitting it, and deserializing it into the engine preserves 100% of the attributes with zero data loss or conversion pruning. All tests compile and pass successfully.
