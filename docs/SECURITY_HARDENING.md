# Velyxora Security Hardening Guide

## Security Configurations & Policies

### 1. API Gateway Hardening
- **Secure Security Headers**:
  - `Strict-Transport-Security` (HSTS): enforces HTTPS.
  - `X-Frame-Options: DENY`: blocks Clickjacking attacks.
  - `X-Content-Type-Options: nosniff`: blocks MIME-sniffing.
  - `Content-Security-Policy`: restricts resource loading to trusted domains.
- **Strict Payload Limits**: Enforced 10MB payload size limits in middlewares.
- **Secure Timeouts**: Configured aggressive Read, Write, and Idle TCP connection timeouts.

### 2. Session Security
- **Strict Concurrency Limits**: Users are strictly limited to 5 concurrent active sessions. If a 6th session is initiated, the oldest active session is automatically terminated and cached in Redis.
- **Refresh Token Rotation (RTR)**: Refresh tokens are rotated on every renew request. Reusing a revoked or old refresh token triggers an immediate cascade revocation of all active sessions for that user to prevent session hijacking.

### 3. MFA/TOTP Hardening
- **AES-GCM-256 Secret Encryption**: TOTP secrets are encrypted before persistence in Postgres using AES-GCM envelope encryption.
- **Brute-force Lockout**: 5 consecutive auth or MFA failures trigger a temporary 15-minute lockout on the account and IP address.

### 4. Database Security
- **No Direct Raw Queries**: All database queries are fully parameterized, completely blocking SQL Injection vectors.
- **Row-Level SELECT FOR UPDATE Locks**: Mutating financial operations enforce row-level locks on balance balances to prevent concurrent race conditions.

### 5. Docker Containers
- **Non-Privileged Run**: Base Go microservice Dockerfiles are hardened to run under dedicated non-root users (`velyxuser` and `velyxgroup`).
- **Minimal Footprint**: Services compile on top of multi-stage minimal scratch/alpine environments.
