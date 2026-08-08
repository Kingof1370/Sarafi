# Disaster Recovery & Resiliency Verification testing Procedures

RESILIENCY verification procedures for developers and security auditors.

## 1. Unit resiliencies testing

- Run `go test velyxora/packages/security -run TestDisasterRecoverySuite -v`
- Renders:
  - Mode transition validations.
  - Multi-owner distributed locks acquisitions and expirations.
  - Verification of backups and restores.

## 2. Integration fault injections testing

To verify fail-closed and resiliency behaviors under real conditions:
1. Boot the cluster: `docker-compose up -d`.
2. Kill PostgreSQL process: `docker stop velyxora-postgres`.
3. Attempt to invoke backup: `POST /api/v1/system/backups`.
4. Verify the backup fails cleanly with connection error (Does not silently succeed).
