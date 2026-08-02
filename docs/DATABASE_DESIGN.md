# Velyxora Exchange — Database Schema & Performance Design

## 1. Relational Layout
All table schemas are normalized and managed by the native Go migration system inside `packages/database/migrations.go`.

---

## 2. Core Wallet Subsystem Tables

### 2.1 wallets
Holds individual wallet profiles with roles and statuses.
* **Fields:** `id` (PK), `user_id` (Index), `type`, `is_locked`, `created_at`, `updated_at`

### 2.2 wallet_balances
Maintains available, locked, reserved, and pending metrics for each asset.
* **Fields:** `wallet_id` (PK, FK), `asset` (PK, Index), `available`, `locked`, `reserved`, `pending`, `total`, `updated_at`

### 2.3 wallet_balance_history
Stores historical ledger snapshots for audit trails.
* **Fields:** `id` (PK), `wallet_id` (FK, Index), `asset` (Index), `available`, `locked`, `reserved`, `pending`, `total`, `timestamp`

### 2.4 wallet_addresses
Deposit addresses mapped deterministically from key derivations.
* **Fields:** `address` (PK), `user_id` (Index), `network` (Index), `public_key`, `derivation_path`, `memo`, `status`, `created_at`

### 2.5 wallet_audits
Chained SHA-256 ledger audit log.
* **Fields:** `id` (PK), `user_id` (Index), `wallet_id` (FK, Index), `asset`, `action`, `amount`, `prev_balance`, `new_balance`, `message`, `prev_hash`, `hash`, `timestamp`

---

## 3. High Availability Indexes & Foreign Keys
Every table features cascading delete foreign keys linking back to `wallets` and `assets_registry` to preserve referential integrity, along with high-selectivity indexes to ensure sub-millisecond query execution times under heavy concurrent load.
