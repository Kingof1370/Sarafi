# Velyxora Structured Logging

Velyxora enforces a strict structured logging format utilizing Go's native high-performance `slog` library.

## Standard Log Fields
Every structured log output contains key context:
- `timestamp`: ISO-8601 standard date and time.
- `service`: Microservice identification (e.g. `api-gateway`, `wallet-service`).
- `component`: Internal subsystem code identification.
- `level`: Log levels (`DEBUG`, `INFO`, `WARN`, `ERROR`).
- `correlation_id` / `request_id`: Tracing identifiers.
- `result`: Execution result (`SUCCESS`, `FAILED`).

## Strict Redaction Policy
Velyxora guarantees that **no sensitive parameters** are written to disk. The logger identifies and redacts:
- passwords
- private keys
- API secrets
- MFA secrets
- refresh tokens
- raw KYC files
- credentials
