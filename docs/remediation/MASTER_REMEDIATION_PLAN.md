# MASTER REMEDIATION PLAN

This master remediation plan covers the full resolution and production-grade refactoring of all critical architectural, security, and algorithmic flaws identified in the Velyxora cryptocurrency exchange repository.

## 1. Problem Audit & Confirmation Status

We have performed a complete, direct, file-by-file audit of all 22,682 lines of code in the repository. Below is the mapping of each of the 16 original problems to their verified source code status.

| No | Issue / Problem Area | Status | Source Code File / Location | Remarks |
|----|----------------------|--------|-----------------------------|---------|
| 1 | Wallet Address Generation | **Confirmed** (Critical) | `packages/common/address_crypto.go` | `GenerateCryptographicAddress` uses raw HMAC-SHA256 hash slicing instead of proper BIP-32/BIP-44 HD key derivations. Non-recoverable and insecure. |
| 2 | Single-Symbol Matcher Bootstrap | **Confirmed** (Critical) | `apps/matching-engine/main.go` | Spawns a single hardcoded matcher `NewMatcher("BTC-USDT")`. Missing dynamic symbol lookup and isolated books. |
| 3 | Double Settlement Paths | **Confirmed** (Critical) | `apps/matching-engine/engine/settlement.go` & `apps/wallet-service/wallet_worker.go` | Post-match balance updates occur synchronously via `SettlementEngine` and asynchronously in `wallet-service` listening on the same `velyxora-trades` topic, introducing serious double-settlement vulnerabilities. |
| 4 | In-Memory Settlement Queue | **Confirmed** (Critical) | `apps/matching-engine/engine/settlement.go` | `SettlementEngine` stores jobs in an in-memory slice `queue []*SettlementJob` with no database persistence. Vulnerable to server crashes and job loss. |
| 5 | Dropped Advanced Order Attributes | **Confirmed** (Critical) | `apps/matching-engine/main.go` | Serialization/conversion steps from `AdvancedOrder` to legacy `types.Order` across Kafka prune critical fields like stop price, reduce only, post only, and iceberg size. |
| 6 | Unsecured Fallback Mock Authentication | **Confirmed** (High) | `apps/backend/main.go` | Fallback mock credentials `test@velyxora.com` / `StrongPass1!` are hardcoded if the database is offline, even when running in production modes. |
| 7 | Static Mock MFA Enrollment | **Confirmed** (High) | `apps/backend/main.go` | Endpoint `/api/v1/mfa/enable` returns hardcoded dummy seeds `JBSWY3DPEHPK3PXP` and static backup codes instead of executing real cryptographic TOTP key provisioning. |
| 8 | Insecure JWT / DB Defaults | **Confirmed** (High) | `apps/backend/main.go` | JWT secrets fall back to default development strings `"super-secret-velyxora-key-999"` instead of failing closed if the environment secret is missing. |
| 9 | Floating-Point (`float64`) Usage for Money | **Confirmed** (Medium) | Throughout matching, execution, settlement, and API models | Critical financial variables are represented using float64, mismatching the `DECIMAL(36,18)` database definitions and risking floating-point rounding errors/dust. |
| 10 | Pricing Engine Priority Core | **Verified Correct** (Positive) | `apps/matching-engine/engine/matcher.go` | Real FIFO Price-Time Priority order matching logic is correctly implemented. |
| 11 | Engine Integration Mismatch | **Confirmed** (Medium) | `apps/matching-engine/main.go` vs `tests/` | Match engine integration code doesn't fully tie PositionEngine or LiquidityBridge into production pipelines, unlike test scenarios. |
| 12 | Simulated Liquidity Fallback | **Confirmed** (Medium) | `apps/matching-engine/engine/liquidity_bridge.go` | External liquidity bridge possesses mock fallback loops that must be safely isolated from active production paths. |
| 13 | Blockchain Operations Gaps | **Confirmed** (Medium) | `apps/wallet-service/wallet_worker.go` | Transaction scanning and confirmations is predominantly ETH-centric, with non-EVM chains relying on thin abstractions. |
| 14 | Simple Address Validation Check | **Confirmed** (Medium) | `packages/common/address_crypto.go` | Simple prefix checking (e.g. starting with "bc1" or "0x") is used instead of cryptographic checksum and alphabet validations. |
| 15 | Missing Infrastructure Manifests | **Partially Confirmed** | Repository structure | The repository contains several Kubernetes manifests and Dockerfiles, but lacks an overarching, ready-to-run overarching docker-compose stack. |
| 16 | Monolithic Frontend page.tsx | **Confirmed** (Medium) | `apps/frontend/src/app/page.tsx` | All terminal widgets, charts, forms, faucet, and dashboard layouts are coupled inside a single 65KB TSX file. |

---

## 2. Dependency Graph

The remediation tasks possess strict dependencies that dictate the order of execution.

```
                  +-----------------------------------------+
                  |  PART 05: Canonical Order Schema        |
                  +--------------------|--------------------+
                                       |
                                       v
                  +-----------------------------------------+
                  |  PART 02: Multi-Symbol Matching Engine  |
                  +--------------------|--------------------+
                                       |
                                       v
                  +-----------------------------------------+
                  |  PART 03: Settlement Consistency        |
                  +--------------------|--------------------+
                                       |
                                       v
                  +-----------------------------------------+
                  |  PART 04: Durable Settlement Queue      |
                  +--------------------|--------------------+
                                       |
                                       v
                  +-----------------------------------------+
                  |  PART 07: Financial Precision           |
                  +--------------------|--------------------+
                                       |
                                       v
                  +-----------------------------------------+
                  |  PART 01: Secure Wallet Derivation      |
                  +--------------------|--------------------+
                                       |
                                       v
                  +-----------------------------------------+
                  |  PART 06: Production Hardening          |
                  +--------------------|--------------------+
                                       |
                                       v
                  +-----------------------------------------+
                  |  PART 08: Integration & E2E Testing     |
                  +-----------------------------------------+
```

---

## 3. Implementation Sequence & Mapping

We will solve and verify all 16 issues in the following sequential Parts, using their dedicated remediation prompts:

### **Part 01: Secure Wallet Derivation and Key Management**
- **Issues Addressed**: Issue 1 (Wallet Address Generation), Issue 14 (Simple Address Validation).
- **Execution Prompt**: `prompts/P01-WALLET-REMEDIATION.md`
- **Actions**: Integrate BIP-39, BIP-32, BIP-44 key trees, actual public key hash derivations, and strict address checksum parsing for BTC, ETH, LTC, TRX, and SOL.

### **Part 02: Multi-Symbol Matching Engine & Market Registry**
- **Issues Addressed**: Issue 2 (Single-Symbol Matcher Bootstrap), Issue 11 (Engine Integration Mismatch), Issue 12 (Simulated Liquidity Fallback).
- **Execution Prompt**: `prompts/P02-MULTI-SYMBOL-MATCHING.md`
- **Actions**: Build `MarketRegistry` to isolate, lookup, route, sequence, and manage independent matchers and order books for all active asset pairs.

### **Part 03: Settlement Consistency & Double Settlement Prevention**
- **Issues Addressed**: Issue 3 (Double Settlement Paths).
- **Execution Prompt**: `prompts/P03-SETTLEMENT-CONSISTENCY.md`
- **Actions**: Consolidate trade balance mutations into a single canonical settlement consumer. Remove double balance posting in `wallet-service` and enforce transaction boundaries.

### **Part 04: Durable Settlement Queue & Outbox Recovery**
- **Issues Addressed**: Issue 4 (In-Memory Settlement Queue).
- **Execution Prompt**: `prompts/P04-DURABLE-SETTLEMENT.md`
- **Actions**: Replace RAM-based slices with PostgreSQL-backed durable queue tracking, outbox transactional writes, and startup state recoveries.

### **Part 05: Canonical Order Schema & Kafka Contract Preservation**
- **Issues Addressed**: Issue 5 (Dropped Advanced Order Attributes).
- **Execution Prompt**: `prompts/P05-ORDER-SCHEMA.md`
- **Actions**: Standardize `types.Order` with all complex fields (StopPrice, PostOnly, Iceberg, ReduceOnly), ensuring complete lossless serialization over Kafka brokers.

### **Part 06: Production Security Hardening & MFA Enrollment**
- **Issues Addressed**: Issue 6 (Fallback Mock Auth), Issue 7 (Static Mock MFA), Issue 8 (Insecure Defaults), Issue 13 (Blockchain Operations Gaps).
- **Execution Prompt**: `prompts/P06-PRODUCTION-HARDENING.md`
- **Actions**: Remove JWT fallback secrets, database fallbacks, and test user login paths. Implement fully dynamic cryptographic TOTP provisioning and MFA confirmation.

### **Part 07: Financial Precision Audit & Decimal Migrations**
- **Issues Addressed**: Issue 9 (Floating-Point Money Representation).
- **Execution Prompt**: `prompts/P07-FINANCIAL-PRECISION.md`
- **Actions**: Transit financial variables, order match loops, fees, and balance calculations from `float64` to exact, high-precision rational/decimal arithmetic.

### **Part 08: Integration Testing & Frontend Connection Verification**
- **Issues Addressed**: Issue 15 (Missing Infrastructure Manifests), Issue 16 (Monolithic Frontend page.tsx).
- **Execution Prompt**: `prompts/P08-INTEGRATION-TESTING.md`
- **Actions**: Establish true WebSocket and REST subscriptions between frontend visual widgets and backend services. Write automated E2E integration test suites.
