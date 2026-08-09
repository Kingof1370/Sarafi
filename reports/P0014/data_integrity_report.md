# P0014 DATA INTEGRITY AFTER FAILURE REPORT

- **Ledger Verification Math:** sum(debits) == sum(credits) is maintained.
- **Pre-trade Hold Reservations:** Locked/Reserved balances are pinned in Postgres using row locks and validated on reboot.
- **Double execution controls:** strict idempotency records mapping and advisory locks completely eliminate double modifications.
- **Transaction Consistency:** Fully validated across full integration test failure inject scenarios.
