# Velyxora - SRE Health Probe Report

We have integrated and verified decoupled health endpoints inside our REST API server.

## Probes Behavior
- **Liveness Probes (`/health/live`)**: Responds with `200 OK` in under `1ms` and has no external database dependencies.
- **Readiness Probes (`/health/ready`)**: Performs concurrent pings to PostgreSQL and Redis, and dials Kafka bootstrap broker with a strict `2s` timeout. Returns `503 Service Unavailable` if any dependency is offline.
