# VELYXORA SECURITY MODEL

Velyxora is designed from day one with security as an absolute priority. This document details the policies, libraries, and strategies used to defend the platform against attacks and ensure asset safety.

## 1. Authentication and Authorization

### JWT Access / Refresh Architecture
* Standard session actions employ JSON Web Tokens (JWT) signed using SHA-256 HMAC algorithms.
* **Access Tokens**: Configured with a default life span of **15 minutes** to minimize impact if intercepted.
* **Refresh Tokens**: Stored securely to request new access tokens; expires in **24 hours**.

### Cryptographic Hashing
* Plaintext passwords are never stored. We employ **Bcrypt** with a standard configuration cost of `12` or higher to ensure resistance against brute-force attacks.

### API Key Verification (HMAC Signature)
* For programmatic api integration, clients utilize API Keys.
* Requests must contain an HMAC signature of the request payload using their private API Secret, verified in our security layers:
```go
VerifyAPIKeySignature(payload, signature, apiSecret)
```

---

## 2. Secure Data Operations

* **Connection Pool Encryption**: PostgreSQL, Redis, and Kafka interfaces mandate TLS encryption layers for production environments.
* **Double-Entry Balance Updates**: Balances are settled using ACID-compliant PostgreSQL transactions. Balances cannot be updated without a matching ledger entry, ensuring audits are always balanced.
* **Zero Hardcoded Secrets**: Configuration is managed via Environment Variables. `.env.example` serves as a blueprint.
