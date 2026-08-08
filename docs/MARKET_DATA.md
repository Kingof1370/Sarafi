# VELYXORA EXCHANGE - MARKET DATA ARCHITECTURE (P0007)
Details real-time and rest-based market data systems.

## Data Pipelines
- All market data originates from the authoritative matching/order-book state.
- Live updates are pushed to the WebSocket Gateway, and historical queries are processed via REST API endpoints.
- No artificial, hardcoded, or fake values are used in production flows.
