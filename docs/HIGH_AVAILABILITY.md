# VELYXORA EXCHANGE - HIGH AVAILABILITY & FAILOVER POLICY

## 1. ARCHITECTURE DESIGNS
The Velyxora platform enforces High Availability across all deployment modules:

- **API Gateway (Stateless):** Implements round-robin DNS routing across multiple stateless containers behind Nginx or AWS ALBs.
- **Redis Cache:** Deployed as highly resilient Multi-AZ Redis clusters with automated failover replication.
- **Kafka Cluster:** Configured with a minimum of 3 brokers and a partition replication factor of 2.
- **Matching Engine & Wallet Worker (Stateful):** Mutually exclusive single-writer models governed by transactional PostgreSQL advisory locks. Active standby instances spin up and wait to acquire locks if primary processes terminate unexpectedly.

## 2. FAILOVER EXECUTION RUNS

| Failure Scenario | Recovery Trigger | Action Steps | Expected Downtime (RTO) |
| :--- | :--- | :--- | :--- |
| **API Gateway Instance Failure** | Kubernetes Liveness Probe | Automated container restart & Traffic redirection | &lt; 2 seconds |
| **Primary Postgres Database Crash** | AWS RDS Failover / Patroni | Promotes read-replica to primary writer | &lt; 10 seconds |
| **Redis Cache Cluster offline** | Auto-reconstruction on reload | Rebuilds revoked sessions cache from Postgres | &lt; 60 seconds |
| **Primary Matching Engine Crash** | pg advisory lock release | Standby instance acquires advisory lock and replays disk journals | &lt; 1 second |
