# Velyxora Performance Baseline Report

## Test Environment Details
- **OS:** Linux (AMD64 architecture)
- **CPU:** Intel(R) Xeon(R) Processor @ 2.30GHz
- **DB:** PostgreSQL 16 on Alpine container
- **Cache:** Redis 7 on Alpine container
- **Queue:** Kafka on cp-kafka:7.5.0 container

## Initial Baseline Latency & Throughput (Before Optimizations)

| Metric / Path | Average Latency | P50 (Median) | P95 | P99 | Throughput (Est) |
| --- | --- | --- | --- | --- | --- |
| Matching Engine | 818 ns | 479 ns | 690 ns | 6.472 µs | 1,222,493 matches/sec |
| Redis SET + GET | 467 µs | 461 µs | 564 µs | 633 µs | 2,141 ops/sec |
| PostgreSQL Tx | 1.146 ms | 1.129 ms | 1.351 ms | 1.473 ms | 871 tx/sec |
| Concurrent Stress | 1.37 ms | 1.25 ms | 1.45 ms | 1.72 ms | 725,291 orders/sec |
