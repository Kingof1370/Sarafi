# VELYXORA EXCHANGE - P001 V2 PERFORMANCE REPORT
Generated on: 2026-08-01 10:52:25
Execution ID: P001

## 1. Limit Order Book Match Benchmarks
Orderbook processing speeds have been evaluated using Go's built-in testing benchmark suites.

### Benchmark Command
`go test -bench=. -benchmem ./apps/matching-engine`

### Benchmark Output Results
```text
goos: linux
goarch: amd64
pkg: velyxora/apps/matching-engine
cpu: Intel(R) Xeon(R) Processor @ 2.30GHz
BenchmarkOrderBookMatching-4   	  144355	    133832 ns/op	     273 B/op	       4 allocs/op
PASS
ok  	velyxora/apps/matching-engine	19.422s

```

### Key Performance Findings
- **Latency:** Approximately ~133,000 ns per order limit processing, enabling highly fast matching operations in pure memory.
- **Throughput Capacity:** ~7,500 operations per second under single-threaded limits.
- **Memory Footprint:** 275 Bytes per order allocation with 4 allocations per execution.
