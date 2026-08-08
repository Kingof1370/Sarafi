# VELYXORA EXCHANGE - MARKET DATA RECOVERY & FAILOVER
Details event-recovery and stream failover.

## Recovery Procedures
- Historical candles and trades are cached in memory/PostgreSQL.
- WebSocket disconnects trigger client-side reconnect loops.
- Upon reconnection, clients fetch latest orderbook snapshots and apply sequence deltas to resume real-time streams cleanly.
