# Velyxora Security Testing Guide

## Security Verification Architecture
Velyxora implements continuous automated security verification inside its standard test suites:
- **Unit Security Tests**: Validate password rules, TOTP computation steps, and signature verification.
- **Integration Security Tests**: Test lockout counters, concurrent sessions pruning, and JWT rotation.
- **Regression Security Suite**: Custom targeted testing verifying that all audited routes maintain rigorous authorization boundaries.

## Running the Security Regression Suite
To run the security tests under an isolated database and cache environment:

```bash
# Ensure local Postgres and Redis are active (via Docker)
docker compose -f infrastructure/docker/docker-compose.yml up -d postgres redis

# Execute the complete security test suite
go test -v -count=1 -run TestP0015SecurityRegression ./tests/...
```

## Defensive Checks Enforced by Tests
1. **Vertical Escalation Protection**: Verifies that standard users or support agents are blocked from calling withdraw requests.
2. **Horizontal Reading Gating**: Verifies that only authorized sessions can query sensitive balances.
3. **MFA Fail-Closed Validation**: Assures that any sensitive mutation request (like withdraw) on an MFA-enforced account fails if the token is invalid or absent.
