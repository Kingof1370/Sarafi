# VELYXORA EXCHANGE - DETERMINISTIC OHLCV CANDLE ENGINE
Details candle aggregation and time-step calculations.

## Aggregator Logic
- Real-time matches automatically feed the candle engine inside `MarketServices`.
- Supports 1m, 5m, 15m, 1h, 1d interval boundaries.
- Uses exact price and quantity rounding, tracking Open, High, Low, Close, Volume, QuoteVolume, and TradeCount from real trades.
