# Velyxora Exchange — Withdrawal Security Controls

## 1. Overview
The Velyxora Withdrawal subsystem enforces rigorous financial controls to neutralize exit-scams, API key drainage, and fraudulent transfer requests.

---

## 2. Key Security Layers

### 2.1 Velocity Limits & Spending Controls
The engine maps strict thresholds on each supported digital asset:
* **Bitcoin (BTC):** Daily cap of 2.0 BTC, single-transaction limit of 0.5 BTC, cooldown timer of 30 minutes.
* **Ethereum (ETH):** Daily cap of 20.0 ETH, single-transaction limit of 5.0 ETH, cooldown timer of 30 minutes.
* **Stablecoins (USDT / USDC):** Daily cap of 50,000 USDT, single-transaction limit of 10,000 USDT, cooldown timer of 15 minutes.

### 2.2 Address Whitelists & Whitelist Cooldowns
Withdrawal operations are locked exclusively to addresses added and whitelisted in the user's `withdrawal_address_book`.

### 2.3 Administrative Multi-Stage Approval Queue
* **Low-Risk Transfers:** Withdrawals below a risk score threshold require 1 administrative approval inside the approvals table.
* **High-Risk Transfers:** Transfers with custom risk scoring (e.g. low trust scores on recipient, newly whitelisted addresses, or bulk volumes) require at least 2 independent administrative signatures inside `withdrawal_approvals` before they can be broadcasted.
