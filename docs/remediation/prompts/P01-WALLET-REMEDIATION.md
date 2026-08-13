# Prompt: Secure Wallet Derivation & Key Management (PART-01)

## Objective
Remediet the insecure and non-derivable cryptographic address generation in `packages/common/address_crypto.go`. Transition from raw HMAC-SHA256 hashes of string index labels to standard BIP-39 mnemonic seed generation, BIP-32 HD key trees, and BIP-44 multi-asset derivation paths to generate authentic, verifiable, and recoverable addresses for Bitcoin (BTC), Ethereum/EVM (ETH, USDT, USDC, BNB, BSC, POLYGON, AVAX), TRON (TRX), Litecoin (LTC), and Solana (SOL).

## Scope
- Modify `packages/common/address_crypto.go`
- Integrate cryptographic libraries for BIP-39, BIP-32, BIP-44, SECP256K1, and Solana ED25519 derivations.
- Implement robust address checksum validation, network segregation (Mainnet/Testnet), and public key hash derivations.

## Requirements & Specifications

### 1. HD Wallet Derivation Pipeline
The system must support:
`Mnemonic (BIP-39) -> Seed -> Master Private Key (BIP-32) -> Purpose (BIP-44) -> Coin Type -> Account -> Change -> Address Index`

Implement exact BIP-44 derivation paths for each coin type:
- **BTC**: `m/44'/0'/0'/0/index` (Bitcoin Mainnet, starts with `1` for legacy or segwit address format).
- **ETH**: `m/44'/60'/0'/0/index` (Ethereum/EVM formats, starts with `0x`).
- **LTC**: `m/44'/2'/0'/0/index` (Litecoin Mainnet, starts with `L`).
- **TRX/TRON**: `m/44'/195'/0'/0/index` (TRON format, starts with `T`).
- **SOL**: `m/44'/501'/0'/0/index` (Solana, Base58 encoded 32 bytes public key).

### 2. Standard Cryptographic Derivation Implementation
- **BTC/LTC/TRON**: Derive the SECP256K1 public key from the derived private key.
  - **BTC Address**: Compute HASH160 (RIPEMD160 of SHA256) of the compressed SECP256K1 public key. Encode the resulting 20-byte hash using Base58Check with version prefix `0x00`.
  - **LTC Address**: Compute HASH160 of the compressed SECP256K1 public key. Encode using Base58Check with version prefix `0x30`.
  - **TRON Address**: Compute the Keccak-256 hash of the uncompressed SECP256K1 public key (excluding the single byte `0x04` prefix). Take the last 20 bytes, prefix with `0x41`, and encode using Base58Check.
- **ETH (EVM) Address**: Compute the Keccak-256 hash of the uncompressed SECP256K1 public key (excluding prefix `0x04`). Take the last 20 bytes and format as `0x` + hexadecimal string (applying EIP-55 Mixed-case checksum address encoding).
- **Solana Address**: Use the standard ED25519 curve derivation. Since ED25519 uses direct public key mapping, derive the ED25519 public key bytes and encode them directly using Base58.

### 3. Verification & Validation Helpers
- Implement strict address validation helpers for each supported network, parsing checksums, version bytes, and alphabet boundaries.
- Disallow simplistic validation hacks like prefix checking (`strings.HasPrefix`). Perform authentic decoding and checksum verification.

### 4. Fail-Closed Principles
- In production, missing or weak `WALLET_SEED` or `WALLET_MNEMONIC` must trigger a strict **FAIL CLOSED** state and abort startup.

## Testing Strategy
- Create `packages/common/address_crypto_test.go` and write unit tests to:
  - Verify that given the same master seed, addresses generated for BTC, ETH, LTC, TRX, and SOL match known, deterministic test vectors.
  - Test EIP-55 checksum validation for Ethereum addresses.
  - Verify that wrong or corrupt addresses are rejected by validation helpers.
  - Test that missing seeds fail closed under `APP_ENV=production`.
