# Programmatic API Keys Specification

This document details the lifecycle, storage, whitelisting, and scopes of programmatic API Keys.

## 1. Cryptographic Storage
To prevent raw credential exposure in case of database leaks:
- The public `api_key` string is stored in plain index.
- The private `api_secret` is encrypted using **AES-256-GCM envelope encryption** with a server-side master key before being saved in `user_api_keys.api_secret_hash`. It is **never** exposed again after creation.

---

## 2. API Key Management Endpoints
- `POST /api/v1/apikeys`: Create secure API key. Requires label, permissions/scopes list, IP allowlist, expiry days, and **P0003 MFA code verification**. Returns the raw API key and secret only once.
- `GET /api/v1/apikeys`: List user's active API keys (with secret omitted).
- `DELETE /api/v1/apikeys/:id`: Revoke/delete API key.

---

## 3. Scopes & Permissions
API Keys support fine-grained scoping. The key can be restricted to specific permissions like `wallet:read`, `wallet:write`, or `trading:write`. Requests verified via HMAC are only allowed to access endpoints whose required permission is present in the API key's scopes.

---

## 4. IP CIDR Allowlisting
- **Supported Formats**: Single IPv4, IPv6, or CIDR blocks (e.g., `192.168.1.0/24`).
- **Enforcement**: Normalized and parsed server-side using Go's `"net"` package. If an allowlist is defined, any request originating from an IP address outside the allowed blocks is immediately blocked with `403 Forbidden`.
