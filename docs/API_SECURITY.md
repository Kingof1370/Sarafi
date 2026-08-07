# API Security Specification

This document details the enterprise-grade security protocols implemented on Velyxora Exchange REST and WebSocket APIs.

## 1. Unified Authentication Protocol
Velyxora provides dual-factor authentication on its gateway services:
- **JWT Session Tokens**: For user-facing Next.js dashboard interactions.
- **HMAC API Keys**: For high-speed algorithmic and programmatic trading clients.

These are unified server-side through `UnifiedAuthMiddleware`, which delegates requests to either the API Key Signature Verifier or standard JWT token checker depending on request headers.

---

## 2. Programmatic HMAC-SHA256 API Request Signing
All programmatic trading requests must be signed with HMAC-SHA256 using the key's private API Secret.

### Custom Headers Required:
- `X-API-KEY`: The public API Key identifier.
- `X-SIGNATURE`: The calculated hex-encoded signature.
- `X-TIMESTAMP`: The current request timestamp (ISO-8601 or Unix milliseconds).
- `X-NONCE`: A unique client-generated request identifier.

### Canonical Signing Payload
The payload is composed by concatenating the following variables exactly, separated by a newline (`\n`):
```
timestamp + "\n" +
nonce + "\n" +
HTTP_METHOD + "\n" +
REQUEST_PATH + "\n" +
REQUEST_BODY
```

### Signature Verification Algorithm
```go
canonicalPayload := timestamp + "\n" + nonce + "\n" + method + "\n" + path + "\n" + body
mac := hmac.New(sha256.New, []byte(apiSecret))
mac.Write([]byte(canonicalPayload))
expectedSig := hex.EncodeToString(mac.Sum(nil))
```

---

## 3. Cryptographic Replay Protection
- **Timestamp Clock Skew**: Request timestamps are validated to be within a **300-second (5 minutes)** window relative to server time. Requests outside this window are immediately aborted with `401 Unauthorized`.
- **Nonce Tracking**: Nonces are stored in a distributed Redis sorted set or cache with a 300-second TTL. If a nonce is reused within the sliding replay window, the request is blocked and logged as a high-severity replay attempt.
