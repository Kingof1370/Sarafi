# Prompt: P02 — Wallet & Cryptography

## 1. Objective
Transition the wallet address generation to standard, cryptographically secure, and recoverable hierarchical deterministic key derivation.

## 2. Current Problem
- `GenerateCryptographicAddress` used insecure HMAC-SHA256 slice derivation instead of standard BIP-32/BIP-44 HD key trees.

## 3. Root Cause
Simplistic key mapping of string index paths without performing SECP256K1 scalar point multiplications.

## 4. Affected Modules
- `packages/common/`

## 5. Affected Files
- `packages/common/address_crypto.go`
- `packages/common/blockchain.go`

## 6. Architectural & Technical Requirements
- Implement standard child key derivation CKDpriv: `(parent_IL + IL) % N`.
- Implement compressed and uncompressed public SECP256K1 calculations followed by EIP-55 casing for ETH or Base58Check for BTC/TRON.

## 7. Migration & Backward Compatibility
- Backwards compatible with existing `wallet_addresses` PostgreSQL tables.

## 8. Security & Financial Requirements
- In production, missing `WALLET_SEED` must trigger Fail-Closed startup crash loops.
- Authenticate all validations with Double-SHA256 and Base58Check checksum verifications.

## 9. Tests & Failure Scenarios
- Test with known vectors for BTC, ETH, LTC, TRX, and SOL.
- Validate invalid address format rejections.

## 10. Acceptance Criteria & Definition of Done
- Standard HD address trees pass deterministic checks.
- Validations confirm checksum digits.
- Completed with Git commit.
