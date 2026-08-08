# Velyxora Blockchain Reorganization Manual

This document details how Velyxora detects and corrects chain reorganizations under P0008.

## Reorg Detection
A chain reorganization (reorg) occurs when alternative blocks replace previously mined blocks.
In `SIMULATION` mode, our background scanner compares confirmed deposits against the current block hashes. If a previously confirmed transaction's block height hash does not match, or the transaction is orphaned, the scanner triggers a reorg correction.

## Reorg Correction & Reversal
If an orphaned transaction is detected:
1. Update deposit status to `REJECTED`.
2. Atomically debit the credited user available balance to prevent unauthorized double-spending.
3. Post compensating ledger reversal entries (Debit user balance, Credit COINBASE pool) to ensure total accounting trial balance remains correct.
4. Publish `deposit.reorged` Kafka event.
