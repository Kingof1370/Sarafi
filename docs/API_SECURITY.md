# API SECURITY STANDARDS

## 1. Unified Authentication Middleware
- Seamlessly resolves standard JWT authorization and programmatic API Key signatures.

## 2. API Key HMAC Signing
- Key secrets are kept encrypted-at-rest.
- Programmatic API requests must supply high-precision HMAC-SHA256 signatures, validated against canonical request payloads.
- Replays are rejected using Redis nonce-indexing and a 300-second maximum clock skew filter.
- Optional CIDR IP allowlisting enforces network authorization boundaries.

## 3. JWT Signing Security
- Signing uses strong HS256/RS256 controls.
- Client-supplied algorithm parameters (e.g. `none`) are rejected.
- Revocations and concurrency caps (max 5 sessions per account) are tracked in PostgreSQL and Redis.
