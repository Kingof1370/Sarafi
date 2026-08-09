# P0014 WALLET RECOVERY REPORT

- **Pending/Confirming Deposits Recovery:** Scan resumes safely on wallet node reboot utilizing blockchain scanner state tracked in Postgres.
- **Interrupted Withdrawals:** Transition states (`BROADCASTING`/`CONFIRMING`) resume execution processing idempotently.
- **Single Writer Lock:** Transaction-Scoped PostgreSQL Advisory locks prevent duplicate withdrawal broadcasts.
- **Double-Entry Balance Safeguards:** Atomic SELECT FOR UPDATE locks on balances enforce pre-trade reservations.
