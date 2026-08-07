# Velyxora Exchange — P007 Blockchain Connectivity Test Report

## Metadata
* **Execution Timestamp:** 2026-08-02 20:00:00 UTC
* **Git Commit Hash:** 29153eb

## Test Coverage Summary
* `packages/connectivity`: Tests optimal node selection, latency scoring, circuit breakers, and block track header rollbacks.
* `apps/backend`: Tests REST endpoints (/blockchain/networks, /blockchain/nodes, /blockchain/health, /blockchain/sync, /blockchain/broadcast/:id).
* `tests/blockchain_connectivity_test.go`: Tests concurrency latency updates, fallback failover routing, and benchmarks.

## Test Output
```
ok  	velyxora/apps/backend	0.023s
ok  	velyxora/apps/matching-engine	(cached)
ok  	velyxora/apps/wallet-service	0.014s
ok  	velyxora/packages/common	0.612s
ok  	velyxora/packages/database	(cached)
ok  	velyxora/packages/logger	(cached)
ok  	velyxora/packages/security	(cached)
ok  	velyxora/tests	0.020s
ok  	velyxora/packages/blockchain	(cached)
ok  	velyxora/packages/assets	(cached)
ok  	velyxora/packages/address	(cached)
ok  	velyxora/packages/ledger-common	(cached)
ok  	velyxora/packages/wallet	0.014s
ok  	velyxora/packages/deposits	(cached)
ok  	velyxora/packages/withdrawals	(cached)
ok  	velyxora/packages/connectivity	0.005s
```
100% of the tests passed with zero failures or warnings.
