# Velyxora Exchange — P007 Wallet Core Database Report

## Metadata
* **Execution Timestamp:** 2026-08-02 12:00:00 UTC
* **Git Commit Hash:** 29153eb

## Schema Design (Migrations 30 - 39)
We expanded the native migration engine with normalized relational schemas to support:
* `blockchain_networks` (id, name, derivation_path, is_active, created_at)
* `wallets` (id, user_id, type, is_locked, created_at, updated_at)
* `assets_registry` (symbol, name, type, precision, base_network, is_active, created_at)
* `assets_metadata` (symbol, contract_address, logo_url, description, website, explorer_url, total_supply, circulating_price, updated_at)
* `assets_permissions` (symbol, can_deposit, can_withdraw, can_trade, min_deposit_amount, min_withdraw_amount, max_daily_withdrawal, withdrawal_fee, updated_at)
* `wallet_balances` (wallet_id, asset, available, locked, reserved, pending, total, updated_at)
* `wallet_balance_history` (id, wallet_id, asset, available, locked, reserved, pending, total, timestamp)
* `wallet_addresses` (address, user_id, network, public_key, derivation_path, memo, status, created_at)
* `wallet_events` (id, event_type, wallet_id, payload, timestamp)
* `wallet_audits` (id, user_id, wallet_id, asset, action, amount, prev_balance, new_balance, message, prev_hash, hash, timestamp)

## Relational Constraints & Indexes
* **Foreign Keys:**
  - `assets_metadata.symbol` -> `assets_registry.symbol` (ON DELETE CASCADE)
  - `assets_permissions.symbol` -> `assets_registry.symbol` (ON DELETE CASCADE)
  - `wallet_balances.wallet_id` -> `wallets.id` (ON DELETE CASCADE)
  - `wallet_balance_history.wallet_id` -> `wallets.id` (ON DELETE CASCADE)
* **Indices:**
  - `idx_blockchain_networks_active` on `blockchain_networks (is_active)`
  - `idx_wallets_user_id` on `wallets (user_id)`
  - `idx_wallets_type` on `wallets (type)`
  - `idx_wallet_balances_asset` on `wallet_balances (asset)`
  - `idx_wallet_addresses_user_network` on `wallet_addresses (user_id, network)`
  - `idx_wallet_events_type_wallet` on `wallet_events (event_type, wallet_id)`
  - `idx_wallet_audits_user_asset` on `wallet_audits (user_id, asset)`
  - `idx_wallet_audits_wallet` on `wallet_audits (wallet_id)`
