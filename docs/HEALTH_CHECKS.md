# Velyxora Dependency Health Checks

Velyxora monitors liveness and readiness states to ensure high-availability of critical services.

## Status Classifications
- **LIVE**: System is running but has not completed bootstrap.
- **READY**: System is fully running and all mandatory dependencies (PostgreSQL, Redis) are online.
- **DEGRADED**: Optional integrations (Kafka, Blockchain simulators) are offline, but system is still accepting requests.
- **NOT_READY**: Mandatory dependency is down. Services will return a `503 Service Unavailable` status.

## Probes Endpoints
- `GET /health`: Detailed status payload.
- `GET /readiness`: High-availability kubernetes-compatible readiness probe.
