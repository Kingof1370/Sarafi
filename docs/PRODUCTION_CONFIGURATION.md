# Production Configuration Reference
## Velyxora Enterprise Exchange

Reference sheet for all production configurations and fail-closed secrets.

| Environment Variable | Description | Security Requirements | Fail-Closed Policy |
|----------------------|-------------|-----------------------|--------------------|
| `APP_ENV` | Environment Type | Must be set to `production` | Strict Enforcement |
| `JWT_SECRET` | Token Signatures Secret | Must be >= 32 chars, non-default | System halts if weak/missing |
| `API_KEY_MASTER_SECRET`| Master API Key GCM Key | Must be >= 32 chars, non-default | System halts if weak/missing |
| `BACKUP_ENCRYPTION_KEY`| Backup Archive Encryption | Must be >= 32 chars, non-default | System halts if weak/missing |
| `DB_PASSWORD` | PostgreSQL Database Pass | Strong password, non-default | System halts if weak/missing |
| `LOG_LEVEL` | Log Verbosity level | Must not be set to `DEBUG` | System halts if set to `DEBUG` |
| `DB_SSLMODE` | SSL/TLS DB Connection | Must be set to `require` for remote | System halts if `disable` |
