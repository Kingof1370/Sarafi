# Velyxora Exchange — Institutional Treasury Management System

## 1. Overview
The Velyxora Institutional Treasury Management System is designed to manage exchange-owned asset capital pools, operational buffer balances, insurance insolvency cushions, and platform fee distributions securely and autonomously.

The subsystem segregates and protects exchange assets through:
* **Strict Capital Pool Isolation:** Corporate treasury, platform reserves, insolvency insurance funds, and client hot wallets exist in separate logical and physical balance structures.
* **Granular Multi-Party Dual Approvals:** Large capital movements require independent multiple administrative signoffs.
* **Real-time Liquidity & Backing Auditing:** Constant monitoring of liquidity parameters and backing collateral ratios.

---

## 2. Exchange Capital Pools
Exchange capital is strictly partitioned into standard institutional asset pools:
1. **HOT Pool (`PoolHot`):** Primary buffer to settle standard client deposits and withdrawals.
2. **WARM Pool (`PoolWarm`):** Semi-liquid asset buffer to top-up Hot pool allocations safely.
3. **COLD Pool (`PoolCold`):** Long-term offline reserves.
4. **TREASURY Pool (`PoolTreasury`):** Core corporate working capital and operational holdings.
5. **RESERVE Pool (`PoolReserve`):** Platform backing collateral supporting reserve accounts.
6. **INSURANCE Pool (`PoolInsurance`):** Protection fund designated to absorb black-swan insolvency events.
7. **FEE WALLET Pool (`PoolFeeWallet`):** Fee accumulation ledger pool collecting trades and operational margins.
8. **OPERATIONAL Pool (`PoolOperational`):** Active pool designed to handle day-to-day corporate expenditures and payroll.

---

## 3. High Availability API Endpoints
* `GET /api/v1/wallet/treasury/overview` — Total assets and health state of all institutional pools.
* `GET /api/v1/wallet/treasury/history` — List of all completed and requested internal transfers.
* `GET /api/v1/wallet/treasury/liquidity` — Live order book spread and bid/ask depth metrics.
* `GET /api/v1/wallet/treasury/reserve` — Backing backing ratio metrics for reserve accounts.
* `POST /api/v1/wallet/treasury/transfer` — Request an institutional transfer.
* `POST /api/v1/wallet/treasury/approve/:id` — Approve an internal movement.
* `POST /api/v1/wallet/treasury/reconcile` — Manually trigger multi-layered automatic reconciliation.
* `GET /api/v1/wallet/treasury/reconcile/results` — Historical list of reconciliation execution reports.
