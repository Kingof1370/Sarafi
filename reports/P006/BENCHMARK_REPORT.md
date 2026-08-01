# VELYXORA EXCHANGE - P006 PART 1 BENCHMARK REPORT
Execution Time: 2026-08-01 13:12:20
Git Commit Hash: 3b36905f0d64a562aa7a4706ecde212ad1daf8ff

## 1. Benchmark Command
`go test -bench=BenchmarkMatchingEnginePipeline -benchmem velyxora/apps/matching-engine/engine`

## 2. Benchmark Output
```text
goos: linux
goarch: amd64
pkg: velyxora/apps/matching-engine/engine
cpu: Intel(R) Xeon(R) Processor @ 2.30GHz
BenchmarkMatchingEnginePipeline-4   	 2021680	       515.6 ns/op	     297 B/op	       2 allocs/op
PASS
ok  	velyxora/apps/matching-engine/engine	1.674s

```
