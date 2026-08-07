# Velyxora Exchange — Enterprise Blockchain Connectivity

## 1. Purpose & Scope
The Blockchain Connectivity subsystem coordinates standard, reliable, and fault-tolerant communication with raw blockchain nodes across supported networks.

---

## 2. Core Components & Architecture
```
+------------------------------------------------------------+
|                    Connectivity Layer                      |
|   +-----------------------+-----------------------------+  |
|   | Node Manager          |  Tracks latencies & scores  |  |
|   +-----------------------+-----------------------------+  |
|   | Circuit Breaker       |  Guards and trips faulty RPC|  |
|   +-----------------------+-----------------------------+  |
|   | Block Tracker         |  Tracks height & reorgs     |  |
|   +-----------------------+-----------------------------+  |
+------------------------------------------------------------+
```

* **Node Manager:** Coordinates primary, secondary, and fallback pools, evaluating health scores based on latencies.
* **Circuit Breaker:** Wraps client request operations, transitioning to `OPEN` state upon reaching error thresholds.
* **Block Tracker:** Ingresses headers to track blocks, caching hashes and identifying chain reorganizations.

---

## 3. Supported Networks
* **Bitcoin**
* **Ethereum**
* **BNB Smart Chain**
* **Polygon**
* **Solana**
* **Avalanche**
* **Tron**
* **Litecoin**
