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

## 3. Deposit Engine Subsystem Tables

### 3.1 blockchain_transactions
Parsed block transaction profiles.
* **Fields:** `tx_hash` (PK), `network`, `asset`, `amount`, `sender`, `receiver`, `block_number`, `gas_used`, `timestamp`

### 3.2 deposit_confirmations
Tracks real-time confirmation block depths.
* **Fields:** `deposit_id` (PK), `confirmations_count`, `required_confirmations`, `status`, `updated_at`

### 3.3 deposit_events
Records audit events from deposit handlers.
* **Fields:** `id` (PK), `event_type`, `deposit_id` (Index), `payload`, `timestamp`

---

## 4. Digital Asset Custody Subsystem Tables (Migrations 59 - 64)

### 4.1 custody_vaults
Stores institutional custody vaults configurations.
* **Fields:** `id` (PK), `name`, `type`, `is_locked`, `created_at`

### 4.2 vault_assets
Maintains total asset balances held within each custody vault segment.
* **Fields:** `vault_id` (PK, FK), `asset` (PK), `balance`, `updated_at`

### 4.3 custody_transfers
Tracks internal asset transfers requested between vaults.
* **Fields:** `id` (PK), `from_vault_id` (FK), `to_vault_id` (FK), `asset`, `amount`, `status` (Index), `required_approvals`, `current_approvals`, `timestamp`

### 4.4 custody_transfer_approvals
Logs multi-party administrative 4-eyes signatures.
* **Fields:** `id` (PK), `transfer_id` (FK, Index), `admin_id`, `created_at`

### 4.5 custody_audits
Stores detailed custody audit and operational tracking events.
* **Fields:** `id` (PK), `vault_id` (Index), `transfer_id`, `asset`, `action`, `amount`, `message`, `timestamp`

### 4.6 custody_emergency_actions
Logs triggered platform halts and operator locking details.
* **Fields:** `id` (PK), `action`, `operator_id`, `details`, `timestamp`

---

## 5. Treasury & Reconciliation Tables (Migrations 65 - 71)

### 5.1 treasury_pools
Maintains configurations for exchange-owned pools.
* **Fields:** `id` (PK), `name`, `type`, `created_at`

### 5.2 pool_assets
Stores asset capital holdings allocated within each pool.
* **Fields:** `pool_id` (PK, FK), `asset` (PK), `balance`, `updated_at`

### 5.3 treasury_transfers
Logs requested, approved, and completed pool capital movements.
* **Fields:** `id` (PK), `from_pool`, `to_pool`, `asset`, `amount`, `status` (Index), `required_approvals`, `current_approvals`, `risk_score`, `timestamp`

### 5.4 treasury_transfer_approvals
Logs administrative approvals.
* **Fields:** `id` (PK), `transfer_id` (FK, Index), `admin_id`, `created_at`

### 5.5 liquidity_records
Saves snapshot metrics of order book bids/asks and spreads.
* **Fields:** `id` (PK), `asset` (Index), `depth_bid`, `depth_ask`, `spread`, `timestamp`

### 5.6 reserve_accounts
Logs capital backing details for exchange backing accounts.
* **Fields:** `id` (PK), `name`, `asset`, `backing_ratio`, `updated_at`

### 5.7 insurance_fund
Tracks margins and caps of insolvency buffers.
* **Fields:** `id` (PK), `asset`, `balance`, `cap_limit`, `updated_at`

### 5.8 reconciliation_results
Saves outputs from automatic five-layered reconciliation runs.
* **Fields:** `id` (PK), `blockchain_verified`, `database_verified`, `ledger_verified`, `wallet_verified`, `transfers_verified`, `is_consistent` (Index), `details`, `timestamp`

---

## 6. Enterprise Monitoring & Incident Response Tables (Migrations 72 - 77)

### 6.1 incidents
Logs system incident creations and severe escalations.
* **Fields:** `id` (PK), `title`, `severity`, `status` (Index), `details`, `timestamp`

### 6.2 alerts
Records security and environmental warnings.
* **Fields:** `id` (PK), `source`, `message`, `severity` (Index), `timestamp`

### 6.3 fraud_events
Logs flagged anomalous deposits or withdrawals.
* **Fields:** `id` (PK), `user_id` (Index), `action_type`, `risk_score`, `details`, `timestamp`

### 6.4 recovery_events
Logs automated service recoveries and healing sequences.
* **Fields:** `id` (PK), `component`, `details`, `timestamp`

### 6.5 monitoring_metrics_records
Stores historical component operational values.
* **Fields:** `id` (PK), `metric_name`, `metric_value`, `timestamp`

### 6.6 health_status_records
Logs current operational statuses of components.
* **Fields:** `component` (PK), `status`, `updated_at`

---

## 7. High Availability Indexes & Foreign Keys
Every table features cascading delete foreign keys linking back to parent registries to preserve referential integrity, along with high-selectivity indexes to ensure sub-millisecond query execution times under heavy concurrent load.
