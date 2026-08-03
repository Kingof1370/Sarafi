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

---

## 3. Digital Asset Custody REST APIs

### 3.1 Get Custody Total Value Overview
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

### 3.2 List Institutional Custody Vaults
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

---

## 4. Institutional Treasury REST APIs

### 4.1 Get Treasury Overview
* **Route:** `GET /api/v1/wallet/treasury/overview`
* **Access:** Private (Authenticated Administrator)
* **Response Output:**
```json
{
  "total_treasury_value_usdt": 12500500.75,
  "status": "OPTIMAL"
}
```

### 4.2 List Internal Transfers History
* **Route:** `GET /api/v1/wallet/treasury/history`
* **Access:** Private (Authenticated Administrator)
* **Response Output:**
```json
{
  "transfers": [
    {"id": "tx_int_111", "from_pool": "TREASURY", "to_pool": "HOT", "asset": "BTC", "amount": 10.0, "status": "COMPLETED", "timestamp": "2026-08-03T01:00:00Z"}
  ]
}
```

---

## 5. Enterprise Monitoring, Fraud & Incident Response REST APIs

### 5.1 Get Component Health Statuses
* **Route:** `GET /api/v1/wallet/monitoring/health`
* **Access:** Private (Authenticated Administrator)
* **Response Output:**
```json
{
  "health_status": [
    {"component": "DATABASE", "status": "HEALTHY", "updated_at": "2026-08-03T02:00:00Z"},
    {"component": "KAFKA", "status": "HEALTHY", "updated_at": "2026-08-03T02:00:00Z"},
    {"component": "WALLET_CORE", "status": "HEALTHY", "updated_at": "2026-08-03T02:00:00Z"}
  ]
}
```

### 5.2 List Flagged Fraud Events
* **Route:** `GET /api/v1/wallet/monitoring/fraud`
* **Access:** Private (Authenticated Security Administrator)
* **Response Output:**
```json
{
  "fraud_events": [
    {
      "id": "frd_111",
      "user_id": "usr_99",
      "action_type": "WITHDRAWAL",
      "risk_score": 0.85,
      "details": "High risk geodistance travelling anomaly",
      "timestamp": "2026-08-03T03:00:00Z"
    }
  ]
}
```

### 5.3 Get System Incident List
* **Route:** `GET /api/v1/wallet/monitoring/incidents`
* **Access:** Private (Authenticated Security Administrator)
* **Response Output:**
```json
{
  "incidents": [
    {
      "id": "inc_111",
      "title": "Database degradation alert",
      "severity": "HIGH",
      "status": "RESOLVED",
      "details": "High latencies matched completely with DB lock queues",
      "timestamp": "2026-08-03T03:00:00Z"
    }
  ]
}
```

### 5.4 Get Operational Recovery History
* **Route:** `GET /api/v1/wallet/monitoring/recovery`
* **Access:** Private (Authenticated Administrator)
* **Response Output:**
```json
{
  "recovery_history": [
    {
      "id": "rec_111",
      "component": "DATABASE",
      "details": "Connection pool successfully refreshed and restored",
      "timestamp": "2026-08-03T03:30:00Z"
    }
  ]
}
```

### 5.5 Trigger Service Self-Healing Manual Recovery
* **Route:** `POST /api/v1/wallet/monitoring/recovery/trigger`
* **Access:** Private (Authenticated Administrator)
* **Payload:**
```json
{
  "component": "DATABASE"
}
```
* **Response Output:**
```json
{
  "message": "Automated service self-healing triggered successfully",
  "id": "rec_111",
  "component": "DATABASE",
  "status": "HEALTHY"
}
```
