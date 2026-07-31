# VELYXORA DEVELOPMENT GUIDE

This document serves as the onboarding manual for engineers contributing to **Velyxora Exchange**.

## Local Setup

### 1. Repository Setup & Go Modules
Initialize and sync Go workspaces:
```bash
go work init
go work use ./apps/backend ./apps/matching-engine ./apps/wallet-service ./packages/common ./packages/database ./packages/logger ./packages/security ./packages/types ./tests
```

### 2. Run Tests
Execute the entire test suite (including helper packages, services, and integration loops):
```bash
go test -v velyxora/packages/... velyxora/apps/... velyxora/tests/...
```

### 3. Frontend App Dev Server
Run the local next.js development target:
```bash
cd apps/frontend
npm run dev
```

---

## Code Standards
* Run standard formatting checks: `go fmt ./...`
* Linter validation: `go vet ./...`
* Always design code using strict interfaces to enable mocking in unit tests.
