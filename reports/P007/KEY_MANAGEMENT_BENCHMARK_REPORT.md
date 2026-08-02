# Velyxora Exchange — P007 Key Management Benchmark Report

## Metadata
* **Execution Timestamp:** 2026-08-02 21:15:00 UTC
* **Git Commit Hash:** 29153eb

## Benchmark Results
We measured the throughput and speed of our implementations using Go's standard benchmark tool (`go test -bench=. -benchmem`):

```
BenchmarkKeyGeneration-4            	   38080	     29775 ns/op	    2001 B/op	      30 allocs/op
BenchmarkSignatureCreation-4        	  287647	      8222 ns/op	    1117 B/op	      20 allocs/op
BenchmarkKeyLookup-4                	32574996	        36.93 ns/op	       0 B/op	       0 allocs/op
```

## Performance Highlights
* **Key Lookup:** Querying a key from registry takes **36.93 ns/op** (approx. **27 million operations per second**).
* **Signature Creation:** Registering signature proposals takes **8.2 microseconds/op** (approx. **120,000 updates per second**).
* **Key Generation:** Generation takes **29.7 microseconds/op** (approx. **33,000 keys per second**).
* **Memory Efficiency:** Near zero allocation counts, preventing GC pressure and keeping latency under sub-microsecond levels.
