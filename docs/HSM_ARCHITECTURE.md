# Velyxora Exchange — HSM Integration Architecture

## 1. Overview
The platform isolates raw signing private key materials inside Hardware Security Modules (HSM) or secure cloud enclaves to mitigate the threat of server compromise.

---

## 2. HSM Interface Abstraction
Every physical HSM vendor or cloud KMS connector implements the standard `HSMProvider` interface inside `packages/keys/keys.go`:

```go
type HSMProvider interface {
	GenerateKey(keyType KeyType) ([]byte, error)
	Sign(keyID string, rawData []byte) ([]byte, error)
	Verify(keyID string, rawData []byte, signature []byte) (bool, error)
}
```

This ensures that business logic remains completely unchanged when upgrading or replacing physical hardware enclaves.
