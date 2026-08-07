# Velyxora Exchange — Enterprise Wallet Core & Institutional Custody

## 1. Purpose & Scope
The Velyxora Enterprise Wallet Core serves as the single source of truth for all digital asset ownership, ledger balances, address allocations, and institutional digital asset custody on the exchange. It acts as a bulletproof double-entry settlement ledger system protecting user and company funds against fraud, concurrency race conditions, key compromise, and malicious administrative insider threats.

The scope of this sub-system covers:
* Deterministic hierarchical address allocation (BIP-39, BIP-44, BIP-32 over `secp256k1`).
* High-throughput thread-safe balance adjustments.
* Strict operational limits and security boundaries on individual wallet roles (Hot, Warm, Cold, Treasury, Operational, Fee, Reserve, Recovery).
* Digital Asset Custody Vault management (Institutional Hot, Warm, Cold, Deep Cold air-gapped, Treasury, Reserve, and Recovery vaults).
* 4-Eyes Multi-Signature admin approval workflow for digital asset transfers from Cold, Deep Cold, or Treasury vaults.
* System-wide secure Emergency Freeze & Unfreeze platform halt.
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
       |         | - secp256k1 BTC/EVM/LTC |             |
       |         | - Ed25519 Solana/TRX    |             |
       |         +-------------------------+             |
       |                      |                          |
       |                      v                          |
       |         +------------+------------+             |
       |         | Custody Manager         |             |
       |         | - Multi-Sig Approvals   |             |
       |         | - Emergency Freeze      |             |
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
2. **Deterministic HD Derivations:** Uses BIP-39 mnemonic phrases and BIP-32 child key derivations over the standard `secp256k1` Koblitz curve to dynamically and safely provision unique deposit addresses without storing private keys in plain-text.
3. **Double-Entry Safeguards:** Transfers use atomic SQL statements with balance segment reservations to prevent negative asset states.
4. **4-Eyes Principle for Custody:** High-risk institutional vault segments (Cold, Deep Cold, Treasury) mandate at least 2 independent administrative approvals before any out-bound asset movement can be committed.
5. **Fail-Safe Emergency Freeze:** Operators can trigger a platform-wide lock, suspending all vaults and transfers instantly to prevent exploitation.

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
* **Cold Vault Isolation:** Cold vaults are decoupled and strictly forbidden from initiating direct transfers. Relies on out-of-band multisig approvals.
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
* `custody_vaults`
* `vault_assets`
* `custody_transfers`
* `custody_transfer_approvals`
* `custody_audits`
* `custody_emergency_actions`

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

---

## 8. Production Observability & Operational Incident Response
Velyxora implements an advanced supervisor platform and operations cockpit to achieve 99.999% uptime and auto-healing capabilities.
* **Components Health Status:** Centralized telemetry engine tracks real-time status (`HEALTHY`, `DEGRADED`, `UNHEALTHY`) of core subsystems (Postgres, Redis, Kafka, Wallet Core, Custody, Treasury, Node connectivity).
* **Automated Emergency Escalations:** If critical components (Database, Kafka) go unhealthy, the supervisor instantly opens a Critical System Incident and generates severe security alerts.
* **Transaction Fraud Engine:** Real-time scoring analyzes geo-distances, impossible travel speed, and blacklisted network IP profiles. Any score $\ge 0.7$ automatically blocks the transaction and triggers immediate security notifications.
* **Operational Self-Healing Recovery:** Exposes automatic recovery mechanics to let human operators or automated kubernetes scripts trigger healing procedures, restoring unhealthy components to default active operations securely.
* **Observability API Cockpit:** Exposes production-ready HTTP REST endpoints:
  - `GET /monitoring/health` — Subsystem health metrics mapping.
  - `GET /monitoring/incidents` — Live system incident queues.
  - `GET /monitoring/fraud` — Flagged transaction security alerts.
  - `GET /monitoring/recovery` — Historical self-healing event traces.
  - `POST /monitoring/recovery/trigger` — Dispatches automated node recovery routines.
