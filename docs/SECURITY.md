# Velyxora Exchange — Cryptographic & Financial Security Architecture

## 1. Overview
Financial safety is the foundation of the Velyxora Exchange. The system enforces strict administrative permissions, deterministic hierarchical address generation, cryptographic ledger chaining, and cold wallet air-gapping to defend the pipeline against any compromise.

---

## 2. Key Elements

### 2.1 Hierarchical Deterministic Key Security
Deposit addresses are generated using a BIP-39 mnemonic phrase and a BIP-32/BIP-44 HD structure:
* Derivation seed is strictly computed using isolated, secure standard crypto hashing techniques.
* Public keys are derived on elliptic curves (secp256k1/P-256 for EVM/BTC, Ed25519 for Solana).
* Private keys are never persisted in plain-text inside the database.

### 2.2 Balance Safeguards & Isolation
* **Segment Validation:** `ValidateWallet` verifies that for every asset: `Available + Locked + Reserved + Pending = Total`. Any discrepancy locks the wallet.
* **Negative State Violation Blockers:** All balance delta calculations throw errors if any segment would drop below zero.

### 2.3 Wallet-Type Permissions
* **Cold Wallet Isolation:** Direct programmatic withdrawals from Cold Wallets are impossible. All cold wallet operations are air-gapped and require physical multi-signature sign-off.
* **Treasury elevated checks:** Operative actions on Treasury Wallets require at least 2 administrative approvals inside the queue before execution.
* **Operational limits:** Limits are configured with single-transaction and cumulative 24-hour caps to prevent rapid drainage.

---

## 3. Cryptographic Ledger Auditing
Every ledger update creates a chained hash record:
```
Hash(n) = SHA256( ID | UserID | WalletID | Asset | Action | Amount | PrevBalance | NewBalance | Message | Hash(n-1) | Timestamp )
```
Any attempt to insert, delete, or modify rows breaks the hash chain, triggering immediate service halts.
