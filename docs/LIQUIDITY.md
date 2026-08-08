# VELYXORA EXCHANGE - LIQUIDITY SYSTEMS (P0007)
Details internal and external liquidity abstractions.

## Internal & Aggregated Order Books
- Internal matcher bid/ask price levels define the core market book.
- Analytical stats (Mid-Price, Weighted Mid, Spread, Depth Imbalance, Health Score) are computed in real-time.

## External Liquidity Integrations (Rule 7 & 8)
- plubgable external providers can implement the `ExternalLiquidityProvider` interface.
- Integrations are strictly treated as untrusted boundaries. All incoming match trades must be validated via `ValidateExternalExecution` (verifying prices, quantities, and sequence) and pass standard risk/ledger rules.
