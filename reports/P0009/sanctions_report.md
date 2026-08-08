# P0009 Sanctions Screening Report

## Sanctions Provider Abstraction
Screens users, withdrawal addresses, deposit sources, and institutions.

### Fail-Closed Assurance
- Provider states: `CLEAR`, `MATCH`, `POTENTIAL_MATCH`, `UNAVAILABLE`, `ERROR`
- If the screening service goes offline or returns an error, the engine defaults to a closed state.
- Withdrawals are immediately held, preventing raw ledger entries or on-chain broadcasting.
- Fully simulated and verified under robust production offline timeout mocks.
