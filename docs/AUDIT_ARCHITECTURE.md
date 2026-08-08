# Velyxora Audit Architecture

To adhere to SOC2 and ISO-27001 corporate standards, Velyxora manages high-integrity security audit logs.

## Audit Coverage
The system tracks and audits critical actions:
- Authentications (Login, Logout, brute force protections).
- MFA enrollments, modifications, and TOTP checks.
- API key creation, scopes modification, and revocation.
- Administrative interventions (emergency freeze, trading halts, blocking users).

## Security Chaining
Logs are securely persisted in the `security_audit_logs` table, indexed by User ID and Event Type, and emitted concurrently over Kafka to long-term cold storage.
