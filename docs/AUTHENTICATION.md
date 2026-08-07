# Velyxora Exchange — User Authentication & Session Flows

## 1. Overview
The Velyxora Platform supports secure REST and WebSocket communication workflows. User logins are authenticated using standard JSON Web Tokens (JWT) with sliding-window expiration controls.

---

## 2. Authentication Lifecycle

### 2.1 User Registration
* **Endpoint**: `POST /api/v1/auth/register`
* Enforces strict, multi-character, special-symbol secure password strength requirements via standard BCrypt hashing.

### 2.2 Standard Login Flow
When MFA is NOT active:
1. Client POSTs credentials to `POST /api/v1/auth/login`.
2. Server validates password hash against PostgreSQL database.
3. Server returns Access and Refresh tokens immediately:
   * **Access Token**: Expires in 15 minutes.
   * **Refresh Token**: Expires in 24 hours.

### 2.3 MFA-Secured Login Flow
When MFA IS active:
1. Client POSTs credentials to `POST /api/v1/auth/login`.
2. Server validates password and checks if `is_mfa_enabled = true`.
3. If true, server halts login sequence, generates a short-lived `mfa_token` (5-minute expiration) and registers it in `user_mfa_sessions` table.
4. Server returns challenge response:
   ```json
   {
     "mfa_required": true,
     "mfa_token": "mfa_sec_xxxx"
   }
   ```
5. Client renders Verification Code challenge panel and prompts user for 6-digit code.
6. Client POSTs verification payload to `POST /api/v1/auth/mfa-verify`.
7. Server validates TOTP or active backup code, cleans up the session, and returns final access/refresh tokens.
