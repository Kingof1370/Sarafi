# Redis Distributed Rate Limiting & Lockout Specification

This document details the Redis sorted-set sliding window rate limiter and progressive brute-force lockout layer.

## 1. Sliding Window Rate Limiting (Redis ZSET)
To prevent bypass across multiple API Gateway instances, Velyxora implements a Redis-backed sliding window rate limiter.

### Redis Command Flow:
1. `ZRemRangeByScore key 0 <cutoff>`: Cleans up old request timestamps older than the sliding window.
2. `ZCard key`: Returns the current active count of requests in the window.
3. `ZAdd key <now> <unique_id>`: Inserts the current request timestamp.
4. `Expire key <window_seconds>`: Refreshes key TTL.

All commands run inside a single atomic Redis Transaction Pipeline.

### Configuration Matrix:
- **Authentication Routes (`/api/v1/auth/*`)**: Max **10 requests per minute** per client IP.
- **Programmatic API Keys (`X-API-KEY`)**: Max **120 requests per minute** per key.
- **Active Session Users (JWT)**: Max **60 requests per minute** per user ID.
- **Public Generic Endpoints**: Max **30 requests per minute** per client IP.

---

## 2. Progressive Brute-Force Lockout
- **Threshold**: **5 consecutive authentication/MFA failures**.
- **Mitigation**: Applies a temporary **15-minute lockout** constraint.
- **Layered Scope**: Binds to **both** the targeted identity (corporate email) AND the requesting IP address. Changing IP or email alone is insufficient to bypass the lockout mechanism.
- **Audit Logging**: Successful authentications reset the failure counters. lockout triggers are logged as high-severity events in PostgreSQL.
