# Velyxora Exchange — Clustering and Scale-Out Architecture

## 1. Overview
This document specifies the clustering layout and state synchronization mechanisms enabling horizontal scaling across Velyxora's distributed nodes.

---

## 2. Shared Data Clustering Abstractions

### 2.1 Database Scaling (Read Replicas)
To achieve extreme read throughput:
* Database connection pools separate primary writers from read replicas.
* The matching engine utilizes `GetWritePool()` for order execution and transactions.
* Public statistics and history APIs route through `GetReadPool()` to leverage postgres scale-out replicas.

### 2.2 Redis Sentinel Failover
State caches and user session registries are managed via clustering:
* Integrates Redis Cluster and Sentinel topologies.
* Automatic Sentinel monitoring redirects read/write traffic during node failures.

### 2.3 Kafka Replication
All system events leverage high replication parameters:
* Partitions are replicated across multiple Kafka brokers.
* Consumer groups handle automated partition rebalancing during node scaling or failures.
