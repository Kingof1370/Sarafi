# P0010 Platform Metrics Report

Velyxora monitors standard low-cardinality counters, gauges, and histograms.

## Instrumentation
- **Uptime & Resources**: Memory allocation, CPU count, and goroutines count.
- **API Traffic**: Latency histograms, request rates, error rates by status code and path.
- **Exchange Performance**: Settlement latency, orders accepted/rejected/cancelled, matching speeds.
- **Failures**: DB, Redis, and Kafka failures are recorded atomically to trigger operators' dashboards.
