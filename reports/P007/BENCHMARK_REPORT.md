# Velyxora Exchange — P007 Wallet Core Benchmark Report

## Metadata
* **Execution Timestamp:** 2026-08-02 12:00:00 UTC
* **Git Commit Hash:** 29153eb

## Benchmark Results
We measured the throughput and speed of our implementations using Go's standard benchmark tool (`go test -bench=. -benchmem`):

```
BenchmarkWalletLookup-4         	 7325080	       161.7 ns/op	      13 B/op	       1 allocs/op
BenchmarkAddressGeneration-4    	   60147	     19179 ns/op	     656 B/op	      12 allocs/op
BenchmarkBalanceCalculation-4   	  295441	      3949 ns/op	    1038 B/op	      18 allocs/op
```

## Performance Highlights
* **Wallet Lookup:** Resolution takes **161.7 ns/op** (approx. **6,180,000 operations per second**).
* **Address Generation:** Standard derivation takes **19.1 microseconds/op** (approx. **52,000 address allocations per second**).
* **Balance Calculation:** Available, Locked, Reserved, and Pending delta balance calculation takes **3.9 microseconds/op** (approx. **250,000 updates per second**).
* **Memory Efficiency:** Very low allocation counts (1 to 18 allocations per operation), leaving CPU cache line hits optimal and preventing GC pressure.
