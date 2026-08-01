# VELYXORA BLOCKCHAIN ABSTRACTION MANUAL

Velyxora provides scalable blockchain adapters to standardize address validations and transaction broadcasting.

## Interface Definition

```go
type BlockchainAdapter interface {
	ValidateAddress(address string) bool
	GetNativeBalance(address string) (float64, error)
	BroadcastTransaction(rawTx string) (string, error)
	GetConfirmations(txHash string) (int, error)
}
```

## Validation Formats

- **Bitcoin (BTC)**: Legacy legacy and Bech32 format parsing (prefix rules `1`, `3`, `bc1`).
- **Ethereum (ETH)**: EVM Hex patterns (`0x` prefix followed by 40 hex characters).
- **Solana (SOL)**: Base58 characters verification rules.
