# VELYXORA API SPECIFICATION

This manual details our REST endpoints for authentication and trading operations.

## Host
* Local URL: `http://localhost:8080`
* Production URL: `https://api.velyxora.com`

---

## 1. Authentication & MFA Endpoints

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
* **Response Status**: `200 OK` (Or returns verification challenge if MFA active)
* **Response Payload (MFA Inactive)**:
```json
{
  "access_token": "eyJhbGciOi...",
  "refresh_token": "eyJhbGciOi...",
  "expires_in": 900
}
```
* **Response Payload (MFA Active)**:
```json
{
  "mfa_required": true,
  "mfa_token": "mfa_sec_xxxx",
  "message": "Multi-Factor Authentication required to finalize session"
}
```

### Finalize MFA Login Verification
* **URL**: `/api/v1/auth/mfa-verify`
* **Method**: `POST`
* **Request Payload**:
```json
{
  "mfa_token": "mfa_sec_xxxx",
  "totp_code": "000000"
}
```
* **Response Status**: `200 OK`

### Get MFA Status (Requires `Authorization`)
* **URL**: `/api/v1/mfa/status`
* **Method**: `GET`
* **Response Payload**:
```json
{
  "is_mfa_enabled": true
}
```

### Initiate MFA Enrollment (Requires `Authorization`)
* **URL**: `/api/v1/mfa/enable`
* **Method**: `POST`
* **Response Payload**:
```json
{
  "mfa_secret": "JBSWY3DPEHPK3PXP",
  "qr_code_url": "otpauth://totp/...",
  "backup_codes": ["1234-5678", "abcd-efgh", "9876-5432"]
}
```

### Confirm MFA Enrollment Activation (Requires `Authorization`)
* **URL**: `/api/v1/mfa/verify`
* **Method**: `POST`
* **Request Payload**:
```json
{
  "totp_code": "000000"
}
```
* **Response Status**: `200 OK`

### Disable MFA Protection (Requires `Authorization`)
* **URL**: `/api/v1/mfa/disable`
* **Method**: `POST`
* **Request Payload**:
```json
{
  "totp_code": "000000"
}
```
* **Response Status**: `200 OK`

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
