# VELYXORA EXCHANGE - ENTERPRISE TRADING ENGINE
This manual details the architecture of our modular trading core.

## Modular Systems
- **Order Management System (OMS):** Core gateway for validating, routing, and tracking order lifecycles.
- **Matching Engine:** High-frequency, in-memory Price-Time Priority limit order matcher.
- **Execution Core:** Processes trades and handles matching calculations.
- **Settlement Core:** Handles atomic balance postings.
- **Risk Core:** Validates margins and sufficient available balances.
- **Fees Core:** Supports tiered taker/maker fees.
