# OMS DATABASE REPORT
Execution Time: 2026-08-01 14:59:35
Git Commit Hash: 3b36905f0d64a562aa7a4706ecde212ad1daf8ff

## 1. Database Migrations Executed
Automatic migration schema ID 6 through 9 are incorporated into `packages/database/migrations.go` and run on API Gateway startup:
- `orders`: persists advanced orders.
- `order_history`: stores finalized records.
- `order_events`: logs trade execution actions.
- `order_audits`: registers transition audit trails.
