# VELYXORA EXCHANGE - ORDER MANAGEMENT SYSTEM (OMS)
The OMS is the central authority managing every order's lifecycle.

## Order Lifespan States
The system enforces strict state machine rules across 12 unique states:
`CREATED` -> `PENDING_VALIDATION` -> `VALIDATED` -> `ACCEPTED` -> `QUEUED` -> `PARTIALLY_FILLED` -> `FILLED`, `CANCELLED`, `EXPIRED`, `REJECTED`, `FAILED`, `ARCHIVED`.

## Validation Pipeline
Each validation check is implemented as an independent, modular step:
1. **Trading Pair & Market Status Validator**
2. **Min/Max Size & Notional Limit Validator**
3. **Tick & Step Size Precision Validator**
4. **Risk & Balance Hold Validator**
