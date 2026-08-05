# Velyxora Exchange — Microservice System Architecture

## 1. System Overview
Velyxora is an autonomous, high-throughput, low-latency, and ultra-secure cryptocurrency exchange built with Go.

```
                  +--------------------------------+
                  |         React Dashboard        |
                  +---------------+----------------+
                                  |
                                  v
                  +---------------+----------------+
                  |          API Gateway           |
                  +---------------+----------------+
                                  |
            +---------------------+---------------------+
            |                                           |
            v                                           v
+-----------+-----------+                   +-----------+-----------+
|    Matching Engine    |                   |      Wallet Service   |
+-----------+-----------+                   +-----------+-----------+
            |                                           |
            +---------------------+---------------------+
                                  |
                                  v
                  +---------------+----------------+
                  |      PostgreSQL / Kafka / Redis |
                  +--------------------------------+
```

## 2. Shared Library Core Architecture
* **`packages/blockchain`:** Unified adapters for BTC, ETH, BSC, Polygon, Solana, Avalanche, Tron, and Litecoin. Handles address allocations, key derivation, and signature verifications.
* **`packages/wallet`:** Manages multi-dimensional Available/Locked/Reserved/Pending/Total balances, daily operational spending limits, and Treasury administrative stage workflows.
* **`packages/custody`:** Orchestrates institutional custody vaults with 4-eyes approvals and global freeze toggles.
* **`packages/treasury`:** Manages corporate pools, allowed transfer matrices, dual approval thresholds, and automated reconciliations.
* **`packages/monitoring`:** Governs real-time health checks, IP and geodistance risk fraud analysis, security alerts, auto-escalations, and service auto-healers.
* **`packages/ha`:** Orchestrates High Availability NodeManager, consensus LeaderElection, heartbeatsfailover, and graceful connection draining.
* **`packages/assets`:** Manages asset configs and deposit/withdrawal/trade limit permissions.
* **`packages/address`:** Handles address registry lookup, allocation, and cryptographic signature verification.
* **`packages/ledger-common`:** Secure chained hash auditing validation.
* **`packages/deposits`:** Enterprise Deposit Engine and Confirmation Tracker, with duplicate block transaction validations, chain reorganization support, and orphan exclusion checks.
