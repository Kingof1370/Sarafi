# Velyxora Enterprise Observability

Velyxora implements a robust, high-performance central observability layer spanning all mission-critical microservices:
- **API Gateway**: Structured request correlation, latency histograms, error-rate logs, and rate-limiting metrics.
- **Matching Engine**: Nano-second matching throughput tracking, order book sequence triggers, execution logs.
- **Wallet Service**: Multi-dimensionalAvailable/Reserved balances checks, five-layered double-entry ledger audits.
- **Security & Compliance Modules**: Brute-force block lockouts, TOTP validations, Travel Rule audits.

## Key Features
1. **Central Observability Architecture**: Shared core telemetry package `packages/common/observability.go` managing atomic gauges, counters, and histograms.
2. **Context-Aware Structured Logging**: Production-ready structured logger wrapped with `slog` to securely filter sensitive items.
3. **Correlation ID Propagation**: Trace IDs passed downstream via REST request contexts and Kafka message metadata headers.
4. **Disaster Recovery & Incidents life-cycle**: Live administration endpoints to raise, triage, track, and resolve system alerts.
