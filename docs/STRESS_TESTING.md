# Velyxora Stress Testing Manual

This manual details stress testing limits, saturation benchmarks, and chaos failure behaviors.

## Executing Stress Tests
To stress the system to its limit under multiple concurrent client threads, execute:

```bash
go test ./tests -run TestMatchingEngineStressAndSaturation -v
```

This test spawns 20 parallel clients submitting 40,000 orders to find the safe processing limit.

## Saturation Limit Metrics

| Clients | Total Orders | Elapsed Time | Saturation Speed | CPU Limit |
| --- | --- | --- | --- | --- |
| 20 | 40,000 | 51.08 ms | 783,008 orders/sec | ~82% |

## Chaos Degradation & Graceful Recovery
Our chaos simulator pauses Redis and Postgres to evaluate connection loss:
- **Redis Outage:** Requests bypass cache and complete safely via database lookups.
- **Postgres Outage:** Queries timeout gracefully, API responses return 503 instead of panicking, and auto-reconnection occurs on recovery.
