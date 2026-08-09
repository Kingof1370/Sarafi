# Velyxora Threat Model

## Threat Modeling Methodology
Velyxora applies STRIDE (Spoofing, Tampering, Repudiation, Information Disclosure, Denial of Service, Elevation of Privilege) methodology to identify threats and enforce strict mitigations.

## STRIDE Threat Analysis

### 1. Spoofing
- **Threat**: Attackers spoofing identity or sessions.
- **Mitigation**: Standard cryptographically-signed JSON Web Tokens (JWT) for interactive user sessions, and programmatic HMAC-SHA256 request signatures for API integrations. Nonce registry in Redis blocks replay spoofing.

### 2. Tampering
- **Threat**: Tampering with request payloads (e.g. order price, withdrawal destination).
- **Mitigation**: Comprehensive server-side validation. API signatures enforce integrity of path, query, and request body. Double-entry accounting ensures ledger data is mathematically balanced and verifiable.

### 3. Repudiation
- **Threat**: Users denying executing trades or withdrawals.
- **Mitigation**: Soc2-compliant administrative and security audit logging persists immutable events to PostgreSQL (`security_audit_logs`).

### 4. Information Disclosure
- **Threat**: Sensitive cryptographic keys, passwords, or transaction records leaking in telemetry or error logs.
- **Mitigation**: Upgraded logger with automatic redaction rules on sensitive patterns. Restricting CORS origins and disabling debug flags in production mode.

### 5. Denial of Service (DoS)
- **Threat**: High-frequency payload attacks flooding the matching engine or API gateway.
- **Mitigation**: Enforces strict Redis-backed sliding-window rate limiters. Enforced max 10MB payload size limit on REST routes.

### 6. Elevation of Privilege
- **Threat**: Regular users calling administrative/support actions, or Support Agents triggering asset withdrawals.
- **Mitigation**: Server-side fine-grained RBAC mapping in PostgreSQL. Every protected REST endpoint enforces strict role permission rules.

## Data Flow Diagrams & Boundaries
- **Untrusted Zone**: Public internet / Web Client / Programmatic Integrations.
- **DMZ / Perimeter**: API Gateway enforcing timeouts, CORS, rate limits, JWT/HMAC verification, and RBAC controls.
- **Trusted Microservices Zone**: Matching Engine and Wallet Service communicating via secure, isolated Kafka topic boundaries.
- **Storage Layer**: Sharded Postgres DB and hot memory Redis.
