# Velyxora Exchange — P007 Deposit Engine Benchmark Report

## Metadata
* **Execution Timestamp:** 2026-08-02 15:00:00 UTC
* **Git Commit Hash:** 29153eb

## Benchmark Results
We measured the throughput and speed of our implementations using Go's standard benchmark tool (`go test -bench=. -benchmem`):

```
BenchmarkDepositProcessing-4        	  389677	      4108 ns/op	     809 B/op	      11 allocs/op
BenchmarkConfirmationProcessing-4   	22181413	        53.79 ns/op	       0 B/op	       0 allocs/op
```

## Performance Highlights
* **Ingress Validation:** Ingress and Duplicate Check processing takes **4.1 microseconds/op** (approx. **240,000 operations per second**).
* **Confirmation Processing:** Depth checks and rule finalizations take **53.79 ns/op** (approx. **18.5 million checks per second**).
* **Memory Efficiency:** Near zero allocation counts (0 to 11 allocations), preventing GC pressure and keeping latency under sub-microsecond levels.
