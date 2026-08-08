# Velyxora High Availability Architecture Guide

This document describes how the platform runs multiple stateless and stateful services.

## 1. Stateless API Gateways

The REST and WebSocket API Gateways are completely stateless.
- Session authorization states are cached in Redis and backed by the PostgreSQL `user_sessions` table.
- Clients can route through standard load balancers safely.

## 2. Distributed Leader/Worker Lock Coordination

To prevent duplicate processing of critical tasks (such as withdrawal broadcasting or scheduled compliance audits), the system uses a distributed database-backed lock:
- **Lock Acquisition**: A worker must call `AcquireDistributedLock` with a unique task key and TTL before executing.
- **Failover**: If a worker node crashes, the lock will expire after its TTL, allowing active standby nodes to automatically assume the lease and continue.
- **No Double-Spending**: Ensures two workers never process the same financial action.
