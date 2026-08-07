# Velyxora Exchange — P007 Key Management Database Report

## Metadata
* **Execution Timestamp:** 2026-08-02 21:15:00 UTC
* **Git Commit Hash:** 29153eb

## Schema Design (Migrations 54 - 58)
We expanded the database with the following normalized relational schemas:
* `cryptographic_keys` (id (PK), version, type, public_key, fingerprint, status, expiration_date, is_hsm_managed)
* `key_rotation_history` (id (PK), old_key_id, new_key_id, rotated_at)
* `key_audit_logs` (id (PK), key_id, action, message, timestamp)
* `signature_requests` (id (PK), raw_data, required_approvals, current_approvals, status, timestamp)
* `signature_approvals` (id (PK), signature_request_id (FK), admin_id, signature, created_at)

## Relational Constraints & Indexes
* **Foreign Keys:** All dependent tables feature foreign keys linking to `signature_requests.id` with `ON DELETE CASCADE` cascading deletes.
* **Indices:**
  - `idx_crypto_keys_status` on `cryptographic_keys (status)`
  - `idx_crypto_keys_fingerprint` on `cryptographic_keys (fingerprint)`
  - `idx_key_rotation_old` on `key_rotation_history (old_key_id)`
  - `idx_key_rotation_new` on `key_rotation_history (new_key_id)`
  - `idx_key_audit_logs_id` on `key_audit_logs (key_id)`
  - `idx_sig_requests_status` on `signature_requests (status)`
  - `idx_sig_approvals_req` on `signature_approvals (signature_request_id)`
