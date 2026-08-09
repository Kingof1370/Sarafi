# SECRETS MANAGEMENT PROCESS

Velyxora manages high-critical system secrets through a centralized configuration module.

## 1. Secrets Abstraction (`packages/security/config.go`)
- Establishes a standard configuration structure that dynamically resolves keys from:
  - System environment variables (for standard development runs).
  - Concrete `SecretProvider` interface implementations (for production vaults).

## 2. Strict Validation & Startup Gates
- When running in production environment (`APP_ENV=production`), the system runs strict validation rules:
  - Rejects default insecure passwords (`postgres`).
  - Rejects default development JWT and API secrets.
  - Rejects insecure SQL SSLModes (`disable`).
  - Rejects running with debug modes (`APP_DEBUG=true`).
- **Failure is closed**: If any security assertion fails, Velyxora aborts startup immediately with error code 1.

## 3. Secret Masking
- System logging utilities redact credentials, tokens, or hashes to prevent leakage.
