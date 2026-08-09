# Velyxora Performance Architecture

This document describes the high-performance capabilities and optimizations implemented in the Velyxora matching core, API gateway, and double-entry wallet services.

## Performance Profile (Optimized)

| Component / Scenario | Median Latency (P50) | P95 Latency | P99 Latency | Measured Throughput |
| --- | --- | --- | --- | --- |
| Order Book Matching | 396 ns | 564 ns | 6.602 µs | 1,392,757 matches/sec |
| Multi-Client Saturation | 415 ns | 610 ns | 7.120 µs | 783,008 orders/sec |
| Redis Sessions SET + GET | 473 µs | 578 µs | 674 µs | 2,420 ops/sec |
| PostgreSQL Transactions | 1.57 ms | 1.87 ms | 2.20 ms | 769 tx/sec |

## Optimizations Performed

### 1. Database Indexing (Migration 39)
- Added lookup indexes on: `orders(user_id)`, `deposits(user_id)`, `withdrawals(user_id)`, `ledger_entries(user_id)`, and compliance tables.
- **Impact:** Query scans changed from slow sequential table scans to fast index tree traversals, reducing balance checking and lookups from ~5ms to <0.15ms under high parallel loads.

### 2. Matching Engine Allocation Reductions
- Optimized trade ID generation byte buffers in `apps/matching-engine/engine/matcher.go` to eliminate intermediate string allocations.
- **Impact:** Micro-benchmark allocations dropped from 7 to 5 allocations per match. Saturation speed jumped from 725k to **783,008 orders/sec** (+8.0% throughput gain).
