# Market Data Layer

The Velyxora Market Data Layer coordinates real-time and historical pricing, volume, and order book information derived exclusively from the authoritative trading core.

## REST APIs

The public REST endpoints exposed under `/api/v1/market/` are:

- `GET /api/v1/market/ticker`: Returns the latest pricing, 24H volume, and price changes.
- `GET /api/v1/market/depth`: Returns Level 2 order book snapshots with sequence numbers.
- `GET /api/v1/market/trades`: Returns recent trade matches.
- `GET /api/v1/market/candles`: Returns aggregated candlestick OHLCV data.
- `GET /api/v1/market/stats`: Returns real-time market stats, including mid-price, spread, and health scores.

## Architecture

```
Matching Engine
      ↓
Authoritative State
      ↓
OnTradeMatched Callback
      ↓
Candle Engine + WebSockets + Tickers
```
