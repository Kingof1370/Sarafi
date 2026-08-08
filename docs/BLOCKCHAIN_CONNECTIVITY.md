# Velyxora Blockchain Connectivity Manual

This document details the blockchain connectivity layer under P0008.

## Connectivity Modes
The system implements a strict separation of connectivity profiles:

- **LIVE**: Connects directly to real on-chain nodes via configured RPC endpoints.
- **TESTNET**: Connects to public sandbox nodes (Goerli, Sepolia, etc.).
- **SIMULATION**: Uses the stateful, thread-safe, in-memory `BlockchainSimulator` to test block scan triggers, confirmations, and reorgs locally.

## Mode Strictness
To prevent accidental silent selection of simulation/mock modes in production:
- The system defaults to `LIVE` connectivity.
- A hard stop checks if `BLOCKCHAIN_MODE=SIMULATION` and `ENV=production`, throwing a critical validation error on initialization.
