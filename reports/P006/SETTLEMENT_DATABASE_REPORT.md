# SETTLEMENT DATABASE REPORT
Execution Time: 2026-08-01 20:23:37
Git Commit Hash: 3b36905f0d64a562aa7a4706ecde212ad1daf8ff

## 1. Extended Database Tables
The PostgreSQL auto-migrations (ID 14 to 17) are written to database:
- `settlements_queue`: pending clearing jobs.
- `settlements_history`: completed clearing history.
- `fee_ledgers`: platform transaction fee ledgers.
- `accounting_reconciliations`: trial balance auditing.
