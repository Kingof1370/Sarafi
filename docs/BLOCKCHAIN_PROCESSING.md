# Velyxora Exchange — Blockchain Processing & Listeners

## 1. Overview
The Blockchain Processing subsystem ingresses blocks, parses receipts, and publishes raw transaction profiles to the Velyxora Wallet and Deposit Services.

---

## 2. Blockchain Ingress Sequence Flow
1. **Block Listeners:** Listeners hook to native nodes or RPC interfaces, subscribing to new block mined events.
2. **Tx Parsers:** For each transaction in a block, the parser validates destination addresses using standard `BlockchainAdapter` regular expressions.
3. **Ingress Filtering:** Transactions matching any user's allocated address are pushed into the `DepositEngine` for validation, duplicate/replay checking, and confirmation increments.

---

## 3. Supported Networks
* **Bitcoin**
* **Ethereum**
* **BNB Smart Chain**
* **Polygon**
* **Solana**
* **Avalanche**
* **Tron**
* **Litecoin**

Future networks are added seamlessly by implementing the `BlockchainAdapter` interface and registering them inside `AdapterRegistry`.
