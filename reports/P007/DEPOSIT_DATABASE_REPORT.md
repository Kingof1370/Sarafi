# Velyxora Exchange — P007 Deposit Engine Database Report

## Metadata
* **Execution Timestamp:** 2026-08-02 15:00:00 UTC
* **Git Commit Hash:** 29153eb

## Schema Design (Migrations 40 - 42)
We expanded the database with the following normalized relational schemas:
* `blockchain_transactions` (tx_hash (PK), network, asset, amount, sender, receiver, block_number, gas_used, timestamp)
* `deposit_confirmations` (deposit_id (PK), confirmations_count, required_confirmations, status, updated_at)
* `deposit_events` (id (PK), event_type, deposit_id, payload, timestamp)

## Relational Constraints & Indexes
* **Foreign Keys:**
  - `deposit_confirmations.deposit_id` -> `deposits.id` (ON DELETE CASCADE)
  - `deposit_events.deposit_id` -> `deposits.id` (ON DELETE CASCADE)
* **Indices:**
  - `idx_blockchain_tx_network_block` on `blockchain_transactions (network, block_number)`
  - `idx_deposit_conf_status` on `deposit_confirmations (status)`
  - `idx_deposit_events_type_dep` on `deposit_events (event_type, deposit_id)`
