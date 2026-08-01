# VELYXORA EXCHANGE - LIQUIDITY ENGINE
This manual details the metrics, spreads, and market health statistics computed by the Liquidity Engine.

## Computed Metrics
- **Bid/Ask Spread:** Absolute difference between best bid and best ask.
- **Mid Price:** Simple average of best bid and ask.
- **Weighted Mid Price:** Average of bid/ask prices weighted by their inverse quantities, projecting true market directions.
- **Depth Imbalance:** Ratio between bid and ask depth volume sheets, flagging liquidity skews.
- **Market Health Score:** A rolling score from 0 to 100 based on narrowness of spread and balanced liquidity distribution.
