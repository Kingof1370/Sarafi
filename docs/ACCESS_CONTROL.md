# Access Control Specification (RBAC & Permissions)

This document details the database-backed Role-Based Access Control (RBAC) and fine-grained endpoint authorization architecture of Velyxora.

## 1. Enterprise Role Hierarchy
Velyxora maps user access levels to a strictly defined, least-privilege role hierarchy:
1. `USER`
2. `VERIFIED_USER`
3. `VIP_USER`
4. `SUPPORT_AGENT`
5. `COMPLIANCE_OFFICER`
6. `SECURITY_OFFICER`
7. `ADMIN`
8. `SUPER_ADMIN`

---

## 2. Fine-Grained Database Permissions
Access rights are mapped to specific database permission strings rather than relying on high-level role names:
- `wallet:read`: Retrieve balance allocations, histories, and wallet addresses.
- `wallet:write`: Create withdrawal requests, request new wallet addresses.
- `trading:write`: Place limit/market orders, cancel orders, bulk cancel.
- `support:read`: Access support ticket systems and view revenue sheets.
- `risk:write`: Set global trading halts, block user identities, suspend market pairs.
- `system:admin`: Trigger secure database backups and manual settlements.
- `super:admin`: Trigger system disaster recovery replays.

---

## 3. Dynamic Server-Side Enforcement
The `RBACMiddleware` interceptor executes the following flow:
1. Pulls the `claims` payload from context (populated via JWT or HMAC verification).
2. Queries the `users` table to fetch the real-time status and role of the identity. Restricted/Suspended/Locked users are blocked immediately.
3. Queries the `role_permissions` table to check if the user's role has been granted the required permission string.
4. If authorized, lets the request proceed. Otherwise, returns `403 Forbidden`.
