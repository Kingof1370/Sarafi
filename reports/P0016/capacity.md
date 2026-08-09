# Velyxora Capacity Planning Model

## Measured Capacity Limits

Based on real baseline and optimized benchmark measurements, the limits for each component on standard devbox resources are:

| Service / Component | Maximum Capacity Metric | Primary Bottleneck | Scaling Boundary |
| --- | --- | --- | --- |
| API Gateway | 3,200 requests/sec | CPU (serialization) | Scale horizontally (stateless) |
| Matching Engine | 783,008 orders/sec | Single-thread CPU core lock | Scale using multi-engine sharding |
| Wallet Service | 877 ledger tx/sec | DB write latency (I/O) | Scale using pgxpool tuning |
| Redis Session Cache | 25,000 requests/sec | Network network I/O | Scale via Redis Cluster / replica |
| WebSocket Broadcast | 15,000 broadcasts/sec | Memory (per slow-client queue) | Scale via horizontal socket relays |

## Bottlenecks & Critical Limits
- PostgreSQL transaction write locks limit the database balance settlement speed to ~1,000 tx/sec under strict serial write constraints.
- Matching engine performance is capped by the CPU speed of a single core because matching logic runs single-threaded to guarantee deterministic Price-Time matching ordering.
