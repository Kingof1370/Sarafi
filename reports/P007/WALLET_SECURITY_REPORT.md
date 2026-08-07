# Velyxora Exchange — P007 Wallet Core & Custody Security Report

## Metadata
* **Execution Timestamp:** 2026-08-02 23:45:00 UTC
* **Git Commit Hash:** [PROD_READY_COMMIT]

## Wallet & Custody Security Foundations
* **Cold Wallet Isolation:** Cold wallets are fully isolated and direct transfers are rejected unless authorized via multi-stage authorizations.
* **Standard-Compliant Cryptography (`secp256k1`):** Completely migrated Bitcoin, EVMs, Litecoin, and Tron HD key derivations and transaction signatures from generic P-256 to standard production `secp256k1` curve via `github.com/btcsuite/btcd/btcec/v2`. This prevents any standard incompatibilities with real blockchain networks.
* **Custody 4-Eyes Principle:** Out-bound movements from high-risk custody vaults (Cold Storage, Deep Cold, Treasury) mandate at least 2 independent admin signatures before execution, eliminating rogue single-operator vector risks.
* **Fail-Safe Emergency Freeze:** Immediate system-wide lockdown mechanism halts all custody movements, locking all vault segments and rejecting any transfers instantly upon alert.
* **Treasury elevated validation:** Operational actions on Treasury wallets require at least 2 administrative approvals (registered in `approvalsQueue`).
* **Operational Limits Protection:** Operational wallets maintain strict daily and single-transaction limit bounds configured inside `limits` to mitigate risk.
* **Cryptographic Chained Audits:** Every financial or balance update appends to a SHA-256 cryptographically chained hash log. Any modification of values (such as previous balances or transaction amounts) breaks the chain and triggers alerts.
* **Input Sanitization:** Public REST API endpoints sanitize input parameters and enforce JWT authentication with strict Bearer-token checking.
