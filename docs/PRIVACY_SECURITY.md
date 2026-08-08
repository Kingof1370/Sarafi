# Data Privacy & Personal Security Policy

Velyxora places data privacy, security, and SOC2/GDPR compliance at the center of its compliance implementation.

## Protection of Sensitive Identity Data
- **No Plaintext Documents**: Raw identity files or biometric selfies are never stored in plaintext inside database fields or logs.
- **Envelope Encryption**: KYC metadata and document properties are stored using AES-256-GCM envelope encryption.
- **Secure Logs**: Personally Identifiable Information (PII) is automatically filtered out from console logs, error payloads, and Kafka messages.

## Audit trails
- Every compliance authorization change, KYC review, case note addition, or alert resolution is completely immutable, capturing WHO, WHAT, WHEN, and WHY.
