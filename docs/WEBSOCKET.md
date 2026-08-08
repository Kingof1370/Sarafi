# WebSocket Gateway and Streams

The Velyxora API Gateway features a hardened, high-throughput WebSocket server under `/ws` supporting public and private subscription channels.

## Streams
- `market:ticker`: Pushes real-time 24H rolling ticker stats.
- `market:depth`: Pushes Level 2 order book updates.
- `market:trades`: Pushes executed trade details.
- `market:candles`: Pushes the latest candlestick updates.
- `market:stats`: Pushes live spread, depth imbalance, and health scores.

## Slow Consumer Protection
To prevent slow subscribers from blocking the broadcast pipeline, Velyxora implements an active slow-consumer protection mechanism:
1. Every client connection has a bounded outbound queue of 256 messages.
2. Writes are non-blocking with standard timeouts.
3. If a client's outbound queue fills up, the gateway marks the connection as unhealthy and safely disconnects it, cleaning up its subscriptions and releasing resources.
