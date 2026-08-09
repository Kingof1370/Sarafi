# Velyxora Performance Regression Framework

## Regression Safeguards
We introduced persistent, automated Go benchmarks and stress-test suites under `tests/performance_benchmark_test.go` and an executable Python performance verifier under `/home/jules/self_created_tools/performance_suite.py`.

## Running Regression Tests
To run the benchmark suites and check for performance regressions under future feature rollouts, execute:

```bash
/home/jules/self_created_tools/performance_suite.py
```

Or run Go benchmarks directly:
```bash
go test ./tests -bench=. -benchmem -run=^$
```
This ensures any memory allocation growth, lock contentions, or latency drift is flagged before committing.
