# Velyxora - Structured Logging Guidelines

Production microservices in the Velyxora ecosystem utilize slog-based structured JSON logs outputted to stdout.

## Log Entry Schema
Each JSON log entry records the following contextual SRE fields:

- `time`: RFC3339 formatted timestamp
- `level`: `INFO`, `WARN`, `ERROR`, or `DEBUG`
- `service`: Microservice name (e.g. `api-gateway`, `matching-engine`, `wallet-service`)
- `trace_id`: Correlation identifier propagated across the service boundary
- `msg`: Context message of the logged event

## Secret Redaction Policy
To prevent credentials leakage, the structured logger strictly monitors fields containing:
- JWTs
- Passwords
- API Secrets / Hashes
- Private Keys
- Seed phrases / Mnemonics
- MFA secrets

If detected, value is replaced with `[REDACTED]` prior to serialization.
