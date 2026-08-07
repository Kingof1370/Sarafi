# Velyxora Exchange — Enterprise Asset Management & Registry

## 1. Purpose & Scope
This document outlines the Asset Registry and Asset Metadata subsystems inside Velyxora Exchange. It establishes how asset tokens (native coins, ERC-20, stablecoins, wrapped assets, and future NFT classes) are registered, maintained, and subjected to trade/withdrawal restrictions on the platform.

---

## 2. Architecture & Design Decisions
The Assets module resides in `packages/assets` as a shared library and integrates directly with the API Gateway and matching engine database to enforce real-time limits.

```
+-------------------------------------------------------------+
|                     Asset Registry                          |
|  +--------------------+----------------------------------+  |
|  | Asset Configurations|   Symbol, Name, Type, Base Net  |  |
|  +--------------------+----------------------------------+  |
|  | Metadata Profile   |   Contract, Explorer, Price, Logo|  |
|  +--------------------+----------------------------------+  |
|  | Granular Perms     |   Deposit/Withdraw/Trade status  |  |
|  +--------------------+----------------------------------+  |
+-------------------------------------------------------------+
```

### Design Decisions
* **Strict Class Separations:** Explicit types for `NATIVE`, `TOKEN`, `STABLECOIN`, and `WRAPPED`.
* **Database Isolation:** Assets are saved on Bootstrap inside the `assets_registry` and left-joined dynamically with `assets_metadata` and `assets_permissions` with `COALESCE` handling null entries cleanly.

---

## 3. Database Schema
* `assets_registry` (symbol, name, type, precision, base_network, is_active, created_at)
* `assets_metadata` (symbol, contract_address, logo_url, description, website, explorer_url, total_supply, circulating_price, updated_at)
* `assets_permissions` (symbol, can_deposit, can_withdraw, can_trade, min_deposit_amount, min_withdraw_amount, max_daily_withdrawal, withdrawal_fee, updated_at)

---

## 4. API Specifications
* `GET /api/v1/wallet/assets` — Returns complete details of all registered assets including permissions and prices.
* `GET /api/v1/wallet/assets/:symbol` — Returns detailed specifications for a single token.

---

## 5. Event Flow
Upon asset registration, an `AssetRegisteredEvent` is published onto the `velyxora-asset-registered` topic using Kafka versioned schemas.

---

## 6. Institutional Asset Partitioning
Registered assets are partitioned across institutional custody vaults:
- **Hot Vault (`v_hot`):** Retains active, liquid exchange balances to cover daily deposit/withdrawal loops.
- **Warm Vault (`v_warm`):** Acts as a buffer and staging pool for immediate hot wallet replenishments.
- **Cold Storage (`v_cold`, `v_deep_cold`):** Air-gapped key management holds 95%+ of institutional holdings under multi-signature administrative lock.
