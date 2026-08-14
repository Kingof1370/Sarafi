# VELYXORA Exchange — Remediation v3 Baseline Audit (Phase 0 / Phase 1)

Method: source code only. Documentation, prior reports under `reports/` and previous
commit messages were NOT treated as evidence. Every finding below cites a file and line
range in the current default branch.

Repository shape: Go workspace (`go.work`), 92 Go files / ~22k LOC,
`apps/{backend,matching-engine,wallet-service,frontend}`, `packages/{common,custody,database,logger,security,types}`,
`tests/`, `infrastructure/`.

## Severity summary

| Severity | Count |
|---|---|
| CRITICAL | 6 |
| HIGH | 7 |
| MEDIUM | 5 |
| LOW | 3 |

---

## CRITICAL

### PB-01 — secp256k1 public key derivation is mathematically fake
- Component: wallet / cryptography
- Files: `packages/common/address_crypto.go:194-235` (`DeriveSECP256K1PublicKey`)
- Current implementation: `x = Gx * k mod p`, `y = Gy * k mod p`. This is scalar
  multiplication of the *coordinates*, not elliptic-curve point multiplication.
- Consequence: every secp256k1-derived address (BTC, ETH/EVM, BNB, Polygon, Avalanche,
  TRON, LTC) is derived from a non-point. Funds sent to such addresses are unspendable —
  no private key can ever sign for them.
- Financial impact: total, irreversible loss of any deposit.
- Required fix: replace with a real secp256k1 implementation
  (`github.com/decred/dcrd/dcrec/secp256k1/v4`) and validate against BIP-32 test vectors 1–3
  and known BTC/ETH address vectors.

### PB-02 — No BIP-39 layer; BIP-32 normal derivation depends on PB-01
- Files: `packages/common/address_crypto.go:148-192`, `:236+`
- `GenerateCryptographicAddress` starts from a raw `seed []byte` with HMAC-SHA512("Bitcoin seed").
  There is no mnemonic → seed (PBKDF2-HMAC-SHA512, 2048 iterations) step anywhere in the repo
  (`rg bip39` → 0 hits), so wallet recovery from a mnemonic is not implementable today.
- Non-hardened CKDpriv calls the broken `DeriveSECP256K1PublicKey` (comment in source itself
  says "mock fallback for test suite compatibility"), so non-hardened paths are invalid.

### PB-03 — Two independent, divergent trading runtimes
- Files: `apps/matching-engine/main.go:79-90` and `apps/backend/oms_handlers.go:40-50`
- Both construct their own `OMSStateMachine`, `RiskEngine`, `Matcher`, `ExecutionEngine` and
  `SettlementEngine`. The backend builds `NewSettlementEngine(nil)` — a settlement engine with
  no database — so any order routed through the API service settles in memory only and is lost
  on restart, while the matching-engine service settles against Postgres.
- Violates single-owner / single-runtime-path for order matching and settlement.

### PB-04 — Hardcoded single market in production runtime
- Files: `apps/matching-engine/main.go:79`, `apps/backend/oms_handlers.go:44` — `NewMatcher("BTC-USDT")`
- `MarketRegistry` exists (`apps/matching-engine/engine/market_registry.go`) and is correct, but the
  production wiring does not use it; `OMSRouter` holds one `matcher`. Cross-market isolation is
  therefore unproven in the runtime path: an ETH-USDT order reaches the BTC-USDT book.

### PB-05 — Mock authentication and fixed MFA secret in the API surface
- Files: `apps/backend/main.go:461-474` (mock login `test@velyxora.com` / `StrongPass1!`,
  `usr_mock_123`), `apps/backend/main.go:731-737` (`POST /v1/mfa/enable` returns the fixed
  secret `JBSWY3DPEHPK3PXP` and static backup codes)
- The login path is guarded by `appEnv == "production"`, but `/mfa/enable` is NOT guarded at all:
  it returns a constant TOTP secret in every environment, stores nothing, and never verifies
  enrollment. `packages/security/mfa.go` already has a correct `GenerateTOTPSecret`, unused here.
- Also `apps/backend/main.go:70-78` still embeds the literal `super-secret-velyxora-key-999`
  as a non-production JWT default.

### PB-06 — float64 across the whole financial core
- Files: `packages/common/precision.go` (entire), `packages/types/types.go`,
  `apps/matching-engine/engine/{matcher,execution,fees,risk,position,settlement}.go`, ~280 hits
- `SafeAdd/SafeSub/SafeMul/SafeDiv` convert to `big.Rat` and then back to `float64`, so the
  binary rounding error is re-introduced on every return. `RoundToPrecision`, `EnforceTickSize`
  and `EnforceLotSize` use `math.Pow`/`math.Round` on floats.
- Prices, quantities, balances, fees, PnL, margin and ledger amounts are all `float64`.

---

## HIGH

### PB-07 — Settlement durability is best-effort, not transactional outbox
- File: `apps/matching-engine/engine/settlement.go:59-90`
- The in-memory `queue` slice is the primary queue; the Postgres insert into `settlements_queue`
  is fire-and-forget (`_, _ = se.db.Pool.Exec(...)`, errors discarded) and happens outside any
  transaction with the execution write. A crash between execution and insert loses the settlement.

### PB-08 — Settlement idempotency is not enforced at the schema level
- `SettlementJob.ID = "set_job_" + exec.TradeID` gives a derived key, but there is no
  `UNIQUE(execution_id)` constraint proven in `packages/database/migrations.go`, and the retry
  path can re-run ledger mutations. One execution → exactly one settlement is not guaranteed.

### PB-09 — KMS mock fallback inside production key paths
- File: `packages/security/keys.go:55-130, 195-205`
- When AWS/Vault configuration is absent, encryption/decryption silently falls back to
  `AWSKMSMock`/`VaultKMSMock` with a hardcoded 32-byte key. Production must fail closed instead.

### PB-10 — No double-entry ledger
- There is no ledger package; balance mutations occur inside settlement without paired
  debit/credit rows, so there is no accounting trail or reconciliation source of truth.

### PB-11 — Simulated liquidity reachable from the runtime
- File: `apps/matching-engine/engine/liquidity_bridge.go:36-140, 279`
- `simulated` mode synthesises an order book. It must be impossible to enable when
  `APP_ENV=production`.

### PB-12 — Order pipeline field loss
- `apps/matching-engine/engine/oms_types.go` / `oms_router.go` vs `apps/backend/oms_handlers.go`:
  the HTTP layer builds orders locally rather than sharing a canonical DTO, so
  `ClientOrderID`, `PostOnly`, `ReduceOnly`, `Iceberg`, `Sequence` are not guaranteed end to end.

### PB-13 — Frontend is a shell
- `apps/frontend/src/app/page.tsx`, `src/lib/api.ts`, `src/store/authStore.ts` only —
  no trading, wallet, MFA or WebSocket screens are wired to the backend.

---

## MEDIUM

- PB-14 — `apps/backend/main.go` is a 1534-line god file mixing config, routing, auth and business logic.
- PB-15 — Committed build noise: `apps/frontend/next_output.log`, `npm_output.log`.
- PB-16 — Mainnet/testnet separation is not enforced by type; asset strings drive address logic.
- PB-17 — No reorg handling or confirmation-depth policy per chain in `packages/common/blockchain.go`.
- PB-18 — `reports/` holds 12 status folders asserting completion that source code contradicts.

## LOW

- PB-19 — Backup codes limited to 3 in `GenerateTOTPSecret`.
- PB-20 — TOTP window ±1 step with no replay cache (a code can be reused inside its window).
- PB-21 — No `go vet`/lint/test gate in `infrastructure/ci-cd`.

---

## Verification status

`go test ./...` and `go vet ./...` were NOT executed in this pass — the audit sandbox has no Go
toolchain installed yet. No result is claimed as passing. Test execution is a precondition for
declaring any Part complete and is scheduled at the start of P01.
