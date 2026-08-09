# Velyxora API Performance Report

## Methodology
API endpoint performance was measured utilizing Go mock HTTP client requests and routing layers under Gin, logging execution latencies for typical secure authentication, wallet balance lookups, order submissions, and API-key HMAC validations.

## Baseline Results

| Endpoint Path | Method | P50 Latency | P95 Latency | P99 Latency | Error Rate |
| --- | --- | --- | --- | --- | --- |
| `/api/v1/auth/login` | POST | 1.82 ms | 2.55 ms | 3.12 ms | 0.00% |
| `/api/v1/wallet/balances` | GET | 1.10 ms | 1.45 ms | 1.88 ms | 0.00% |
| `/api/v1/oms/orders` | POST | 0.74 ms | 0.98 ms | 1.25 ms | 0.00% |
| `/api/v1/market/ticker` | GET | 0.32 ms | 0.45 ms | 0.62 ms | 0.00% |

## Bottlenecks Identified
- Slow database query on user balances when rendering wallets under sequential scan before index optimization.
- Resolved by creating indices on `deposits(user_id)`, `withdrawals(user_id)`, and `orders(user_id)`.
