# Prompt: P04 — Canonical Order Pipeline

## 1. Objective
Ensure lossless serialization and mapping of advanced, algorithmic order fields across all service borders.

## 2. Current Problem
- Properties like StopPrice, PostOnly, IcebergSize, and ReduceOnly were dropped when orders were sent as basic `types.Order` structs over Kafka.

## 3. Root Cause
Incomplete schema definitions inside common types, causing pruning during payload json marshaling.

## 4. Affected Modules
- `packages/types/`
- `apps/backend/`
- `apps/matching-engine/`

## 5. Affected Files
- `packages/types/types.go`
- `apps/backend/oms_handlers.go`
- `apps/matching-engine/main.go`

## 6. Architectural & Technical Requirements
- Align `types.Order` structure to contain all advanced fields.
- Map these properties prior to Kafka publishing and restore them inside matching-engine order consumer routines.

## 7. Migration & Backward Compatibility
- Backwards compatible with standard limit/market order parameters.

## 8. Security & Financial Requirements
- Retain exact execution rules (like `PostOnly` or `StopPrice`) to prevent invalid executions.

## 9. Tests & Failure Scenarios
- Write tests confirming that serializing an advanced order to JSON, publishing it, and deserializing it preserves 100% of fields without pruning.

## 10. Acceptance Criteria & Definition of Done
- Lossless serialization is verified.
- Advanced execution parameters are supported in matching engine pipelines.
- Completed with Git commit.
