# P0011 High Availability & Node Coordination Report

This report evaluates system behavior under clustered environments.

## 1. Stateless API Gateways
- REST API and WebSocket Gateway pods are stateless.
- Active user JWT sessions are verified against Redis caches and fall back to PostgreSQL `user_sessions` tables.
- Multiple instances can run safely under load-balanced layers.

## 2. Distributed Leader/Worker Lease Coordination
- To prevent split-brain processing of critical routines (such as withdrawal broadcasting or scheduled audits), workers must lease a distributed lock.
- Implementation: Enforced via PostgreSQL table `dr_coordination_locks` using atomic inserts and expirations.
- Expiration: Active locks automatically expire after their specified TTL, allowing secondary standby nodes to safely acquire the lease and resume workflows without double processing.
