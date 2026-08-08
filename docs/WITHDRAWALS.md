# Velyxora Withdrawals Manual

This document details the withdrawal lifecycle and transaction construction pipeline under P0008.

## Withdrawal Lifecycle States
The withdrawal lifecycle transitions deterministically through the following states:

`REQUESTED -> VALIDATING -> PENDING_APPROVAL -> APPROVED -> BROADCASTING -> CONFIRMING -> COMPLETED`

### State Transitions
1. **Creation**: When a user submits a withdrawal via `/withdraw`, the API Gateway validates authentication, user status, destination address format, available balance, and checks custody policy limits.
2. **Reservation**: Balance reservation is atomic. Available balance is deducted and reserved atomically using a PostgreSQL row-level lock (`SELECT FOR UPDATE`).
3. **Approval**: If the withdrawal exceeds custody dual-approval thresholds, it is placed in `PENDING_APPROVAL` status and requires administrative signatures. Otherwise, it starts as `APPROVED`.
4. **Broadcast**: The background `WithdrawalWorker` picks up `APPROVED` withdrawals, builds transaction inputs (UTXOs/Nonces), signs them with simulated keys, and broadcasts via the network-specific adapter, transitioning status to `BROADCASTING`.
5. **Finality**: The worker polls confirmations until finality. Upon completion, the reserved segment is permanently debited and ledger transactions are posted, transitioning the withdrawal to `COMPLETED`.
