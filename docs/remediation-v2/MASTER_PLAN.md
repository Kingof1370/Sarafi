# Velyxora Exchange: V2 Master Remediation Plan

This master plan structures the logical sequencing and execution boundaries of all 15 remediation prompts required to convert the Velyxora cryptocurrency exchange repository into a highly secure, precision-safe, and production-ready system.

---

## 1. Sequence & Mapping

| Prompt ID | Objective | Dependency | Execution Boundary | Remarks |
|-----------|-----------|------------|-------------------|---------|
| **P04** | Aligned Canonical Order Schema | None | `packages/types/`, `main.go`, `oms_handlers.go` | Added advanced, algorithmic fields directly inside types.Order to achieve lossless Kafka serialization. |
| **P03** | Multi-Market Matching Registry | P04 | `apps/matching-engine/engine/`, `main.go`, `oms_handlers.go` | Implemented thread-safe isolated Matcher books and registry. |
| **P05** | Single Canonical Settlement | P03 | `apps/wallet-service/main.go`, `settlement.go` | Dismantled duplicate settlement balance mutation loops. |
| **P06** | Durable Outbox Settlement Queue | P05 | `apps/matching-engine/engine/settlement.go` | Persists execution jobs inside settlements_queue database tables. |
| **P07** | Rational Financial Precision | P06 | `packages/common/precision.go`, `settlement.go` | Transit all balance mutations to precise math/big.Rat arithmetic. |
| **P02** | Secure BIP-44 HD Wallet Derivation | P07 | `packages/common/address_crypto.go` | Deterministic address generation for BTC, ETH, LTC, TRX, and SOL. |
| **P08** | Security & Production Hardening | P02 | `apps/backend/main.go`, `oms_handlers.go`, `mfa.go` | Dynamic TOTP provisioning, Fail-Closed secret loading, mock login bypass removal. |
| **P01** | Structural & Architecture Cleanup | P08 | Throughout packages and main apps | Dead code removal, circular imports checks, and boundary separation. |
| **P09** | Blockchain Deposit / Confirmation | P01 | `apps/wallet-service/wallet_worker.go` | Safe confirmation depths, block heights scanning states tracking. |
| **P10** | Risk Calculations & Liquidity | P09 | `apps/matching-engine/engine/risk.go`, `position.go` | Verification of margin ratios, liquidation, leverage bands. |
| **P11** | Backend Gateway REST / WS APIS | P10 | `apps/backend/main.go`, `websocket.go` | Live endpoints tracking, Slow consumer websocket thread-safe protects. |
| **P12** | Frontend Runtime Integration | P11 | `apps/frontend/src/app/page.tsx`, `api.ts` | Real-time WS orderbook and ticker connections, mock data removal. |
| **P13** | Infrastructure & Manifests Hardening | P12 | `infrastructure/`, `Dockerfile` | Hardened minimal container runtimes, non-root system users. |
| **P14** | Automated Regression & Fail Testing | P13 | `tests/remediation_integration_test.go` | Failure testing, Kafka retry, crash recovery tests. |
| **P15** | Final Security & Code Audit | P14 | Entire repository | Final sweep for floating point drift, left-over mock keys, credentials. |

---

## 2. Definition of Done (DoD) per Prompt

Every single part is considered completed if and only if it conforms to:
1. **Source Code modified**: Actual edits applied directly in correct boundary.
2. **Runtime wired**: Logic fully imported and executed during runtime.
3. **Tests added**: Dedicated tests created under standard package file naming.
4. **Tests pass**: Ran package-wide tests successfully.
5. **Documentation updated**: MD files added describing work.
6. **Git Commit created**: Dedicated, descriptive git commit recorded.
