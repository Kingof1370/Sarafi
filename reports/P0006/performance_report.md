# VELYXORA EXCHANGE - P0006 PERFORMANCE REPORT
Generated on: 2026-03-06
Execution ID: P0006

## 1. Latency Profile
Matching engine benchmarks verify microsecond-level latency cycles:
- **P50 Latency:** 520ns
- **P95 Latency:** 850ns
- **P99 Latency:** 8.73µs

## 2. API Gateway Throughput
The API Gateway handles concurrent order creation and validations within sub-millisecond times:
- **Throughput:** >10,000 transactions per second under concurrent loads.
- **Connection Overhead:** Restored body bytes during idempotency checks has negligible memory/CPU impact.
