# SECURITY HARDENING ARCHITECTURE

This document outlines the Enterprise Security Hardening and Infrastructure Controls implemented in Velyxora to satisfy strict compliance standard frameworks (SOC2, ISO 27001).

## 1. CORS & Origin Enforcement
- Unsafe wildcard CORS defaults are rejected in production mode.
- Allowed origins must be specified via `CORS_ALLOWED_ORIGINS` environment variables. If empty or wildcarded in a production environment, requests fail closed.

## 2. Secure HTTP Headers
- **Strict-Transport-Security (HSTS)**: Enforced via `max-age=31536000; includeSubDomains; preload` header to prevent SSL-stripping attacks.
- **X-Frame-Options**: Set to `DENY` to protect against clickjacking attempts.
- **X-Content-Type-Options**: Set to `nosniff` to block mime sniffing.
- **Content-Security-Policy**: Configured with strict content routing directives.

## 3. Denial of Service (DoS) Protections
- **Payload Limits**: Implemented maximum HTTP body limits (10MB) via `MaxBytesReader` middleware on all endpoints.
- **Connection Timeouts**: Hardened http server configs with explicit timeouts:
  - `ReadTimeout`: 15 seconds
  - `WriteTimeout`: 15 seconds
  - `IdleTimeout`: 60 seconds

## 4. Container & Infrastructure Hardening
- **Non-Root Containers**: Docker runtimes create and switch to a secure, limited `velyxuser` to execute binary targets.
- **Tightened Permissions**: File systems operations use `0600` owner-only read-write permissions on sequence logs and backups.
