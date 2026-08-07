# Velyxora Exchange — P007 Final Database Report

## Metadata
* **Execution Timestamp:** 2026-08-03 05:00:00 UTC
* **Git Commit Hash:** [PROD_READY_COMMIT]
* **Branch Name:** p007-wallet-custody-complete

## Complete Database Schema (Migrations 59 - 77)
We added custody, treasury, automatic reconciliation, monitoring, and fraud-specific relational tables to PostgreSQL via the native SQL migration engine inside `packages/database/migrations.go`:
* `custody_vaults` (id, name, type, is_locked, created_at)
* `vault_assets` (vault_id, asset, balance, updated_at)
* `custody_transfers` (id, from_vault_id, to_vault_id, asset, amount, status, required_approvals, current_approvals, timestamp)
* `custody_transfer_approvals` (id, transfer_id, admin_id, created_at)
* `custody_audits` (id, vault_id, transfer_id, asset, action, amount, message, timestamp)
* `custody_emergency_actions` (id, action, operator_id, details, timestamp)
* `treasury_pools` (id, name, type, created_at)
* `pool_assets` (pool_id, asset, balance, updated_at)
* `treasury_transfers` (id, from_pool, to_pool, asset, amount, status, required_approvals, current_approvals, risk_score, timestamp)
* `treasury_transfer_approvals` (id, transfer_id, admin_id, created_at)
* `liquidity_records` (id, asset, depth_bid, depth_ask, spread, timestamp)
* `reserve_accounts` (id, name, asset, backing_ratio, updated_at)
* `insurance_fund` (id, asset, balance, cap_limit, updated_at)
* `reconciliation_results` (id, blockchain_verified, database_verified, ledger_verified, wallet_verified, transfers_verified, is_consistent, details, timestamp)
* `incidents` (id, title, severity, status, details, timestamp)
* `alerts` (id, source, message, severity, timestamp)
* `fraud_events` (id, user_id, action_type, risk_score, details, timestamp)
* `recovery_events` (id, component, details, timestamp)
* `monitoring_metrics_records` (id, metric_name, metric_value, timestamp)
* `health_status_records` (component, status, updated_at)

## Relational Constraints & Indexes
* **Foreign Keys:**
  - `vault_assets.vault_id` -> `custody_vaults.id` (ON DELETE CASCADE)
  - `custody_transfers.from_vault_id` -> `custody_vaults.id` (ON DELETE CASCADE)
  - `custody_transfers.to_vault_id` -> `custody_vaults.id` (ON DELETE CASCADE)
  - `custody_transfer_approvals.transfer_id` -> `custody_transfers.id` (ON DELETE CASCADE)
  - `pool_assets.pool_id` -> `treasury_pools.id` (ON DELETE CASCADE)
  - `treasury_transfer_approvals.transfer_id` -> `treasury_transfers.id` (ON DELETE CASCADE)
* **Indices:**
  - `idx_custody_trans_status` on `custody_transfers (status)`
  - `idx_custody_app_trans` on `custody_transfer_approvals (transfer_id)`
  - `idx_custody_audits_vault` on `custody_audits (vault_id)`
  - `idx_treasury_trans_status` on `treasury_transfers (status)`
  - `idx_treasury_app_trans` on `treasury_transfer_approvals (transfer_id)`
  - `idx_liquidity_rec_asset` on `liquidity_records (asset)`
  - `idx_reconciliation_res_consistent` on `reconciliation_results (is_consistent)`
  - `idx_incidents_status` on `incidents (status)`
  - `idx_alerts_severity` on `alerts (severity)`
  - `idx_fraud_user` on `fraud_events (user_id)`
