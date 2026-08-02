# Velyxora Exchange — Enterprise Wallet Core

## 1. Purpose & Scope
The Velyxora Enterprise Wallet Core serves as the single source of truth for all digital asset ownership, ledger balances, and address allocations on the exchange. It acts as a bulletproof double-entry settlement ledger system protecting user funds against fraud, concurrency race conditions, and key compromise.

The scope of this sub-system covers:
* Deterministic hierarchical address allocation (BIP-39, BIP-44, BIP-32).
* High-throughput thread-safe balance adjustments.
* Strict operational limits and security boundaries on individual wallet roles.
* Relational database state persistence with complete foreign key checks and cascading constraints.
* Multi-topic real-time Kafka event streaming.

---

## 2. Architecture & Design Decisions
The architecture implements clean domain-driven design (DDD) principles with complete decoupling of database models, blockchain adapters, and API endpoints.

```
       +-------------------------------------------------+
       |                 API Gateway                     |
       +-----------------------+-------------------------+
                               | (HTTP/WebSockets)
                               v
       +-------------------------------------------------+
       |               Wallet Service                    |
       |  +-------------------------------------------+  |
       |  |          PersistentWalletService          |  |
       |  +----+--------------+------------------+----+  |
       |       |              |                  |       |
       |       v              v                  v       |
       |  +----+----+   +-----+----+       +-----+----+  |
       |  | Wallet  |   | Address  |       | Asset    |  |
       |  | Manager |   | Registry |       | Registry |  |
       |  +----+----+   +-----+----+       +-----+----+  |
       |       |              |                  |       |
       |       +--------------+------------------+       |
       |                      |                          |
       |                      v                          |
       |         +------------+------------+             |
       |         | Blockchain Adapters     |             |
       |         +-------------------------+             |
       +----------------------++-------------------------+
                              || (TCP Connection / Pool)
                              vv
               +--------------++-------------+
               |  PostgreSQL  || Kafka Bus   |
               +--------------++-------------+
```

### Design Decisions
1. **In-Memory Speed + DB Consistency:** Reads and validations occur on fast thread-safe memory registries (`sync.RWMutex`) to minimize latency (<200ns lookups), while database writes occur sequentially to PostgreSQL using jackc/pgx connection pools.
2. **Deterministic HD Derivations:** Uses BIP-39 mnemonic phrases and BIP-32 child key derivations to dynamically and safely provision unique deposit addresses without storing private keys in plain-text.
3. **Double-Entry Safeguards:** Transfers use atomic SQL statements with balance segment reservations to prevent negative asset states.

---

## 3. Data Flow & Sequence Flows
When a new address is allocated:
1. Client requests address allocation via `/api/v1/wallet/address` endpoint.
2. API Gateway validates JWT credentials and proxies the request to `PersistentWalletService`.
3. `PersistentWalletService` retrieves the user's wallet, computes the key index, and uses the blockchain adapter to derive the next address.
4. Generates standard public keys and addresses deterministically.
5. Saves the record to Postgres (`wallet_addresses`) and publishes a versioned `AddressGeneratedEvent` on Kafka topic `velyxora-address-generated`.

---

## 4. Security Considerations & HA
* **Cold Wallet Isolation:** Cold wallets are decoupled and strictly forbidden from initiating direct transfers. Relies on out-of-band multisig approvals.
* **Treasury Verification:** Treasury transfers require at least 2 administrative approvals in the validations queue.
* **Double-Entry Verification:** Chained hashes validation prevents database tampering. If a row hash is modified manually, the chain is broken, triggering immediate fail-safes.
* **High Availability:** Fully stateless service runs in active-active configurations, with Redis session maps caching lookup details.

---

## 5. Relational Database Design
See standard PostgreSQL tables in `packages/database/migrations.go`:
* `wallets`
* `wallet_balances`
* `wallet_balance_history`
* `wallet_addresses`
* `wallet_audits`

---

## 6. Event Architecture
Dedicated Kafka topics:
* `velyxora-wallet-created`
* `velyxora-wallet-updated`
* `velyxora-balance-updated`
* `velyxora-address-generated`
* `velyxora-asset-registered`
* `velyxora-wallet-validated`
* `velyxora-wallet-audit`

---

## 7. Logging & Monitoring
* **Logging:** Emits slog structured JSON audits containing trace IDs, latencies, and service parameters.
* **Monitoring:** Exposes `/metrics` Prometheus counters tracking available memory usage, CPU load, and goroutine leak metrics.
