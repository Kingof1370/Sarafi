# Velyxora Exchange — P007 Custody Database Report

## Metadata
* **Execution Timestamp:** 2026-08-03 05:00:00 UTC
* **Git Commit Hash:** [PROD_READY_COMMIT]
* **Branch Name:** p007-wallet-custody

## Database Schema (Migrations 59 - 64)
We added custody-specific relational tables to PostgreSQL via the native SQL migration engine inside `packages/database/migrations.go`:
* `custody_vaults` (id, name, type, is_locked, created_at)
* `vault_assets` (vault_id, asset, balance, updated_at)
* `custody_transfers` (id, from_vault_id, to_vault_id, asset, amount, status, required_approvals, current_approvals, timestamp)
* `custody_transfer_approvals` (id, transfer_id, admin_id, created_at)
* `custody_audits` (id, vault_id, transfer_id, asset, action, amount, message, timestamp)
* `custody_emergency_actions` (id, action, operator_id, details, timestamp)

## Relational Constraints & Indexes
* **Foreign Keys:**
  - `vault_assets.vault_id` -> `custody_vaults.id` (ON DELETE CASCADE)
  - `custody_transfers.from_vault_id` -> `custody_vaults.id` (ON DELETE CASCADE)
  - `custody_transfers.to_vault_id` -> `custody_vaults.id` (ON DELETE CASCADE)
  - `custody_transfer_approvals.transfer_id` -> `custody_transfers.id` (ON DELETE CASCADE)
* **Indices:**
  - `idx_custody_trans_status` on `custody_transfers (status)`
  - `idx_custody_app_trans` on `custody_transfer_approvals (transfer_id)`
  - `idx_custody_audits_vault` on `custody_audits (vault_id)`
