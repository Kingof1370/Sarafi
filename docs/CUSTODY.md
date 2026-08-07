# Velyxora Exchange — Institutional Digital Asset Custody Platform

## 1. Overview
The Velyxora Digital Asset Custody Platform is an enterprise-grade custody management sub-system. It is responsible for partitioning institutional and user holdings into segregated secure vaults, enforcing multi-party 4-eyes authorizations on transfers, and providing secure system-wide emergency freezes.

The custody architecture protects assets against the following attack vectors:
* Single admin credential compromise.
* Database field tampering.
* Microservice compromise (by using independent, stateless, in-memory validation rules).

---

## 2. Institutional Vault Segments
Holdings are strictly partitioned into distinct vault roles, each mapped to different operational priority and security tiers:

1. **HOT Vault (`TypeHot` / `v_hot`):** Active pool designed to handle daily deposit and withdrawal velocity.
2. **WARM Vault (`TypeWarm` / `v_warm`):** Semi-liquid buffer to replenish hot balances periodically.
3. **COLD Storage (`TypeCold` / `v_cold`):** Air-gapped secure holding representing 90%+ of total platform holdings. Mandates 4-eyes authorization.
4. **DEEP COLD Vault (`TypeDeepCold` / `v_deep_cold`):** Heavy high-security reserve storage. Requires multi-signature out-of-band validation.
5. **TREASURY Vault (`TypeTreasuryVault` / `v_treasury`):** Platform operational and working capital holdings.
6. **RESERVE Vault (`TypeReserveVault` / `v_reserve`):** Platform insurance and emergency reserve funds.
7. **RECOVERY Vault (`TypeRecoveryVault` / `v_recovery`):** Isolated pool for disaster recovery scenarios.

---

## 3. 4-Eyes Principle & Multi-Party Approvals
For security purposes, transfers from high-risk vaults (such as COLD, DEEP_COLD, and TREASURY) cannot be initiated or completed by a single operator.
* **Cold & Deep Cold Transfers:** Require at least **2** independent administrative approvals.
* **Hot & Warm Transfers:** Require **1** administrative approval.

### Transaction Lifecycle:
1. **REQUESTED:** An operator creates a transfer request from a source vault to a destination vault. Assets remain locked inside the source vault.
2. **APPROVED:** A qualified admin registers a digital signature. Status updates to `APPROVED` if further signatures are required.
3. **COMPLETED:** Once the signature threshold is achieved, the balances are deducted and credited atomically within a database transaction block, updating status to `COMPLETED`.

---

## 4. System-Wide Emergency Freeze
The custody manager exposes a global emergency lockdown flag:
* **FREEZE:** Instantly locks all vaults and suspends all custody transfers. No deposits can be credited, and no withdrawals or transfers can be processed.
* **UNFREEZE:** Restores operations after security audit and validation.
