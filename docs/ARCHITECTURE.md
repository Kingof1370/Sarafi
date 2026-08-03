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
* **`packages/custody`:** Orchestrates institutional digital asset custody segments (Hot, Warm, Cold, Deep Cold, Treasury, Reserve, Recovery vaults) with 4-eyes multi-signature approval pipelines and secure system-wide emergency freeze flags.
* **`packages/assets`:** Manages asset configs and deposit/withdrawal/trade limit permissions.
* **`packages/address`:** Handles address registry lookup, allocation, and cryptographic signature verification.
* **`packages/ledger-common`:** Secure chained hash auditing validation.
* **`packages/deposits`:** Enterprise Deposit Engine and Confirmation Tracker, with duplicate block transaction validations, chain reorganization support, and orphan exclusion checks.
