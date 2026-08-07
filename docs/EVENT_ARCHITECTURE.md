# Velyxora Exchange — Real-Time Kafka Event Architecture

## 1. Overview
The Velyxora sub-systems communicate asynchronously using apache Kafka to guarantee high throughput, loose coupling, and backward compatibility.

---

## 2. Dedicated Wallet Subsystem Topics
We publish structured, versioned event schemas on dedicated, independent topics:

### 2.1 velyxora-wallet-created
Emitted when a new Hot/Warm/Cold/Treasury wallet is provisioned.
* **Schema (v1):** `WalletCreatedEvent` (fields: `version`, `wallet_id`, `user_id`, `type`, `timestamp`)

### 2.2 velyxora-wallet-updated
Emitted when a wallet's status changes (such as locking or unlocking operations).
* **Schema (v1):** `WalletUpdatedEvent` (fields: `version`, `wallet_id`, `is_locked`, `timestamp`)

### 2.3 velyxora-balance-updated
Emitted on any available/locked/reserved/pending segment balance updates.
* **Schema (v1):** `BalanceUpdatedEvent` (fields: `version`, `wallet_id`, `asset`, `available`, `locked`, `reserved`, `pending`, `total`, `timestamp`)

### 2.4 velyxora-address-generated
Emitted when a unique deposit address is deterministically derived and allocated to a user.
* **Schema (v1):** `AddressGeneratedEvent` (fields: `version`, `user_id`, `network`, `address`, `derivation_path`, `timestamp`)

### 2.5 velyxora-asset-registered
Emitted when support for a new token is registered in the exchange registries.
* **Schema (v1):** `AssetRegisteredEvent` (fields: `version`, `symbol`, `name`, `type`, `precision`, `base_network`, `timestamp`)

### 2.6 velyxora-wallet-validated
Emitted when structural health or integrity validations are run.
* **Schema (v1):** `WalletValidatedEvent` (fields: `version`, `wallet_id`, `is_valid`, `error_count`, `timestamp`)

### 2.7 velyxora-wallet-audit
Emitted when a cryptographically chained audit record is appended to the ledger logs.
* **Schema (v1):** `WalletAuditEvent` (fields: `version`, `audit_id`, `user_id`, `wallet_id`, `asset`, `action`, `amount`, `prev_balance`, `new_balance`, `hash`, `timestamp`)
