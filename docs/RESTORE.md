# Controlled Restore & Database Schema Verification Guide

This document describes the manual database restoration procedure.

## 1. Restore Execution Sequence

1. **Integrity Validation Check**: The system validates that the target backup file exists on disk, reads the ciphertext, and verifies that the computed SHA-256 hash matches the database metadata registry.
2. **Decryption and Schema Parsing**: Decrypts the archive using the AES-256 symmetric key, parses the JSON payload, and verifies the maximum migration level.
3. **Authorization Check**: Requires appropriate administrator identity and active MFA TOTP validation.
4. **Clean Transactional Overwrite**: Clears existing states within a single database transaction and restores users, balances, ledger entries, and orders.
5. **Post-Restore Health Check**: Initiates live database ping probes and sets the global system mode back to `NORMAL`.

## 2. Triggering Restore via REST API

```http
POST /api/v1/system/recovery/validate
Authorization: Bearer <ADMIN_JWT>
X-MFA-Code: 123456
Content-Type: application/json

{
  "backup_id": "bak_1712200000000",
  "execute": true
}
```
