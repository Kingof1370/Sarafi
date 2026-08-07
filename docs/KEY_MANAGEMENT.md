# Velyxora Exchange — Cryptographic Key Management Specification

## 1. Purpose & Scope
The Cryptographic Key Management subsystem controls the secure generation, lifecycle, rotation, and revocation of all encryption and signing keys on the platform. It provides compatibility with Hardware Security Modules (HSM), cloud key management services, and multi-signature threshold sign-off procedures.

---

## 2. Core Components & Architecture
```
+------------------------------------------------------------+
|                     Key Management                         |
|   +-----------------------+-----------------------------+  |
|   | Key Manager           |  Generates and rotates keys |  |
|   +-----------------------+-----------------------------+  |
|   | HSM Foundation        |  Hardware Security Abstraction|  |
|   +-----------------------+-----------------------------+  |
|   | Key Registry          |  Stores versions & metadata |  |
|   +-----------------------+-----------------------------+  |
+------------------------------------------------------------+
```

* **Key Manager:** Coordinates core lifecycles: generation, rotation, expiration, and revocation.
* **HSM Foundation:** Abstraction interface allowing easy integration of external Hardware Security Module and Cloud KMS providers (e.g. AWS KMS, Offline Cold Enclaves).
* **Key Registry:** Databases key details, fingerprint hashes, and version lineages.

---

## 3. Supported Curves & Algorithms
The key management infrastructure natively supports:
* **Ed25519:** For Solana signatures.
* **ECDSA (secp256k1 / P-256):** For BTC, EVM networks, Tron, and Litecoin.
* **Secure Random Number Generation:** Employs standard cryptographically secure random readers (`crypto/rand`) for all seed materials.
