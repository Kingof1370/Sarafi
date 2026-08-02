# Velyxora Exchange — P007 Withdrawal Engine Benchmark Report

## Metadata
* **Execution Timestamp:** 2026-08-02 18:00:00 UTC
* **Git Commit Hash:** 29153eb

## Benchmark Results
We measured the throughput and speed of our implementations using Go's standard benchmark tool (`go test -bench=. -benchmem`):

```
BenchmarkWithdrawalValidation-4     	  193735	     10022 ns/op	    1359 B/op	      28 allocs/op
BenchmarkApprovalProcessing-4       	 5422509	       217.6 ns/op	      24 B/op	       1 allocs/op
```

## Performance Highlights
* **Withdrawal Validation:** Cooldowns, whitelists, and velocity checks take **10.0 microseconds/op** (approx. **100,000 checks per second**).
* **Approval Processing:** Multi-signature approval registrations take **217.6 ns/op** (approx. **4,600,000 approvals per second**).
* **Memory Efficiency:** Very low allocation bounds (1 to 28 allocations per operation), keeping CPU cache performance optimal.
