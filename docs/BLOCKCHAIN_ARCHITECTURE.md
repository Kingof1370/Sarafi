# Velyxora Exchange — Blockchain Abstraction Layer Architecture

## 1. Purpose & Scope
This document specifies the Blockchain Abstraction Layer (BAL) of the Velyxora Exchange. The BAL isolates all network-specific cryptographic and ledger differences from the core exchange services, offering a unified, high-performance interface for multi-chain support.

---

## 2. Supported Blockchain Networks
The BAL supports standard native coins and token models across eight primary networks:
1. **Bitcoin (BTC)** (HD path derivation: `m/44'/0'/0'/0/0`)
2. **Ethereum (ETH)** (HD path derivation: `m/44'/60'/0'/0/0`)
3. **BNB Smart Chain (BSC)** (HD path derivation: `m/44'/60'/0'/0/0` EVM)
4. **Polygon (MATIC)** (HD path derivation: `m/44'/60'/0'/0/0` EVM)
5. **Solana (SOL)** (HD path derivation: `m/44'/501'/0'/0'`)
6. **Avalanche (AVAX)** (HD path derivation: `m/44'/60'/0'/0/0` EVM)
7. **Tron (TRX)** (HD path derivation: `m/44'/195'/0'/0/0`)
8. **Litecoin (LTC)** (HD path derivation: `m/44'/2'/0'/0/0`)

---

## 3. Modular Interface Architecture
Every blockchain connector implements the standard `BlockchainAdapter` Go interface inside `packages/blockchain/adapter.go`:

```go
type BlockchainAdapter interface {
	NetworkName() string
	ValidateAddress(address string) bool
	GenerateAddress(privateKey []byte) (string, error)
	DerivePublicKey(privateKey []byte) []byte
	SignTransaction(privateKey []byte, txData []byte) ([]byte, error)
	VerifySignature(publicKey []byte, txData []byte, signature []byte) (bool, error)
	DerivationPath() string
}
```

This interface facilitates adding a new blockchain network (EVM or non-EVM) with zero code modifications to the core wallet service.

---

## 4. Key Security & Signature Abstraction
* **BIP-39 & BIP-32 HD Wallets:** Implements deterministic, hierarchical child derivations utilizing a secure BIP-39 mnemonic phrase.
* **Signature Algos:** Uses standard ecdsa (P-256 / secp256k1) for BTC, ETH, BSC, Polygon, Avalanche, Tron, and Litecoin, and ed25519 for Solana.
* **Address Validation:** Matches regular expressions and checksum checks (e.g. Base58 check on Tron/BTC, hexadecimal check on EVM, Base58 on Solana).
