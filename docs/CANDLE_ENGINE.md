# Candle Aggregation Engine

The Candle Engine processes matched trades deterministically and aggregates them into candlesticks.

## Timeframes
The following standard intervals are supported:
- `1m`, `5m`, `15m`, `30m`, `1h`, `4h`, `1d`, `1w`

## Calculations
Each candlestick includes:
- Open, High, Low, Close
- Base Volume, Quote Volume
- Trade Count
- Open Time, Close Time

## Persistence
All finalized candles are stored persistently in PostgreSQL (table `candles`) to guarantee durable historical storage, and cached in Redis for fast retrieval.
