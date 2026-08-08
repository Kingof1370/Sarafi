# Velyxora Performance Metrics

Velyxora implements a thread-safe, low-cardinality metrics dashboard system under `packages/common/observability.go`.

## Key Metrics List
- `api_requests_total`: Counter tracking total endpoint visits by path and response status code.
- `api_latency_ms`: Histogram detailing request-response latencies.
- `authentication_failures_total`: Incremented on failed authentication attempts.
- `mfa_brute_force_lockouts_total`: Track lockouts triggered by brute force protections.
- `orders_accepted_total`: Incremented on successful order submission.
- `settlement_latency_ms`: Time taken to execute ledger settlement.

## Prometheus Integration
All metrics are formatted as standard Prometheus notation and exposed on `/metrics`.
