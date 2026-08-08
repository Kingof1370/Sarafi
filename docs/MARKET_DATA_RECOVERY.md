# Market Data Recovery

The Velyxora Market Data pipeline has been engineered to recover gracefully from infrastructure service disruptions:

- **PostgreSQL / DB Restart**: Candles and historical data can be re-queried, and the state recovers safely.
- **Redis Restart**: Latest hot states are rebuilt from the database on cache miss.
- **WebSocket Disconnects**: Clients detect disconnects and use our sequence-based synchronization protocol to resume without losing state consistency.
