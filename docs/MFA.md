# Velyxora Exchange — Multi-Factor Authentication (MFA) & Strong Authentication

## 1. Overview
Multi-Factor Authentication (MFA) is a mandatory, core cryptographic protective layer for Velyxora corporate user registration and login workflows. Under **P0003**, a standards-compliant RFC 6238 Time-Based One-Time Password (TOTP) engine was designed, verified, and fully integrated with standard API Gateway routes, secure persistent database structures, and Next.js frontends.

---

## 2. Technical Scope & Specifications

### 2.1 Cryptographic Key Setup (TOTP)
* **Secret Generation**: Secure random Base32 strings (160 bits entropy) generated using high-strength random generation blocks.
* **Secret Encryption**: Stored in PostgreSQL `users` table as highly-secure AES-GCM-256 ciphertexts, utilizing SHA-256 hashed JWT secrets as keys. Raw plaintext secrets are never persisted.
* **TOTP URI & QR Codes**: Outputs standard `otpauth://totp/Velyxora:{Email}?secret={Secret}&issuer=Velyxora` URIs, rendered visually inside the dashboard setup wizard.

### 2.2 Replay Protection
Each validated token is cached in a thread-safe global cache for 60 seconds. Duplicate submissions of the same 6-digit standard TOTP token within the valid time interval window are instantly rejected to block replay attacks.

### 2.3 Brute-Force & Lockout Rules
To avoid brute-force cracking of 6-digit tokens:
* Maximum failed attempts threshold: **5 attempts**.
* Failure cooldown period: **15 minutes**.
* Any authorization actions on the account during the cooldown period return `429 Too Many Requests`.

### 2.4 Backup Recovery Codes
During MFA enrollment, **8** cryptographically-secure random recovery codes (format: `XXXX-XXXX`) are generated and returned once to the user.
* To avoid plain-text compromise, recovery codes are hashed using high-strength **Bcrypt** configurations and saved inside the `user_mfa_backup_codes` table.
* On login challenge, the user can verify a backup recovery code, which instantly authorizes access, sets its state to used (`is_used = true`), and generates normal sessions.

---

## 3. Database Schema Layout (Migration 30)
1. **`user_mfa_backup_codes`**: Persists BCrypt hashes of generated backup codes.
2. **`user_mfa_sessions`**: Stacks temporary active login challenge states (valid for 5 minutes).
3. **`security_events`**: High-fidelity structured logs tracking enrollment, logins, disable operations, and failures.
