# Velyxora Load Test Report

## Objective
Measure latency and error rate under normal to high load levels.

## Configuration
- **Load level:** Constant 10,000 requests over 10 seconds.
- **Concurrent clients:** 10

## Results

| Target Endpoint | Requests/sec | P50 Latency | P95 Latency | P99 Latency | Error Rate |
| --- | --- | --- | --- | --- | --- |
| `/api/v1/market/ticker` | 1,000 rps | 0.32 ms | 0.45 ms | 0.62 ms | 0.00% |
| `/api/v1/wallet/balances` | 1,000 rps | 1.10 ms | 1.45 ms | 1.88 ms | 0.00% |
| Matching matches | 100,000 mps | 396 ns | 564 ns | 6.602 µs | 0.00% |
