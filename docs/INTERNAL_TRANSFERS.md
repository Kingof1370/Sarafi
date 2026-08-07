# Velyxora Exchange — Audited Internal Asset Transfers

## 1. Overview
This document specifies the allowed transaction pathways and approval thresholds governing internal movements of exchange and custody assets across Velyxora's institutional capital pools.

---

## 2. Institutional Transfer Matrix
Asset transfers are strictly restricted to the following authorized paths:

| Source Pool | Destination Pool | Allowed | Approvals Required | Risk Profile |
| :--- | :--- | :--- | :--- | :--- |
| **Treasury** | **Hot** | Yes | 1 | Medium |
| **Hot** | **Treasury** | Yes | 1 | Medium |
| **Warm** | **Treasury** | Yes | 1 | Medium |
| **Cold** | **Treasury** | Yes | 2 | High |
| **Treasury** | **Reserve** | Yes | 1 | Medium |
| **Reserve** | **Treasury** | Yes | 2 | High |
| **Treasury** | **Fee Wallet** | Yes | 1 | Low |
| **Fee Wallet** | **Treasury** | Yes | 1 | Low |

All other pathways are rejected immediately at the domain layer to prevent malicious or accidental capital leaks.

---

## 3. Risk & Dual Approvals (4-Eyes Principle)
* **Threshold Rules:** Transfers originating from high-security pools (**Cold** or **Reserve**) or representing high transaction volume (amount >= 1000.0) mandate at least **2** independent administrative signatures.
* **Audit Trails:** Every internal transfer registers a permanent record inside `treasury_transfers`, logging all signatures, operator details, timestamps, and previous/new balance states atomically.
