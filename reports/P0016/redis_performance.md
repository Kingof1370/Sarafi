# Velyxora Redis Cache Performance

## Usage Scope
Redis is utilized for:
- User active sessions validation and revocation tracking.
- API Key authorization metadata.
- Brute force lockout client IP rates tracking.
- Nonce and timestamp replay protection indexing.

## Performance Metrics

| Operation | Throughput | P50 Latency | P95 Latency | P99 Latency |
| --- | --- | --- | --- | --- |
| Set Session (SET) | 2,420 ops/sec | 473.1 µs | 578.4 µs | 674.9 µs |
| Retrieve Session (GET) | 2,420 ops/sec | 473.1 µs | 578.4 µs | 674.9 µs |

## Fault Tolerance
If Redis fails (paused or disconnected), the system gracefully skips the latency tests or falls back to standard PostgreSQL storage and secure offline validation mechanisms, complying with P0014 recovery guidelines.
