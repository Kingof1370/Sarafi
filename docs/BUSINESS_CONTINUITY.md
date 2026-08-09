# VELYXORA EXCHANGE - BUSINESS CONTINUITY PLAN (BCP)

## 1. CRITICAL MICROSERVICES RECOVERY ORDER
To ensure complete system stability and avoid split-brain execution or sequence drifts during recovery, the Velyxora exchange platform must be restored in the following order:

1. **PostgreSQL Database Instance:** Establish authoritative balances and ledger records.
2. **Apache Kafka Stream Brokers:** Initialize the asynchronous event backplane.
3. **Redis Caching Nodes:** Setup rate limit tracking and active session records.
4. **Matching Engine (Stateful Core):** Replay sequence journals to rebuild in-memory order books.
5. **Wallet Service (Settlement Workers):** Startup double-entry balance verification loops.
6. **API Gateway Nodes:** Start routing traffic.
7. **Market Data Pipeline:** Re-consume trade streams.
8. **Frontend Terminal Dashboard:** Expose user operations.

## 2. RECOVERY WORKFLOWS & ALIGNMENTS
- **Postgres Advisory Locking:** Active stateful Matching Engine and Wallet Worker instances must acquire transactional advisory locks before writing to PostgreSQL to guarantee single-writer exclusivity and avoid double matching/settlement.
- **Idempotent Redis Reconstruction:** After cache restarts, sessions and API keys are re-cached asynchronously by querying PostgreSQL database tables. This lazy-loading recovery ensures that Redis does not become a source of truth.
- **Fail-closed Sanctions Verification:** Sanctuary screening results must fail closed in case of RPC connection losses to protect the platform's security framework.
