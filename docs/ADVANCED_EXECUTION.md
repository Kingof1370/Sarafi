# VELYXORA EXCHANGE - ADVANCED TRADING EXECUTION
Details advanced order execution and lifecycle properties.

## Supported Advanced Mechanics
- **IOC (Immediate Or Cancel):** Fills immediate portion and cancels remainder.
- **FOK (Fill Or Kill):** Fills entire size immediately or cancels the order entirely.
- **Post-Only:** Protects maker status. Rejects orders if they would execute immediately as taker.
- **Market Orders:** Fills immediately against bids/asks depth, Cancelling any unfilled portion.
- **Stop Limits/Stop Losses:** Untriggered stop levels monitored in price-cross loops before entering limit queues.
