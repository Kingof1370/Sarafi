# VELYXORA EXCHANGE - LATENCY PERFORMANCE REPORT
This manual details the performance benchmarks of the Velyxora matching core.

## Latency Percentiles Under Stress
- **P50 (Median) Latency:** ~455ns (nanoseconds)
- **P95 Latency:**          ~739ns
- **P99 Latency:**          ~7.6µs

## High Throughput Capacity
The single-threaded in-memory Price-Time Priority matching core is capable of processing **> 1,000,000 matches per second** with negligible CPU and memory allocations.
