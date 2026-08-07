# Velyxora Exchange — RPC Client Architecture

## 1. Overview
RPC clients are wrapped securely by the standard `CircuitBreaker` utility inside `packages/connectivity`.

---

## 2. Circuit Breaker States
1. **CLOSED:** Normal operating conditions. Errors increment the failure counter.
2. **OPEN:** Tripped state. Error limit reached (3 errors). Immediate failover occurs, blocking further calls to the target node.
3. **HALF-OPEN:** Resets and tests the connection with a probe call after a 5-minute cooldown.
