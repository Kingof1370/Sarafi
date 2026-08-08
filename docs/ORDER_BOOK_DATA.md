# VELYXORA EXCHANGE - ORDER BOOK DATA STREAMS
Describes REST orderbook and WebSocket depth delivery.

## Snapshot & Delta Synchronization
- Clients can query order book snapshots via `GET /api/v1/oms/orderbook`.
- Delta updates are streamed continuously via `market:orderbook` over WebSocket.
- Deterministic sequence numbers allow clients to verify integrity and perform resynchronization on packet loss.
