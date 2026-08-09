# Velyxora Capacity Planning & Bottleneck Modeling

This document maps out maximum system capacities and scaling boundaries.

## Resource & Scaling Limits

| Component | Max Measured Capacity | Hard Limit Bottleneck | Remedy |
| --- | --- | --- | --- |
| Matching Engine | 1,392,757 matches/sec | Single-core CPU speed | Symbol sharding (horizontal sharding) |
| API Gateway | 3,200 requests/sec | JSON serialization CPU | Horizontal scaling behind load balancer |
| Database | 769 tx/sec | Disk write I/O / locks | pgxpool config tuning / Read replicas |
| Redis Cache | 25,000 ops/sec | Network network I/O | Redis Cluster |
| WebSocket Gateway | 15,000 broadcasts/sec | Memory per slow client | Offload to stateless socket relays |

## Sharding Guidelines
For trading scaling, symbols (e.g. `BTC-USDT` and `ETH-USDT`) should be sharded across independent matching engine pods, ensuring each matching engine is a single-writer for its assigned symbol book.
