# Velyxora - Health Check Architecture

Velyxora provides decoupled health probes designed for cloud-native orchestration engines (like Kubernetes or Docker Compose).

## Endpoints

### 1. Liveness Probe
- **URL**: `/health/live`
- **Method**: `GET`
- **Purpose**: Verifies that the Go application process is active and running.
- **Dependency Isolation**: Does not depend on external databases (PostgreSQL), memory stores (Redis), or messaging brokers (Kafka), avoiding cascading restarts.
- **Response**: `200 OK`

### 2. Readiness Probe
- **URL**: `/health/ready`
- **Method**: `GET`
- **Purpose**: Validates that all critical dependencies are ready to accept traffic.
- **Dependencies Checked**:
  - PostgreSQL (active pool check)
  - Redis (caching and rate-limiter check)
  - Kafka (cluster connectivity check)
- **Response**: `200 OK` or `503 Service Unavailable`
