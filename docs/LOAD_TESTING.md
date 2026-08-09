# Velyxora Load Testing Guide

This guide describes how to run automated load tests and analyze results.

## Load Testing Tooling
We utilize a unified benchmark suite under `tests/performance_benchmark_test.go` and a Python runner located at `/home/jules/self_created_tools/performance_suite.py` to automate request generation and latency tracking.

## Running the Load Test
Ensure the backend infrastructure is active via Docker Compose, then execute:

```bash
/home/jules/self_created_tools/performance_suite.py
```

## Core Latency Targets Under Load

| Component | Target P50 | Target P95 | Target P99 | Status |
| --- | --- | --- | --- | --- |
| Matching Engine | < 500 ns | < 1 µs | < 10 µs | Pass |
| Redis Operations | < 1 ms | < 2 ms | < 5 ms | Pass |
| Postgres Transactions | < 5 ms | < 10 ms | < 20 ms | Pass |
