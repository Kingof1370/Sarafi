# Velyxora Exchange — Enterprise Monitoring & Health Supervisor

## 1. Overview
The Velyxora Enterprise Monitoring and Health Supervisor provides continuous real-time oversight of all microservices, datastores, and message buses inside the exchange. It detects latencies, degradation, and connectivity faults, providing operators with absolute visibility into the platform's health.

---

## 2. Monitored Systems
The monitoring platform tracks health state across 8 critical components:
1. **DATABASE:** Checks PostgreSQL connection pool health and response latencies.
2. **KAFKA:** Monitors message broker connection state and publishing lag.
3. **REDIS:** Monitors session cache memory pools and cluster health.
4. **WALLET_CORE:** Monitors balance segments consistency.
5. **CUSTODY_VAULTS:** Audits secure vault segregation locks and transfers status.
6. **TREASURY_POOLS:** Monitors backing ratio and reserve account status.
7. **API_GATEWAY:** Tracks incoming REST routes latency and error rates.
8. **BLOCKCHAIN_BAL:** Monitors RPC node heights, synch speeds, and circuit breakers.

---

## 3. Auto-Heal Recovery Coordination
Upon detecting component failures (e.g. database degradation or RPC timeouts):
* Emits a `HealthChangedEvent` onto Kafka topic `velyxora-health-changed`.
* Alerts are logged in `alerts`.
* Recovery manager initiates automated healing sequences, restoring component health without manual operator intervention.
