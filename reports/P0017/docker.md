# P0017 Docker Report

## Docker Hardening Overview
Every service Dockerfile in Velyxora has been verified and hardened for enterprise security.

## Verification Facts
- **Non-Root Execution:** Evaluated and upgraded Go container builds to run as `velyxuser:velyxgroup` and node builds to run as `node:node`.
- **Multi-Stage Builds:** Multi-stage builds are active across Go and Next.js services.
- **Graceful Shutdown:** `STOPSIGNAL SIGTERM` is explicitly configured to ensure graceful shutdowns.
- **Compilations:** Successfully built and verified `velyxora/api-gateway:latest` locally.
- **Status:** **PASS**
