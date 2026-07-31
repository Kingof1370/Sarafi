# VELYXORA API SPECIFICATION

This manual details our REST endpoints for authentication and trading operations.

## Host
* Local URL: `http://localhost:8080`
* Production URL: `https://api.velyxora.com`

---

## 1. Authentication Endpoints

### Register User
* **URL**: `/api/v1/auth/register`
* **Method**: `POST`
* **Request Payload**:
```json
{
  "email": "user@velyxora.com",
  "password": "super-secure-passphrase"
}
```
* **Response Status**: `201 Created`

### User Login (Authenticate)
* **URL**: `/api/v1/auth/login`
* **Method**: `POST`
* **Request Payload**:
```json
{
  "email": "user@velyxora.com",
  "password": "super-secure-passphrase"
}
```
* **Response Status**: `200 OK`
* **Response Payload**:
```json
{
  "access_token": "eyJhbGciOi...",
  "refresh_token": "eyJhbGciOi...",
  "expires_in": 900
}
```

---

## 2. Trading Endpoints (Requires `Authorization: Bearer <JWT_TOKEN>`)

### Place Order
* **URL**: `/api/v1/trading/orders`
* **Method**: `POST`
* **Request Payload**:
```json
{
  "symbol": "BTC-USDT",
  "side": "BUY",
  "type": "LIMIT",
  "price": 50000.0,
  "quantity": 0.5
}
```
* **Response Status**: `202 Accepted`
* **Response Payload**:
```json
{
  "message": "Order created and queued",
  "order": {
    "id": "ord_1700000000",
    "user_id": "usr_uuid_123",
    "symbol": "BTC-USDT",
    "side": "BUY",
    "type": "LIMIT",
    "price": 50000,
    "quantity": 0.5,
    "filled_quantity": 0,
    "status": "NEW"
  }
}
```
