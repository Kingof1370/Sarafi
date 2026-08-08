# Advanced Execution Engine

Velyxora implements a rich suite of order execution types and Lifespan (Time-in-Force) controls.

## Supported Order Types
- `LIMIT`: Places a limit order on the book.
- `MARKET`: Executes immediately at the best market price.
- `STOP_LIMIT`: Triggers a limit order when the market crosses the stop price.
- `TAKE_PROFIT_LIMIT`: Triggers a profit-taking limit order.

## Lifespan / Time-In-Force (TIF)
- `GTC` (Good 'Til Cancelled)
- `IOC` (Immediate Or Cancel)
- `FOK` (Fill Or Kill)

## Modifiers
- `PostOnly`: Prevents orders from unexpectedly taking immediate liquidity.
- `ReduceOnly`: Ensures orders can only reduce exposure and never increase it.
