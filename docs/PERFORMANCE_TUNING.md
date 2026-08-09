# Velyxora Performance Tuning Guidelines

This document provides system-level tuning configurations for production deployment.

## PostgreSQL Configuration
To maximize transaction throughput under high write contention:
- **`max_connections`**: set to 100-250 (keep `pgxpool.MaxConns` bounded to 25 to avoid connection starvation).
- **`shared_buffers`**: set to 25% of total system memory.
- **`synchronous_commit`**: set to `on` for core ledger entries to maintain absolute safety.

## Redis Configuration
- **`maxmemory-policy`**: set to `volatile-lru` to allow safe eviction of expired sessions without breaking nonce limits.

## Matching Engine tuning
- Trade allocations must be kept minimal by pre-allocating slices.
- Limit orders uses internal monotonic sequence counter rather than slow Unix system clocks for matching trade IDs.
