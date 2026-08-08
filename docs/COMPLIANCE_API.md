# Compliance REST API Specifications

The Velyxora API Gateway exposes the following secure compliance endpoints.

## User-Facing Endpoints
- **Get KYC Status**: `GET /api/v1/compliance/kyc/status`
- **Submit KYC**: `POST /api/v1/compliance/kyc/submit`
- **Get Restrictions**: `GET /api/v1/compliance/restrictions`
- **Get Self Risk Score**: `GET /api/v1/compliance/risk`

## Administrative Endpoints (RBAC `risk:write` Required)
- **Get Alerts**: `GET /api/v1/compliance/alerts`
- **Get Alert Details**: `GET /api/v1/compliance/alerts/:id`
- **Get Cases**: `GET /api/v1/compliance/cases`
- **Get Case Details**: `GET /api/v1/compliance/cases/:id`
- **Create Case**: `POST /api/v1/compliance/cases`
- **Update Case Status**: `PATCH /api/v1/compliance/cases/:id`
- **Get Any User Risk**: `GET /api/v1/compliance/risk?user_id=:user_id`

## Deposit Mock Simulator
- **Inbound Mock Deposit**: `POST /api/v1/wallet/deposits/mock`
- **Deposit History**: `GET /api/v1/wallet/deposits/history`
