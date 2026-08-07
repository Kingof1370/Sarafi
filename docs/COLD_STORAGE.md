# Velyxora Exchange — Cold Storage & Multi-Signature Infrastructure

## 1. Overview
This document specifies the design, security protocols, and operational procedures governing Velyxora's Cold Storage infrastructure. Cold storage protects the vast majority of user and exchange assets (95%+) against online exploits, key leakage, and administrator compromise.

---

## 2. Key Architecture & Air-Gapped Key Gen
To completely eliminate the risk of online network-based attacks:
* **Key Generation:** Private keys for cold and deep cold vaults are generated inside fully air-gapped Hardware Security Modules (HSMs) or isolated machines utilizing BIP-39 mnemonic seeds.
* **Elliptic Curve:** Utilizes the industry-standard `secp256k1` Koblitz curve for ECDSA networks (Bitcoin, EVMs, Litecoin, Tron) and `ed25519` for Solana.
* **Deterministic Paths:** Derivations follow strict BIP-32 and BIP-44 path standards, ensuring identical address output structures across isolated environments.

---

## 3. Threshold Multi-Signature (m-of-n)
Cold and deep cold asset transfers utilize a secure m-of-n threshold multi-signature signing scheme:
* **Cold Vault transfers:** Require **2** independent signatures (2-of-3 or 2-of-5 admin keys).
* **Signing Procedure:**
  1. Transaction payload is generated on the active matching node.
  2. Transaction is queued in `signature_requests`.
  3. Signers download the payload, verify the destination address, sign locally, and submit their signatures.
  4. Once 2 valid signatures are registered, the transaction is marked as signed and queued for network broadcast.
