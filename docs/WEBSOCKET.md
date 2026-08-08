# VELYXORA EXCHANGE - WEBSOCKET SYSTEM
Details the real-time WebSocket connection manager and streams.

## Connections & Heartbeats
- Bounded queues (size 256) per client ensure non-blocking delivery.
- Slow consumers are safely dropped without bottlenecking the main gateway.
- Enforces strict Ping/Pong read/write deadlines to prevent socket leakage.

## Supported Private & Public Streams
- `market:trades` - Real-time matched executions.
- `market:orderbook` - Incremental level-2 order book depth.
- `market:ticker` - 24-hour symbol stats.
