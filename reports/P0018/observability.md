# P0018 Observability & SRE Metrics

## Status: PASS

## 1. Metrics & Logs
* **Structured Logging**: Contextual structured JSON logs are managed by `packages/logger/logger.go`. Sensitive parameters (passwords, tokens, keys) are automatically redacted prior to log output.
* **Trace Propagation**: OpenTelemetry-style traces propagate across service boundaries using custom Kafka header MapCarriers.
* **Metrics Endpoint**: `/metrics` exports live Prometheus-compatible performance counters (HTTP request counts, DB latency, engine matches).

## 2. Live Incident Integration
* An active background SRE alerting manager periodically monitors core infrastructure health.
* If Postgres, Redis, or Kafka becomes unreachable, an SRE Incident is automatically created in PostgreSQL and a high-priority warning is printed. Once connection is restored, the SRE alerting thread marks the incident as `RESOLVED`.
