# Prompt: Integration Testing & Frontend Connection Verification (PART-08)

## Objective
Remediet the integration gaps between Frontend visual components and Backend real-time APIs. Ensure that all terminal widgets (Order book, Trades, Charts, Faucet, Balances, and KYC) display actual, live data from REST and WebSocket gateways rather than mock data fallbacks, except when explicitly labeled as local simulation mode. Write complete automated test suites to verify system behavior from E2E perspective.

## Scope
- Validate frontend endpoints mapping in `apps/frontend/src/app/page.tsx` and `apps/frontend/src/lib/api.ts`.
- Connect and verify websocket and REST subscriptions.
- Write robust end-to-end integration test scenarios.

## Requirements & Specifications

### 1. Actual End-to-End WebSocket Connection
- Verify that the frontend subscribes to actual L2 Depth events published via Kafka over the REST Gateway WebSocket (`/ws`).
- Ensure that order placements and balance modifications on the backend trigger instant, real-time visual updates to the user balances card and order book widgets in the UI.

### 2. Mock Elimination in Frontend Contract
- Replace all static, hardcoded frontend elements with dynamic queries hitting `/api/v1/wallet/balances`, `/api/v1/compliance/kyc/status`, and related endpoints.
- Ensure that actual compliance restriction checks are gracefully caught and visually notified to the end user inside the Trading Terminal.

### 3. Comprehensive Automation Testing (E2E)
- Write an E2E execution flow test that spins up API Gateway, runs matching engine routines, registers a user, submits a valid deposit, submits dynamic order transactions, matches them, triggers the durable settlement queue, and asserts that balance changes reflect perfectly in the DB ledger.

## Testing Strategy
- Create multi-service integration test files under `tests/remediation_integration_test.go`:
  - Verify complete sequence matching and order fill flows over Kafka.
  - Assert that websocket clients receive L2 depth snapshots and trade alerts synchronously after executions.
  - Confirm that the final ledger states pass global accounting audit reconciliations.
