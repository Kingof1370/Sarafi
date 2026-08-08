# Velyxora Transaction Processing Manual

This document details the transaction construction, signing, and broadcasting pipelines under P0008.

## Processing Pipeline

### 1. Construction
The `WithdrawalWorker` utilizes the `BlockchainAdapter` to construct valid transactions. This includes:
- Nonce Tracking for accounts-based chains (e.g. Ethereum/EVM).
- UTXO tracking and consolidation for UTXO-based chains (e.g. Bitcoin).

### 2. Cryptographic Signing
All constructed transactions are cryptographically signed using simulated HSM providers or local private keys. Under no circumstances are raw private keys saved, cached, or logged.

### 3. Broadcasting & Polling
Signed raw transaction hexes are transmitted using the adapter's broadcast API. The transaction's on-chain confirmation status is polled in the background until it reaches full finality.
