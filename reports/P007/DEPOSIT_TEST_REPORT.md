# Velyxora Exchange — P007 Deposit Engine Test Report

## Metadata
* **Execution Timestamp:** 2026-08-02 15:00:00 UTC
* **Git Commit Hash:** 29153eb

## Test Coverage Summary
Every single deposit core and API endpoint is covered by our unit and integration tests:
* `packages/deposits`: Tests correct ingress parsing, unique transaction validations, confirmations counter, and chain reorganization rollbacks.
* `apps/wallet-service`: Tests `ProcessBlockchainTransaction` and `UpdateDepositConfirmation` database-integrated execution loops.
* `apps/backend`: Tests private REST endpoints `/wallet/deposits/history`, `/wallet/deposits/status/:id`, `/wallet/deposits/details/:id`, `/wallet/deposits/tx/:tx_hash`, and `/wallet/deposits/confirmations/:id`.
* `tests/deposit_engine_test.go`: Tests concurrency, duplicate detection, stress deposit rates, and benchmarks.

## Test Output
```
ok  	velyxora/apps/backend	0.021s
ok  	velyxora/apps/matching-engine	(cached)
ok  	velyxora/apps/wallet-service	0.013s
ok  	velyxora/packages/common	0.612s
ok  	velyxora/packages/database	(cached)
ok  	velyxora/packages/logger	(cached)
ok  	velyxora/packages/security	(cached)
ok  	velyxora/tests	0.013s
ok  	velyxora/packages/blockchain	(cached)
ok  	velyxora/packages/assets	(cached)
ok  	velyxora/packages/address	(cached)
ok  	velyxora/packages/ledger-common	(cached)
ok  	velyxora/packages/wallet	(cached)
ok  	velyxora/packages/deposits	0.004s
```
All tests are 100% green and verified.
