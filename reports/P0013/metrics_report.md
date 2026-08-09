# Velyxora - SRE Metrics Integration Report

We have integrated standard Prometheus-style technical and business metrics inside our shared observability module.

## Registered Indicators
- **HTTP Total Requests**: `velyxora_http_requests_total`
- **HTTP Error Rate**: `velyxora_http_errors_total`
- **HTTP Percentile Latencies**: `velyxora_http_latency_seconds`
- **WS Session Counts**: `velyxora_ws_connections_active`
- **Trade Volume**: `velyxora_trading_volume_units_total`
- **Postgres Failures & Query Time**: `velyxora_postgres_query_latency_seconds` and `velyxora_postgres_errors_total`
- **Redis Failures**: `velyxora_redis_failures_total`
- **Kafka Failures**: `velyxora_kafka_producer_failures_total` and `velyxora_kafka_consumer_errors_total`

Every single metric registered is real-time and updated dynamically from the core execution flows of our microservices.
