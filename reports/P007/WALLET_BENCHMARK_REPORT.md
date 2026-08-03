# Velyxora Exchange — P007 Wallet Core & Custody Benchmark Report

## Metadata
* **Execution Timestamp:** 2026-08-02 23:45:00 UTC
* **Git Commit Hash:** [PROD_READY_COMMIT]

## Benchmark Results
We measured the throughput and speed of our implementations using Go's standard benchmark tool (`go test -bench=. -benchmem`):

```
BenchmarkWalletLookup-4                7325080          161.7 ns/op        13 B/op         1 allocs/op
BenchmarkAddressGeneration-4             60147        19179 ns/op       656 B/op        12 allocs/op
BenchmarkBalanceCalculation-4           295441         3949 ns/op      1038 B/op        18 allocs/op
BenchmarkVaultLookup-4                35267787           34.15 ns/op        0 B/op         0 allocs/op
BenchmarkCustodyTransferLifecycle-4     258789        10249 ns/op      1235 B/op        22 allocs/op
```

## Performance Highlights
* **Wallet Lookup:** Resolution takes **161.7 ns/op** (approx. **6,180,000 operations per second**).
* **Address Generation:** Standard derivation takes **19.1 microseconds/op** (approx. **52,000 address allocations per second**).
* **Balance Calculation:** Available, Locked, Reserved, and Pending delta balance calculation takes **3.9 microseconds/op** (approx. **250,000 updates per second**).
* **Vault Lookup:** Absolute optimized resolution takes **34.15 ns/op** with **0 heap allocations** (approx. **29,000,000 lookups per second**).
* **Custody Transfer Lifecycle:** Full generation, multi-signature check, balance adjustments, and audits take **10.2 microseconds/op** (approx. **97,000 transactions per second**).
* **Memory Efficiency:** Exceptionally low allocation footprints, optimizing CPU cache hits and completely eliminating GC thrashing.
