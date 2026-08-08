# Velyxora Deposits Manual

This document details the enterprise-grade deposit monitoring pipeline built under P0008.

## Pipeline Architecture
The deposit scanning pipeline operates as a real-time background loop:

`Blockchain -> Simulator/Adapter -> Scanner (Wallet Service) -> Validation -> Confirmation Tracking -> Wallet Balance Credits & Ledger Entries -> Event Publication (Kafka)`

### 1. Detection
The `DepositScanner` runs as a background routine in `apps/wallet-service`. It pulls registered deposit addresses from `wallet_addresses` and uses the corresponding `BlockchainAdapter` to scan transaction outputs.

### 2. Confirmations Tracking
A detected transaction is initially recorded with `types.DepositPending` (or `types.DepositConfirmed`). The scanner updates confirmation counts per block.

### 3. Crediting & Finality
When the configured confirmation threshold is reached (e.g. BTC = 2, ETH = 3, SOL = 5), the deposit is transitioned to `types.DepositCompleted` state. The balance is credited atomically to the user's account using row-level locking (`SELECT FOR UPDATE`), and corresponding Double-Entry Ledger entries are written.
