# Prompt: P08 — Security & Production Hardening

## 1. Objective
Harden API authentication parameters, eliminate development default fallbacks, and implement secure, cryptographic TOTP enrollment provisioning.

## 2. Current Problem
- JWT secrets fell back to development strings.
- MFA enable returned hardcoded secret tokens.
- Mock credentials login bypass existed.

## 3. Root Cause
Insecure development configuration fallbacks inside main production paths.

## 4. Affected Modules
- `apps/backend/`
- `packages/security/`

## 5. Affected Files
- `apps/backend/main.go`
- `apps/backend/oms_handlers.go`
- `packages/security/mfa.go`
- `apps/backend/production_hardening_test.go`

## 6. Architectural & Technical Requirements
- In production, missing secrets must trigger a strict Fail-Closed startup crash loop.
- Generate dynamic, high-entropy 32-character Base32 secrets and secure backup codes on enrollment.

## 7. Migration & Backward Compatibility
- Backwards compatible with standard JWT claims validation engines.

## 8. Security & Financial Requirements
- Save secrets inside Postgres users table marked as verification-required prior to active verification.

## 9. Tests & Failure Scenarios
- Verify startup behavior: Set `APP_ENV=production` and assert launching without setting `JWT_SECRET` crashes loop.
- Verify unique, random TOTP URL formats.

## 10. Acceptance Criteria & Definition of Done
- Development bypasses dismantled in production mode.
- Cryptographic TOTP keys provisioning is verified.
- Completed with Git commit.
