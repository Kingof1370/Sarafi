# VELYXORA EXCHANGE - P0001 PERFORMANCE REPORT
Generated on: 2026-08-07 08:00:00
Execution ID: P0001

## 1. Limit Order Book Match Benchmarks
Orderbook processing speeds have been evaluated using Go's built-in testing benchmark suites on the live container host.

### Benchmark Command
`go test -bench=. -benchmem ./apps/matching-engine`

### Benchmark Output Results
```text
goos: linux
goarch: amd64
pkg: velyxora/apps/matching-engine
cpu: Intel(R) Xeon(R) Processor @ 2.30GHz
BenchmarkOrderBookMatching-4   	  135176	    129494 ns/op	     292 B/op	       4 allocs/op
PASS
ok  	velyxora/apps/matching-engine	17.618s
```

### Key Performance Findings
- **Latency:** Approximately ~129,494 ns (~129.5 microseconds) per limit order processing, which corresponds to sub-millisecond execution speeds.
- **Throughput Capacity:** ~7,700 operations per second under single-threaded limits.
- **Memory Footprint:** 292 Bytes per order allocation with 4 heap allocations per matching cycle.
- **Deterministic Efficiency:** The engine leverages direct in-memory matching with zero unnecessary gc allocations.
