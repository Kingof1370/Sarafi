# P0018 Security & Cryptographic Auditing

## Status: PASS

## 1. Authentication & Session Protections
* **Argon2/Bcrypt Password Hashing**: Enforces strong passwords and secure hashes.
* **Concurrent Session Limits**: Strictly caps active sessions to 5. Exceeding this automatically revokes the oldest session in PostgreSQL and invalidates it in Redis.
* **Brute-Force Lockouts**: Consistently blocks email addresses and IP addresses for 15 minutes after 5 consecutive login failures.
* **Token Rotation (RTR)**: Refresh token reuse instantly triggers a critical security breach alert and revokes all active sessions for that user.

## 2. Cryptographic API Signatures
* Programmatic API calls require HMAC-SHA256 signature calculations verified using the client's API Secret.
* Prevents replay attacks by checking timestamp clock skews (capped at 300s) and checking nonce hashes in Redis.

## 3. Communication & Server Hardening
* Security headers (HSTS, X-Frame-Options, X-Content-Type-Options, Content-Security-Policy) are strictly applied.
* CORS policies are closed by default in production.
* Request payloads are strictly limited to 10MB to prevent Denial-of-Service attacks.
