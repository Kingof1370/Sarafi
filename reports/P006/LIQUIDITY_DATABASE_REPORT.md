# LIQUIDITY DATABASE REPORT
Execution Time: 2026-08-01 21:56:59
Git Commit Hash: 3b36905f0d64a562aa7a4706ecde212ad1daf8ff

## 1. Extended Database Tables
The PostgreSQL auto-migrations (ID 18 to 21) are written to database:
- `liquidity_metrics`: saves market spread and imbalance stats.
- `spread_history`: stores computed spreads.
- `depth_history`: saves bid/ask volume snapshots.
- `surveillance_alerts`: saves flagged anomalous behaviors.
