# Velyxora Exchange — Real-Time Behavioral Fraud Detection Engine

## 1. Overview
The Real-Time Fraud Detection Engine supervises all transactional ingress and egress on the platform, calculating risk scores for deposits and withdrawals based on environmental, network, and behavioral attributes.

---

## 2. Fraud Evaluation Rules
The engine assesses risk across five separate dimensions:
* **Large Volume Spikes:** Transactions above 10,000.0 USDT are flagged for manual compliance review.
* **Geographical Travel Anomaly:** Calculates distance velocity between subsequent operations. If distance speed is impossible (e.g. traveling 1000km in under 10 minutes), risk is escalated.
* **Network IP Risk Analysis:** Detects requests originating from known blacklisted networks or VPN pools.
* **Device Fingerprint Mismatch:** Tracks changes in browser hashes and device profiles.
* **Account Risk Scoring:** Aggregates all indicators into a final transaction risk score.

---

## 3. Threat Mitigation
If the calculated risk score matches or exceeds **0.70**:
1. The transaction is immediately **FLAGGED**.
2. A `FraudDetectedEvent` is published onto Kafka.
3. Security alerts are created inside the administrative dashboard.
4. Withdrawal operations on the target account are temporarily frozen pending review.
