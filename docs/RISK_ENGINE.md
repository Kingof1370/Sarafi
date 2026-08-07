# VELYXORA EXCHANGE - RISK ENGINE
Details pre-trade risk and abuse validation systems.

## Persistent Validation
Risk validations do not rely exclusively on volatile memory:
1. **Available Balance Check:** Authoritative PostgreSQL locking is carried out using `SELECT FOR UPDATE` to query available balances.
2. **Atomic Reservation:** Moves quote/base funds from the `available` to the `reserved` database column prior to admitting orders into the matcher core.
3. **Abuse & Spam Detection:** Implements rapid-fire checks limiting users to 10 placements/sec.
