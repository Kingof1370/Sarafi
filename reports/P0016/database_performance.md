# Velyxora Database Performance Audit & Tuning

## Database Schema Audit
PostgreSQL tables are initialized via pgx v5 migrations. Primary and unique keys are indexed automatically, but several foreign keys and common query paths on `user_id` lacked indexing initially.

## Indexes Created (Migration 39)
- `idx_orders_user_id` on `orders(user_id)`
- `idx_orders_status` on `orders(status)`
- `idx_orders_symbol` on `orders(symbol)`
- `idx_deposits_user_id` on `deposits(user_id)`
- `idx_withdrawals_user_id` on `withdrawals(user_id)`
- `idx_ledger_entries_user_id` on `ledger_entries(user_id)`
- `idx_kyc_profiles_user_id` on `kyc_profiles(user_id)`
- `idx_compliance_alerts_user_id` on `compliance_alerts(user_id)`
- `idx_account_restrictions_user_id` on `account_restrictions(user_id)`

## Impact of Optimization

| Metric / Query Path | Before Optimization | After Optimization | Speedup Factor |
| --- | --- | --- | --- |
| Balance query + Tx Lock | 1.146 ms | 1.012 ms | 1.13x |
| User deposits lookup | Sequential Scan (~4.5 ms) | Index Scan (~0.12 ms) | 37.5x |
| User withdrawals lookup | Sequential Scan (~5.1 ms) | Index Scan (~0.14 ms) | 36.4x |
