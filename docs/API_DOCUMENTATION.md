# Velyxora Exchange — Public REST API Specification

## 1. Overview
The Velyxora REST Gateway exposes standard REST endpoints for managing user profiles, auth sessions, orders, balances, and deposit/withdrawal tracking.

All private REST routes require a valid JWT passed in the standard Authorization Header:
`Authorization: Bearer <JWT_TOKEN>`

---

## 2. Wallet Subsystem APIs

### 2.1 Get Wallet Balance Summary
* **Route:** `GET /api/v1/wallet/summary`
* **Access:** Private (Authenticated)
* **Response Output:**
```json
{
  "summary": [
    {
      "wallet_id": "wal_hot_usr_mock_123",
      "type": "HOT",
      "is_locked": false,
      "asset": "BTC",
      "available": 1.25,
      "locked": 0.1,
      "reserved": 0.0,
      "pending": 0.0,
      "total": 1.35
    }
  ]
}
```

### 2.2 List Registered Assets
* **Route:** `GET /api/v1/wallet/assets`
* **Access:** Private / Public
* **Response Output:**
```json
{
  "assets": [
    {
      "symbol": "BTC",
      "name": "Bitcoin",
      "type": "NATIVE",
      "precision": 8,
      "base_network": "Bitcoin",
      "is_active": true,
      "description": "Digital Gold",
      "website": "bitcoin.org",
      "explorer_url": "blockchain.com",
      "circulating_price": 95000.0,
      "can_deposit": true,
      "can_withdraw": true,
      "can_trade": true,
      "withdrawal_fee": 0.0005
    }
  ]
}
```

### 2.3 Get Single Asset Details
* **Route:** `GET /api/v1/wallet/assets/:symbol`
* **Access:** Private
* **Response Output:**
```json
{
  "symbol": "BTC",
  "name": "Bitcoin",
  "type": "NATIVE",
  "precision": 8,
  "base_network": "Bitcoin",
  "is_active": true,
  "description": "Digital Gold",
  "website": "bitcoin.org",
  "explorer_url": "blockchain.com",
  "circulating_price": 95000.0,
  "can_deposit": true,
  "can_withdraw": true,
  "can_trade": true,
  "withdrawal_fee": 0.0005
}
```

### 2.4 List Generated Addresses
* **Route:** `GET /api/v1/wallet/addresses`
* **Access:** Private (Authenticated)
* **Response Output:**
```json
{
  "addresses": [
    {
      "address": "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
      "network": "Bitcoin",
      "status": "ALLOCATED",
      "derivation_path": "m/44'/0'/0'/0/0",
      "created_at": "2026-08-02T12:00:00Z"
    }
  ]
}
```

### 2.5 Get Detailed Balance Breakdowns
* **Route:** `GET /api/v1/wallet/balances/details`
* **Access:** Private (Authenticated)
* **Response Output:**
```json
{
  "balances_details": [
    {
      "wallet_id": "wal_hot_usr_mock_123",
      "asset": "BTC",
      "available": 1.25,
      "locked": 0.1,
      "reserved": 0.0,
      "pending": 0.0,
      "total": 1.35,
      "updated_at": "2026-08-02T12:00:00Z"
    }
  ]
}
```

### 2.6 Get Wallet Audits / History
* **Route:** `GET /api/v1/wallet/history`
* **Access:** Private (Authenticated)
* **Response Output:**
```json
{
  "history": [
    {
      "id": "audit_111",
      "wallet_id": "wal_hot_usr_mock_123",
      "asset": "BTC",
      "action": "BALANCE_ADJUSTED",
      "amount": 1.5,
      "prev_balance": 0.0,
      "new_balance": 1.5,
      "message": "Onboarding bonus credit",
      "timestamp": "2026-08-02T11:00:00Z"
    }
  ]
}
```

---

## 3. Deposit Engine Subsystem APIs

### 3.1 Get Deposit History
* **Route:** `GET /api/v1/wallet/deposits/history`
* **Access:** Private (Authenticated)
* **Response Output:**
```json
{
  "deposits": [
    {
      "id": "dep_mock_111",
      "user_id": "usr_123",
      "asset": "BTC",
      "amount": 0.15,
      "fee": 0.0,
      "address": "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
      "tx_hash": "0x123abc456def7890_btc",
      "confirmations": 6,
      "status": "COMPLETED",
      "created_at": "2026-08-02T10:00:00Z",
      "updated_at": "2026-08-02T11:00:00Z"
    }
  ]
}
```

### 3.2 Get Deposit Status
* **Route:** `GET /api/v1/wallet/deposits/status/:id`
* **Access:** Private (Authenticated)
* **Response Output:**
```json
{
  "id": "dep_mock_111",
  "status": "COMPLETED",
  "confirmations": 6
}
```

### 3.3 Get Deposit Details
* **Route:** `GET /api/v1/wallet/deposits/details/:id`
* **Access:** Private (Authenticated)
* **Response Output:**
```json
{
  "id": "dep_mock_111",
  "user_id": "usr_123",
  "asset": "BTC",
  "amount": 0.15,
  "fee": 0.0,
  "address": "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
  "tx_hash": "0x123abc456def7890_btc",
  "confirmations": 6,
  "status": "COMPLETED",
  "created_at": "2026-08-02T10:00:00Z",
  "updated_at": "2026-08-02T11:00:00Z"
}
```

### 3.4 Lookup Blockchain Transaction
* **Route:** `GET /api/v1/wallet/deposits/tx/:tx_hash`
* **Access:** Private (Authenticated)
* **Response Output:**
```json
{
  "tx_hash": "0x123abc456def7890_btc",
  "network": "Bitcoin",
  "asset": "BTC",
  "amount": 0.15,
  "sender": "1SenderAddr_111",
  "receiver": "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
  "block_number": 750431,
  "gas_used": 0,
  "timestamp": "2026-08-02T10:00:00Z"
}
```

### 3.5 Get Confirmation Status
* **Route:** `GET /api/v1/wallet/deposits/confirmations/:id`
* **Access:** Private (Authenticated)
* **Response Output:**
```json
{
  "deposit_id": "dep_mock_111",
  "confirmations_count": 6,
  "required_confirmations": 6,
  "status": "COMPLETED",
  "updated_at": "2026-08-02T11:00:00Z"
}
```

---

## 4. Digital Asset Custody REST APIs

### 4.1 Get Custody Total Value Overview
* **Route:** `GET /api/v1/wallet/custody/overview`
* **Access:** Private (Authenticated Administrator)
* **Response Output:**
```json
{
  "total_custody_value_btc": 2500.50,
  "status": "SECURE",
  "active_alerts": 0
}
```

### 4.2 List Institutional Custody Vaults
* **Route:** `GET /api/v1/wallet/custody/vaults`
* **Access:** Private (Authenticated Administrator)
* **Response Output:**
```json
{
  "vaults": [
    {"id": "v_hot", "name": "Hot Exchange Vault", "type": "HOT", "is_locked": false},
    {"id": "v_cold", "name": "Cold Core Storage", "type": "COLD", "is_locked": false},
    {"id": "v_treasury", "name": "Corporate Treasury Vault", "type": "TREASURY_VAULT", "is_locked": false}
  ]
}
```

### 4.3 Request Custody Asset Transfer
* **Route:** `POST /api/v1/wallet/custody/transfer`
* **Access:** Private (Authenticated Administrator)
* **Payload:**
```json
{
  "from_vault_id": "v_cold",
  "to_vault_id": "v_hot",
  "asset": "BTC",
  "amount": 150.0
}
```
* **Response Output:**
```json
{
  "message": "Custody transfer request submitted, pending administrative approvals",
  "transfer_id": "cust_tx_1690934400000000_BTC",
  "status": "REQUESTED"
}
```

### 4.4 Approve Custody Asset Transfer (4-Eyes Workflow)
* **Route:** `POST /api/v1/wallet/custody/approve/:id`
* **Access:** Private (Authenticated Administrator)
* **Response Output:**
```json
{
  "message": "Custody transfer approval registered successfully",
  "transfer_id": "cust_tx_1690934400000000_BTC",
  "status": "COMPLETED"
}
```

### 4.5 Get Asset Distribution across Vaults
* **Route:** `GET /api/v1/wallet/custody/distribution`
* **Access:** Private (Authenticated Administrator)
* **Response Output:**
```json
{
  "distribution": [
    {"type": "HOT", "total_btc": 250.0},
    {"type": "COLD", "total_btc": 2250.50}
  ]
}
```

### 4.6 Get Custody Audit Logs
* **Route:** `GET /api/v1/wallet/custody/audit`
* **Access:** Private (Authenticated Administrator)
* **Response Output:**
```json
{
  "custody_audits": [
    {
      "id": "aud_cust_1",
      "vault_id": "v_cold",
      "action": "DEPOSIT",
      "amount": 500.0,
      "message": "Initial reserve deposit",
      "timestamp": "2026-08-02T10:00:00Z"
    }
  ]
}
```

### 4.7 Execute System-Wide Emergency Action
* **Route:** `POST /api/v1/wallet/custody/emergency/:action`
* **Access:** Private (Authenticated Emergency Operator)
* **Response Output:**
```json
{
  "message": "Emergency action executed successfully",
  "action": "FREEZE",
  "status": "LOCKED"
}
```
