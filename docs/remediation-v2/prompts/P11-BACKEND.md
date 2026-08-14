# Prompt: P11 — Backend REST & WS Gateway

## 1. Objective
Ensure thread-safe WebSocket connections, proper RBAC permission checks, and active slow-consumer protection.

## 2. Current Problem
- Concurrent WebSocket writes on connection channels could trigger crashes if not properly serialized.

## 3. Root Cause
Lack of thread-safe locking wrappers on connection write methods.

## 4. Affected Modules
- `apps/backend/`

## 5. Affected Files
- `apps/backend/main.go`
- `apps/backend/websocket.go`

## 6. Architectural & Technical Requirements
- Utilize thread-safe locking wrappers to serialize on-connection transmissions.
- Enforce strict HSTS, X-Frame-Options: DENY, and CSP headers.

## 7. Migration & Backward Compatibility
- Compatible with standard WebSocket connection profiles.

## 8. Security & Financial Requirements
- Enforce RBAC permission checks (`wallet:read` and `wallet:write`) on all sensitive routes.

## 9. Tests & Failure Scenarios
- Mock concurrent websocket writers. Verify that connection writes execute without panics.

## 10. Acceptance Criteria & Definition of Done
- WebSocket connections are thread-safe and secured against Slow-consumers.
- Completed with Git commit.
