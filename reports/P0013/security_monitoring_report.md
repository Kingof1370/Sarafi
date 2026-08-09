# Velyxora - SRE Security Observability Report

We have mapped critical security indicators into technical observability metrics:

- **Brute Force Lockouts**: Tracked via `velyxora_security_events_total` with severity `HIGH`.
- **API Key & MFA Failures**: Increments `velyxora_api_key_failures_total` and `velyxora_auth_failures_total` metrics.
- **CSRF & Lockout Violations**: Recorded cleanly inside standard log outputs and durable database audit trails.
