# Velyxora Exchange — P007 Withdrawal Engine Database Report

## Metadata
* **Execution Timestamp:** 2026-08-02 18:00:00 UTC
* **Git Commit Hash:** 29153eb

## Schema Design (Migrations 43 - 48)
We expanded the database with the following normalized relational schemas:
* `withdrawal_address_book` (id, user_id, address, network, label, is_whitelisted, trust_score, verified_at, UNIQUE (user_id, address))
* `withdrawal_queue` (id, user_id, asset, amount, fee, address, status, risk_score, retries, created_at, updated_at)
* `withdrawal_approvals` (id, withdrawal_id, admin_id, created_at)
* `withdrawal_audits` (id, user_id, withdrawal_id, asset, amount, prev_status, new_status, message, timestamp)
* `withdrawal_risk_events` (id, user_id, withdrawal_id, details, timestamp)
* `withdrawal_broadcast_history` (tx_hash, withdrawal_id, payload_hex, timestamp)

## Relational Constraints & Indexes
* **Foreign Keys:** All dependent tables feature foreign keys linking to `withdrawal_queue.id` with `ON DELETE CASCADE` cascading deletes.
* **Indices:**
  - `idx_withdraw_addr_user_addr` on `withdrawal_address_book (user_id, address)`
  - `idx_withdrawal_q_status` on `withdrawal_queue (status)`
  - `idx_withdrawal_app_id` on `withdrawal_approvals (withdrawal_id)`
  - `idx_withdrawal_aud_id` on `withdrawal_audits (withdrawal_id)`
  - `idx_withdrawal_broad_id` on `withdrawal_broadcast_history (withdrawal_id)`
