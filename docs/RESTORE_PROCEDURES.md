# VELYXORA EXCHANGE - RESTORE PROCEDURES

## 1. RESTORE STRATEGY & WORKFLOW
Restoring the exchange database must be executed with absolute safety controls to ensure authoritative financial state consistency. Restoring directly into an active production database is strictly prohibited.

## 2. RESTORE PROCEDURE (OPERATOR GUIDELINES)

### Step 1: Initialize isolated verify environment
Ensure the target recovery server is configured in sandbox/verification mode. Operators must fetch the encrypted backup archive (e.g. `bk_db_*.enc`) from secure storage.

### Step 2: Decrypt the backup archive
Decrypt the AES-256-GCM encrypted file utilizing the dedicated backup key:
```bash
# Decrypt archive stream
openssl enc -d -aes-256-gcm -K <BACKUP_ENCRYPTION_KEY> -in bk_db.enc -out bk_db.sql
```

### Step 3: Run isolated sandbox validation
Create a dynamic verification database instance to restore the dump:
```bash
docker exec -it velyxora-db createdb -U velyxora velyxora_restore_verify
docker exec -i velyxora-db psql -U velyxora -d velyxora_restore_verify < bk_db.sql
```

### Step 4: Validate double-entry balanced state
Run core database verification assertions on the restored isolated schema:
- **Ledger Balances Verification:** Check that the sum of ledger debits equals the sum of ledger credits.
- **Sequence Verification:** Assert that sequence identifiers and monotonic trade logs are not corrupted.
- **Key Count Verification:** Verify user credentials count and API Key active state is fully preserved.

### Step 5: Destroy verification sandbox
After verifying compliance, destroy the isolated verification database:
```bash
docker exec -it velyxora-db dropdb -U velyxora velyxora_restore_verify
```
