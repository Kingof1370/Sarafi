# Velyxora - SRE Incident Monitoring

Velyxora manages and tracks technical incident workflows using the relational database ledger.

## Workflow Statuses
- `OPEN`: Incident is active and currently being investigated.
- `RESOLVED`: Incident condition has cleared up and dependency is restored.

## DB Schema Relations
- **`sre_incidents`**: Houses primary incident records (severity, title, description, status, impacted service, timestamps).
- **`sre_alerts`**: Tracks specific metric alarms or error codes connected to the open incident.
