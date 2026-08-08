# Liquidity Abstraction

Velyxora provides a modular, clean interface to interact with internal and external liquidity providers.

## Interface Definition

```go
type LiquidityProvider interface {
	ID() string
	GetOrderBook(symbol string) (*types.OrderBookL2, error)
	ExecuteOrder(order *types.Order) (*types.Trade, error)
	GetStatus() string
}
```

## Safety Boundaries
All external executions must go through standard risk, wallet, and ledger validation inside the exchange core. External liquidity sources are treated as untrusted boundaries and can never bypass regulatory controls.
