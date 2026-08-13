# Prompt: Production Security Hardening & MFA Enrollment (PART-06)

## Objective
Remediet high-priority security vulnerabilities and non-production mock flows in the authentication and configurations layer. Specifically:
1. Eliminate JWT default fallback secrets.
2. Eliminate mock database credentials and hardcoded credentials fallbacks.
3. Replace the static dummy MFA enrollment endpoints (`JBSWY3DPEHPK3PXP`) with a fully cryptographic, dynamic TOTP provisioning system.
4. Enforce strict **Fail-Closed** states on all missing configuration secrets in production environments.

## Scope
- Modify `apps/backend/main.go` and authentication files.
- Modify `packages/security/mfa.go`, `packages/security/config.go` (if exists), or `packages/security/security.go`.
- Audit JWT secret, DB connection secrets, and brute force lockout policies.

## Requirements & Specifications

### 1. Absolute Fail-Closed Secret Loading
- In production (`APP_ENV=production`), if `JWT_SECRET`, `DB_PASSWORD`, `WALLET_SEED`, or any critical API configuration is missing or matches development defaults (like `"super-secret-velyxora-key-999"`), the application must panic or call `os.Exit(1)` immediately during startup.
- Completely dismantle any "test mock user" or "mock password login" fallbacks (like `usr_mock_123` or `test@velyxora.com` fallback bypass) within production execution paths.

### 2. Cryptographic MFA Provisioning
- Completely rewrite `/api/v1/mfa/enable`.
- For each request, dynamically generate a cryptographically random, high-entropy 32-character Base32 encoded key (the secret seed).
- Generate a proper TOTP provisioning URL format (`otpauth://totp/Velyxora:<user_email>?secret=<secret_seed>&issuer=Velyxora`).
- Generate unique, secure, and single-use cryptographic backup codes.
- Persist the secret key and hashed backup codes into the database for the requesting user, marked as `PENDING_VERIFICATION` until the user submits a valid TOTP code to confirm enrollment.

### 3. Rate Limiting and Session Management
- Ensure rate limiters do not possess static, unauthenticated bypass modes.
- Strengthen brute force lockout policies, persisting login failure counters in Redis or DB.

## Testing Strategy
- Create security and authentication tests:
  - Test startup behavior: Set `APP_ENV=production` and assert that launching the service without setting `JWT_SECRET` fails closed instantly.
  - Test TOTP provisioning: Call `/api/v1/mfa/enable` twice for different users. Assert that the secrets generated are unique, random, and valid TOTP URLs.
  - Verify that unconfirmed MFA secrets are not used to validate login, and once verified, old login paths require MFA validation.
