# VELYXORA EXCHANGE - ADMINISTRATIVE OPERATIONS
Platform operational playbooks:
1. **Global Trading Halts:** Pause matching engine pipelines during anomalies.
2. **Account Blocklists:** Revoke tarrif and trading permissions of flagged accounts.
3. **Observability Telemetry:** Query standard liveness health check state at `GET /health` or system metrics at `GET /api/v1/system/metrics`.
4. **Incident response:** When alerts fire, operators must create an incident via `POST /api/v1/incidents` and assign engineers to investigate.
5. **Auditing:** Fetch real-time compliance audit trails from `GET /api/v1/audit/events` to verify system integrity.
