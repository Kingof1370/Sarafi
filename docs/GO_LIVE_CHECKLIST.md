# Go-Live Checklist
## Velyxora Enterprise Exchange

Pre-flight checks to complete before launching to production.

- [ ] **Config / Secret Validation:** `APP_ENV=production` is verified, weak/default development secrets are disabled, and fail-closed checkers are active.
- [ ] **Docker Hardening:** No container runs as root. Minimal base images are active with clean signals (`STOPSIGNAL SIGTERM`).
- [ ] **Kubernetes Manifests:** Probes (liveness, readiness, startup) are assigned, and HPA scales workloads based on CPU.
- [ ] **Database Integrity:** No unencrypted DB connection parameters are active. PostgreSQL connection pools use secure TLS configurations.
- [ ] **Disaster Recovery:** Daily AES-256 backup schedules verified. Isolated schema and financial ledger restore tests executed successfully.
- [ ] **E2E Validation:** All registration, logging, MFA, trading, and balance ledger tests pass with race detector enabled.
