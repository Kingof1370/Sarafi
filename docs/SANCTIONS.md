# Sanctions and Watchlist Architecture

Velyxora integrates a robust, provider-agnostic Sanctions Screening Engine.

## Abstraction Interface
The screener exposes a clear interface:
```go
type SanctionsProvider interface {
	Screen(ctx context.Context, targetType string, targetValue string) (SanctionsStatus, string, error)
}
```

## Failure-Closed Security Policy
This rule is absolute. Under no circumstances will a sanctions provider failure (timeout, API offline, error response) silently convert into a `CLEAR` or `APPROVED` status.
- If an external screening provider is unavailable, the transaction is marked as held and blocked under an explicit `UNAVAILABLE` or `ERROR` state.
- All withdrawals are permanently blocked if sanctions screen checks return errors.
- Manual intervention by a `COMPLIANCE_OFFICER` is required to release any held transfers.
