# Velyxora - Alerting Engine

The SRE alert manager executes live periodic health check scans and fires alerts when dependencies are unreachable.

## SRE Alert Rules
1. **PostgreSQL Connection Lost**:
   - Condition: Ping failure on PostgreSQL.
   - Severity: `CRITICAL`
   - Action: Register a new incident in `sre_incidents` and a triggered alert in `sre_alerts`.
2. **Redis Connection Lost**:
   - Condition: Ping failure on Redis.
   - Severity: `HIGH`
   - Action: Register a new incident in `sre_incidents` and a triggered alert in `sre_alerts`.
3. **Kafka Connection Lost**:
   - Condition: Dial failure on Kafka bootstrap brokers.
   - Action: Register a new incident in `sre_incidents` and a triggered alert in `sre_alerts`.

## Self-Healing / Auto-Mitigation
When dependency health probes return to `UP`, the alerting manager automatically marks open incidents as `RESOLVED` and records the resolution timestamp.
