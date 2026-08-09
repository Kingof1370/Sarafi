# Velyxora - SRE Failure Injection Report

Conducted test-driven failure injection checks to verify SRE monitoring, logging, and self-healing.

## Scenarios Checked
1. **PostgreSQL Down**: Simulated database unreachable. Liveness probe stayed `UP`, readiness probe switched to `DOWN` with status `503 Service Unavailable`, and a critical SRE incident was generated.
2. **Redis Down**: Rate-limiter fell back to open state securely, readiness probe went `DOWN` with `503`, and Redis Offline incident opened.
3. **Kafka Down**: Kafka down incident registered inside PostgreSQL and readiness probe returned status `503`.
4. **Auto-Mitigation**: REST API successfully transitioned active incidents and alerts to `RESOLVED` when dependencies returned to `UP` state.
