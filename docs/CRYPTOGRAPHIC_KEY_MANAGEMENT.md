# CRYPTOGRAPHIC KEY MANAGEMENT

This document details the envelope encryption and key rotation policies designed to protect customer data.

## 1. Centralized Key Manager (`packages/security/keys.go`)
- Combines versioned encryption keys under a structured mapping.
- Automatically handles ciphertext prefixes (`v[version]:[ciphertext]`) to resolve the exact key required during decryption.

## 2. Backward-Compatible Key Rotation
- When key rotation is initiated, new secrets are encrypted using the **latest active version**.
- Ciphertexts that do not match version patterns are decoded using the legacy unversioned keys, ensuring existing data remains accessible.
- Verified mathematically through automated unit testing.

## 3. Cryptographic Storage Standards
- High-precision wallets, API secrets, and sessions are encrypted using **AES-256-GCM** envelope keys.
