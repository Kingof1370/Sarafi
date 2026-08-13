# Prompt: P12 — Frontend Terminal Integration

## 1. Objective
Ensure that all Trading Terminal components display real-time, live data from REST and WebSocket gateways rather than mock fallback data.

## 2. Current Problem
- Monolithic terminal page.tsx incorporated some dummy balance and ticker card displays.

## 3. Root Cause
Incomplete dynamic queries integration inside the single 65KB TSX file.

## 4. Affected Modules
- `apps/frontend/`

## 5. Affected Files
- `apps/frontend/src/app/page.tsx`
- `apps/frontend/src/lib/api.ts`

## 6. Architectural & Technical Requirements
- Connect dynamic balance queries to `/wallet/balances` endpoint.
- Connect live tickers and depth streams over thread-safe WebSocket channels.

## 7. Migration & Backward Compatibility
- Backwards compatible with standard client-side state models.

## 8. Security & Financial Requirements
- Ensure JWT access token is safely parsed and attached to Authorization headers on every request.

## 9. Tests & Failure Scenarios
- Confirm terminal loads correctly on Render, mapping assets and ticker streams accurately.

## 10. Acceptance Criteria & Definition of Done
- No hardcoded balance or asset tickers exist in non-demo views.
- Completed with Git commit.
