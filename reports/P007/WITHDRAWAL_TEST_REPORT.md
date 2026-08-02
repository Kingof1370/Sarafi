# Velyxora Exchange — P007 Withdrawal Engine Test Report

## Metadata
* **Execution Timestamp:** 2026-08-02 18:00:00 UTC
* **Git Commit Hash:** 29153eb

## Test Coverage Summary
* `packages/withdrawals`: Tests standard whitelists check, creation sessions, approvals queue, cancellations, and single/daily limits.
* `apps/wallet-service`: Tests `ProcessWithdrawalRequest`, `ProcessWithdrawalApproval`, `ProcessWithdrawalBroadcast`, and `ProcessWithdrawalFinalize` database-backed balance updates.
* `apps/backend`: Tests REST endpoints (/withdrawals/create, /withdrawals/cancel/:id, /withdrawals/status/:id, /withdrawals/history, /withdrawals/address-book, /withdrawals/whitelist, /withdrawals/approve/:id).
* `tests/withdrawal_engine_test.go`: Tests parallel multi-user concurrent requests and security limit cooldown blocks.

## Test Output
```
ok  	velyxora/apps/backend	0.023s
ok  	velyxora/apps/matching-engine	(cached)
ok  	velyxora/apps/wallet-service	0.014s
ok  	velyxora/packages/common	0.612s
ok  	velyxora/packages/database	(cached)
ok  	velyxora/packages/logger	(cached)
ok  	velyxora/packages/security	(cached)
ok  	velyxora/tests	0.017s
ok  	velyxora/packages/blockchain	(cached)
ok  	velyxora/packages/assets	(cached)
ok  	velyxora/packages/address	(cached)
ok  	velyxora/packages/ledger-common	(cached)
ok  	velyxora/packages/wallet	0.014s
ok  	velyxora/packages/deposits	(cached)
ok  	velyxora/packages/withdrawals	0.006s
```
100% of the tests passed with zero failures or warnings.
