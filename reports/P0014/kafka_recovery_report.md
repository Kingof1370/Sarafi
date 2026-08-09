# P0014 KAFKA RECOVERY REPORT

- **Topic Retention Policies:** 7 days retention for critical financial streams.
- **Consumer Group Offset Management:** Offsets are committed programmatically to broker coordinates and mirrored in Postgres.
- **Event Replay Guarantees:** Ephemeral states can be reconstructed safely without duplication of ledger movements.
- **Topics Audited:** `velyxora-orders`, `velyxora-trades`, `velyxora-depth`, `velyxora-balances`, `velyxora-withdrawals`, `velyxora-deposits`.
