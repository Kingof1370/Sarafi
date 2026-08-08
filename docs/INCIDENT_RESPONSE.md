# Velyxora Incident Management & Response

Velyxora features an automated and manual incident response system allowing officers to flag, track, and resolve production critical issues.

## Lifecycle States
1. **OPEN**: Incident created by monitoring system or operator.
2. **ACKNOWLEDGED**: Incident assigned to an engineer.
3. **INVESTIGATING**: Diagnostics underway.
4. **MITIGATED**: System stabilized.
5. **RESOLVED**: Resolution notes updated, status changed to RESOLVED.
6. **CLOSED**: Archived.

## Incident REST API
Protected with `system:admin` fine-grained RBAC permission:
- `POST /api/v1/incidents`: Log a new incident.
- `GET /api/v1/incidents`: List all active incident boards.
- `PATCH /api/v1/incidents/:id`: Transition statuses and attach notes.
