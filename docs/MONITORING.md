# Velyxora - Monitoring Guide

This manual describes how to monitor Velyxora services in production environments.

## Dashboard & Scraping
The Prometheus metrics server is exposed under the standard endpoint `/metrics` in the API Gateway on port `8080`.

To scrape Velyxora, add the following target definition to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'velyxora'
    scrape_interval: 5s
    static_configs:
      - targets: ['localhost:8080']
```

## Key SRE Panels
- **System Latency**: `velyxora_http_latency_seconds`
- **Active Connections**: `velyxora_ws_connections_active`
- **Dependency Failures**: `velyxora_redis_failures_total` and `velyxora_postgres_errors_total`
- **Order Processing Rate**: `velyxora_orders_submitted_total` vs `velyxora_orders_accepted_total`
