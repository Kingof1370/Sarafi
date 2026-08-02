# Velyxora Exchange — Node Management Specifications

## 1. Overview
The Node Manager maintains active pools of nodes (Primary, Secondary, Fallback) to guarantee 100% platform availability.

---

## 2. Health & Scoring Strategy
Each node's `HealthScore` is calculated dynamically on every request ping:
* **Latency Penalization:** Scoring starts at `1.0` and deducts linearly up to `0.0` at pings of 1000ms.
* **Availability Checks:** Offline or tripped status resets the score to `0.0` instantly.
* **Automatic Failover:** When a node trips, the engine routes requests to the next highest scoring node.
