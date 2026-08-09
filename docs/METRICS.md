# Velyxora - Metrics Specification

The following Prometheus metrics are exported by Velyxora Observability Manager:

## SRE Technical Metrics
- `velyxora_http_requests_total` [Counter] (Labels: `method`, `path`, `status`)
- `velyxora_http_errors_total` [Counter] (Labels: `method`, `path`, `status_class`)
- `velyxora_http_latency_seconds` [Histogram] (Labels: `method`, `path`)
- `velyxora_auth_failures_total` [Counter] (Labels: `type`)
- `velyxora_rate_limit_events_total` [Counter] (Labels: `route`)
- `velyxora_api_key_failures_total` [Counter] (Labels: `reason`)
- `velyxora_ws_connections_active` [Gauge]
- `velyxora_ws_connections_total` [Counter] (Labels: `event`)
- `velyxora_kafka_producer_failures_total` [Counter] (Labels: `topic`)
- `velyxora_kafka_consumer_errors_total` [Counter] (Labels: `topic`, `group_id`)
- `velyxora_kafka_consumer_lag_bytes` [Gauge] (Labels: `topic`, `group_id`)
- `velyxora_redis_failures_total` [Counter] (Labels: `operation`)
- `velyxora_postgres_query_latency_seconds` [Histogram] (Labels: `query_type`)
- `velyxora_postgres_errors_total` [Counter] (Labels: `query_type`, `error_category`)
- `velyxora_matching_latency_seconds` [Histogram] (Labels: `stage`, `symbol`)
- `velyxora_blockchain_rpc_failures_total` [Counter] (Labels: `network`)
- `velyxora_blockchain_confirmation_latency_seconds` [Histogram] (Labels: `network`)
- `velyxora_compliance_alerts_total` [Counter] (Labels: `rule_triggered`, `severity`)
- `velyxora_security_incidents_total` [Counter] (Labels: `category`, `severity`)

## Business Metrics
- `velyxora_orders_submitted_total` [Counter] (Labels: `symbol`, `side`, `type`)
- `velyxora_orders_accepted_total` [Counter] (Labels: `symbol`, `side`)
- `velyxora_orders_rejected_total` [Counter] (Labels: `symbol`, `reason`)
- `velyxora_orders_matched_total` [Counter] (Labels: `symbol`)
- `velyxora_orders_cancelled_total` [Counter] (Labels: `symbol`, `reason`)
- `velyxora_trading_volume_units_total` [Counter] (Labels: `symbol`)
- `velyxora_trade_count_total` [Counter] (Labels: `symbol`)
- `velyxora_active_symbols` [Gauge] (Labels: `symbol`)
- `velyxora_active_users_count` [Gauge]
- `velyxora_wallet_operations_total` [Counter] (Labels: `operation`, `asset`)
- `velyxora_deposit_status_total` [Counter] (Labels: `asset`, `status`)
- `velyxora_withdrawal_status_total` [Counter] (Labels: `asset`, `status`)
- `velyxora_aml_alerts_total` [Counter] (Labels: `rule_triggered`)
- `velyxora_kyc_status_changes_total` [Counter] (Labels: `tier`, `status`)
- `velyxora_security_events_total` [Counter] (Labels: `event_type`, `severity`)
