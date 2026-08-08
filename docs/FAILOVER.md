# Velyxora Failover and Database Redundancy Guidelines

This document details failover actions for database and queue layers.

## 1. PostgreSQL Cluster Failover

- **Primary DB Node**: Handles all read-write operations with row-level locks on available-reserved balance segments.
- **Hot Standbys**: Asynchronous streaming replication nodes.
- **Failover Trigger**: Managed by replication tools (such as Patroni/PgBouncer). Upon primary node failure, a standby is promoted to primary, and the REST API Gateways swap connection targets.

## 2. Redis Replication and Failover

- **AOF (Append Only File)**: Activated with `appendfsync everysec` for maximum data durability.
- **Redis Sentinel**: Tracks health and performs automatic failover to read replicas.
- **Postgres Fallback**: If Redis is completely down, gateways fall back to PostgreSQL `user_sessions` to verify authentication states.
