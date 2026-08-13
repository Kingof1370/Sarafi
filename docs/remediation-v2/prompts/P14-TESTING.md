# Prompt: P14 — Integration & Failure Testing

## 1. Objective
Establish complete automated integration and failure testing coverage across all critical-path exchange execution flows.

## 2. Current Problem
- Tests predominantly focused on single-module unit checks with less focus on E2E integration failure coverage.

## 3. Root Cause
Incomplete end-to-end integration test setup across multiple coupled microservices.

## 4. Affected Modules
- `tests/`

## 5. Affected Files
- `tests/remediation_integration_test.go`
- `tests/integration_test.go`

## 6. Architectural & Technical Requirements
- Write comprehensive integration scenarios.
- Verify system performance under failure scenarios: duplicate execution, duplicate settlement, consumer restart, database restart, process crash, and Kafka retry.

## 7. Migration & Backward Compatibility
- Backwards compatible with standard `go test` workspace runs.

## 8. Security & Financial Requirements
- Confirm that the final ledger states pass global accounting audit reconciliations with zero discrepancy down to the 18th decimal place.

## 9. Tests & Failure Scenarios
- Confirm system behaves as expected under high-frequency load testing.

## 10. Acceptance Criteria & Definition of Done
- Complete automated E2E integration test runs pass successfully.
- Completed with Git commit.
