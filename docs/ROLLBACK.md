# Rollback Procedures
## Velyxora Enterprise Exchange

Detailed procedures to safely roll back production builds under failure.

### 1. Stateless Services (API Gateway, Frontend, Wallet Service)
Stateless applications can be safely rolled back to a previous stable tag instantly:
```bash
kubectl rollout undo deployment/api-gateway -n velyxora
kubectl rollout undo deployment/frontend -n velyxora
kubectl rollout undo deployment/wallet-service -n velyxora
```

### 2. State-Machine Services (Matching Engine)
The Matching Engine processes sequential, monotonic order matching events. To roll back:
1. Halt order ingest via Kafka queue.
2. Replay the state ledger from the nearest stable PostgreSQL snapshot + subsequent Kafka journal offsets.
3. Validate matching engine determinism metrics before resuming.
