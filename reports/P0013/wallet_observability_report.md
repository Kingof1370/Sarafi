# Velyxora - SRE Wallet Service Observability Report

We have integrated core technical SRE metrics inside our balance ledger double-entry and withdrawal engines:

- **Wallet Operations Total**: Captures `velyxora_wallet_operations_total` categorized by operation type (`settlement`) and asset class (`BTC`, `USDT`).
- **Ledger Latency**: Query timings on balance locks (`SELECT FOR UPDATE`) are logged in the database latency histogram metric.
- **Transactional Errors**: Database update failures or insufficient balance exceptions increment `velyxora_postgres_errors_total`.
