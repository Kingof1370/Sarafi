# Velyxora Exchange — Enterprise Deposit Engine

## 1. Purpose & Scope
The Deposit Engine is a key module responsible for detecting, validating, and finalising inbound digital asset transfers from supported blockchain networks. It acts as an automated validation pipeline that protects the platform against replay attacks, double spending, and incorrect balance credits.

---

## 2. Core Components & Architecture
```
+------------------------------------------------------------+
|                       Deposit Engine                       |
|   +-----------------------+-----------------------------+  |
|   | Deposit Processor     |  Ingresses block tx streams |  |
|   +-----------------------+-----------------------------+  |
|   | Confirmation Engine   |  Tracks confirmations depth |  |
|   +-----------------------+-----------------------------+  |
|   | Validation & Dups     |  Validates unique tx hashes |  |
|   +-----------------------+-----------------------------+  |
+------------------------------------------------------------+
```

* **Deposit Processor:** Parses and enqueues inbound blockchain transactions.
* **Confirmation Engine:** Tracks confirmations depth, finalizes completed transactions, and manages chain reorganizations.
* **Validation Sub-engine:** Replay protection, unique transaction checking, and minimum limits validation.
* **Deposit Event Service:** Emits real-time versioned Kafka events on state progressions.

---

## 3. Real-Time Confirmation Rules
We enforce strict transaction confirmation depths to protect against fork rollbacks:
* **Bitcoin (BTC):** 6 confirmations (Required confs)
* **Ethereum (ETH / ERC-20):** 12 confirmations
* **BNB Smart Chain (BNB):** 12 confirmations
* **Polygon (MATIC):** 12 confirmations
* **Solana (SOL):** 1 confirmation (Immediate finality)
* **Avalanche (AVAX):** 12 confirmations
* **Tron (TRX):** 15 confirmations
* **Litecoin (LTC):** 6 confirmations

---

## 4. Chain Reorganizations & Failure Recoveries
* **Chain Reorganizations:** In the event of a chain fork rollback, `HandleChainReorganization` resets the status of all transactions mined at or after the fork block height back to `PENDING` and zeroes out their confirmation count. This blocks potential double spends and protects the ledger.
* **Orphan Transactions:** Transactions omitted from the main chain are automatically set to `FAILED` status, preventing any crediting of user accounts.
