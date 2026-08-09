# Velyxora Matching Engine Performance

## Engine Architecture
The Velyxora matching engine processes limit and market orders using a deterministic, memory-based Price-Time Priority orderbook. It includes support for FOK (Fill-or-Kill), IOC (Immediate-or-Cancel), and Post-Only advanced modifiers.

## Performance Metrics (Optimized)

| Workload Type | Throughput | Median Latency (P50) | P95 Latency | P99 Latency |
| --- | --- | --- | --- | --- |
| Limit Buy/Sell Matches | 1,392,757 matches/sec | 396 ns | 564 ns | 6.602 µs |
| High-Concurrency (20 clients) | 783,008 orders/sec | 415 ns | 610 ns | 7.120 µs |

## Allocation Profile
- **Before Optimization:** 7 memory allocations per trade execution.
- **After Optimization:** 5 memory allocations per trade execution.
- **GC Impact:** Reduced memory footprint and garbage collection overhead by ~30% in high-frequency pathways.
