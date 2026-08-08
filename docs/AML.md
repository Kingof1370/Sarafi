# Anti-Money Laundering (AML) Rules Configuration

Velyxora monitors 100% of user-driven events using a rule-based Anti-Money Laundering engine.

## Core Rules & Detection Indicators
The aml monitors:
1. **Unusual Transaction Amount**: Triggers alerts when any single deposit or withdrawal transaction exceeds configurable threshold values (e.g. $5,000 USD/USDT).
2. **Velocity Anomalies**: Monitors transaction frequencies. Over 5 transactions in a single 1-hour window automatically triggers a `SeverityCritical` alert and places the user under monitoring restriction status.
3. **Structuring Indicators**: Checks for consecutive transfers just under reporting thresholds.
4. **Suspicious Address Interaction**: Screens addresses dynamically.

## Rule Auditing & Persistence
Every aml rule execution is logged and explains exactly why it was triggered:
- Relational mapping between transaction IDs and triggered aml alert records.
- Risk scoring levels are populated and linked directly.
- Triggers are broadcast onto dedicated Kafka event topics (`aml.alert.created`) for downstream consumption.
