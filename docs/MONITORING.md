# Velyxora Platform Monitoring

Velyxora provides automated platform monitoring, real-time alerts trigger loops, and administrative consoles.

## Metrics Coverage
- **API Request Count & Latency**: Quantiles tracking performance profiles.
- **Authentication & MFA Failures**: Incremented on unsuccessful login or credential verification actions.
- **Settlement Latency**: Computes time taken to settle base/quote ledger double-entry transfers in milliseconds.
- **Kafka / DB / Redis Latencies**: Gauges database query speeds and Kafka delivery failures.

## Operational Alert Rules
The alerting rules cover:
1. **Critical Database Unavailable**: Instantly alerts on PG connection timeouts.
2. **High API Error Rates**: Triggers on 5xx status spikes.
3. **MFA Brute-force Lockouts**: Registers alerts on temporary client blocks.
