# Velyxora Exchange — Digital Asset Security & Emergency Controls

## 1. Overview
The security of digital assets inside the Velyxora platform is maintained by a multi-layered verification and active containment system. This system acts as a protective shield against anomalous asset transfers, malicious administrators, and system exploits.

---

## 2. Emergency Freeze Containment
The platform features an active containment protocol:
* **Halt Level:** Global emergency freeze locks all vaults and transactions.
* **Trigger:** Can be activated manually by authorized security operators or automatically by the anomaly detection engines (e.g., detecting wash trading, rapid-fire withdrawals, or ledger hash mismatches).
* **Isolation:** Rejects any outbound asset transfers immediately, preserving capital while audits are executed.

---

## 3. Cryptographic Ledger Hash Audits
To prevent direct database manipulation:
* Balance states are mirrored to cryptographically chained hash logs in `wallet_audits`.
* Every entry maintains a SHA-256 hash incorporating the previous entry's hash, forming an audit chain.
* Regular background audit workers verify the chain's integrity. Any mismatch triggers an automated global freeze.

---

## 4. Spending and Operational Limits
Every hot and operational wallet maintains configured transaction limits:
* **Single Transaction Limit:** Maximum asset amount per transaction.
* **Daily Cumulative Limit:** Maximum spending velocity in a 24-hour window.
* **Velocity Checks:** Enforced at the domain level inside the `WalletManager` before any transfer is queued.
