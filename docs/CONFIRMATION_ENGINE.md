# Velyxora Confirmation Engine Manual

This document details the time-to-finality confirmation tracking engine under P0008.

## Confirmation Requirements
The confirmation thresholds are asset and network-specific to reflect standard security profiles:

- **Bitcoin (BTC)**: 2 confirmations (lowered for simulation test finality speed).
- **Ethereum (ETH)**: 3 confirmations.
- **Solana (SOL)**: 5 confirmations.

## Finality Tracking
The `DepositScanner` and `WithdrawalWorker` poll the respective blockchain adapters to retrieve latest block heights and transaction confirmation depths. A transaction is only cleared and settled to user available balances once the required confirmation threshold is achieved.
