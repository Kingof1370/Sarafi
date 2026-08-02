# Velyxora Exchange — Enterprise Withdrawal Engine

## 1. Purpose & Scope
The Withdrawal Engine manages all outbound cryptocurrency transactions on the Velyxora Exchange. It establishes strict security checks (velocity limits, whitelists, administrative multi-stage approvals) to protect corporate and user reserves.

---

## 2. Core Components & Architecture
```
+------------------------------------------------------------+
|                      Withdrawal Engine                     |
|   +-----------------------+-----------------------------+  |
|   | Withdrawal Processor  |  Initiates & cancels sessions|  |
|   +-----------------------+-----------------------------+  |
|   | Withdrawal Validator  |  Checks whitelists & limits |  |
|   +-----------------------+-----------------------------+  |
|   | Address Book Registry |  Verified recipient addresses|  |
|   +-----------------------+-----------------------------+  |
|   | Executor & Scheduler  |  Signs and broadcasts txs   |  |
|   +-----------------------+-----------------------------+  |
+------------------------------------------------------------+
```

* **Withdrawal Processor:** Initiates, tracks, and manages withdrawal states.
* **Withdrawal Validator:** Enforces limits (per user, per asset, cumulative 24h limits, single limits), whitelist checks, and active cooldown boundaries.
* **Address Book & Whitelists:** Registers whitelisted addresses verified by the user.
* **Executor & Scheduler:** Cryptographically builds and signs transactions using blockchain adapters, broadcasting to networks and logging receipts in database historical tables.
* **Auditing & Events:** Chains SHA-256 historical states and emits Kafka event messages.

---

## 3. Transaction Broadcast & Retry Layer
* **Transaction Building:** Decoupled adapters build raw payloads on curves (secp256k1/Ed25519) matching the target blockchain.
* **Status Tracking:** Progresses states from `REQUESTED` to `VALIDATED`, `APPROVED`, `BROADCAST`, `CONFIRMED`, and `COMPLETED`.
* **Retry Loop:** Schedulers automatically retry broadcasts in the background on connection drops or fee bumps, logging every receipt inside `withdrawal_broadcast_history`.
