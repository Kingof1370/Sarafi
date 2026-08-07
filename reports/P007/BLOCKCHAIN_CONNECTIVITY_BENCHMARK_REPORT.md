# Velyxora Exchange — P007 Blockchain Connectivity Benchmark Report

## Metadata
* **Execution Timestamp:** 2026-08-02 20:00:00 UTC
* **Git Commit Hash:** 29153eb

## Benchmark Results
We measured the throughput and speed of our implementations using Go's standard benchmark tool (`go test -bench=. -benchmem`):

```
BenchmarkOptimalNodeSelection-4     	18749541	        65.12 ns/op	       0 B/op	       0 allocs/op
BenchmarkBlockTracker-4             	 1000000	      1171 ns/op	     159 B/op	       4 allocs/op
```

## Performance Highlights
* **Optimal Node Selection:** Selection takes **65.12 ns/op** (approx. **15.3 million selections per second**).
* **Block tracker parsing:** Headers parsing and caching takes **1.17 microseconds/op** (approx. **850,000 block tracks per second**).
* **Memory Efficiency:** Zero allocations on node selection, keeping CPU cache performance optimal.
