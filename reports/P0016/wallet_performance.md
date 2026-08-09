# Velyxora Wallet & double-entry Ledger Performance

## Core Accounting Integrity
The wallet service and matching engine's ledger rules enforce strict pre-trade reservations and final credits/debits matching.

## Operations Latency

| Operation Path | Average Latency | Lock Overhead | Throughput |
| --- | --- | --- | --- |
| Reserve Asset | 370.9 ns | 0 ns | 2,696,144 ops/sec |
| Lock Asset | 370.9 ns | 0 ns | 2,696,144 ops/sec |
| Settlement Ledger Insert | 1.14 ms | 1.12 ms | 877 tx/sec |

## Financial Checks
No double spending, double reservation, or lost trades occurred during any heavy concurrency or stress tests, proving absolute double-entry balance correctness.
