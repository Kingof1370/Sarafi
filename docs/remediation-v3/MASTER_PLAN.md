# VELYXORA Remediation v3 — Master Plan

Ordering is dependency-driven, not severity-driven: correctness of the runtime graph (P01)
and of money representation (P07 primitives) gate everything downstream.

| Part | Scope | Blocks |
|---|---|---|
| P01 | Architecture & runtime wiring: single trading core owned by matching-engine, backend becomes a client; delete the duplicate engine stack in `oms_handlers.go`; remove committed build logs | P03,P04,P05,P11 |
| P02 | Wallet & cryptography: real secp256k1, BIP-39/32/44, per-chain address encoding, test vectors | P09 |
| P03 | Multi-symbol matching: `MarketRegistry` becomes the only entry point; per-market book+matcher; cross-market isolation tests | P04 |
| P04 | Order pipeline: one canonical order DTO in `packages/types`, field-preservation across API → OMS → matching → execution | P05 |
| P05 | Settlement architecture: canonical settlement path, `ExecutionID`/`SettlementID`/idempotency key, DB unique constraint, atomic transaction | P06,P07 |
| P06 | Durable settlement: transactional outbox + Kafka consumer + recovery on boot; crash/restart/replay tests | P14 |
| P07 | Ledger & financial precision: double-entry ledger, integer/decimal money type replacing float64 on all financial paths | P10 |
| P08 | Security & MFA: fail-closed secrets, remove mock auth and fixed TOTP secret, real enrollment with storage, recovery codes, rotation; remove KMS mock fallbacks | P11 |
| P09 | Blockchain: confirmations, reorg handling, deposit detection, withdrawal idempotency, strict mainnet/testnet split | P10 |
| P10 | Risk / margin / position on top of the ledger; no fake balances or simulated liquidity in production | P11 |
| P11 | Backend API & WebSocket: thin transport over the trading core, auth, validation, idempotency, pagination, real events | P12 |
| P12 | Frontend integration: login, MFA, balances, orders, book, trades, positions, wallet, deposits/withdrawals over real APIs and WS | P14 |
| P13 | Infrastructure: compose/k8s parity with the service graph, migrations, health/readiness, secrets | P14 |
| P14 | Integration, E2E and failure testing (crash, replay, duplicate, network partition, concurrency) | P15 |
| P15 | Final source and architectural audit + final report | — |

## Definition of Done (applies to every Part)
1. Source code changed in the architecturally correct package.
2. New code imported, instantiated and reachable from the real runtime path.
3. Old/duplicate implementation removed or redirected — never left active in parallel.
4. Tests added, including failure scenarios.
5. `go build ./...`, `go vet ./...`, `go test ./...` executed and output recorded.
6. Independent commit with a real diff (documentation-only commits never close a Part).
