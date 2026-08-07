# VELYXORA EXCHANGE - TRADING ARCHITECTURE
Summary of the complete hardened trading core of Velyxora.

## Key Architectures (P0006)
1. **API Gateway & Router (`apps/backend`):** High-speed entry point routing validated requests downstream.
2. **OMS State Machine (`apps/matching-engine`):** Coordinates 12 unique states sequentially.
3. **Price-Time priority Matcher:** Deterministic price levels matching.
4. **Persistent Risk Engine:** Performs pre-trade database-backed SELECT FOR UPDATE checks.
5. **Durable double-entry Ledger (`packages/database`):** Relational transactional integrity.
