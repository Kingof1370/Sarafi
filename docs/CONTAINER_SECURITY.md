# CONTAINER HARDENING DIRECTIVES

## 1. Minimal Alpine Run Images
- Container stages are designed with a two-step build pattern, compiling binaries inside high-performance Go-alpine builders, and running them inside clean Alpine environments.

## 2. Running as Non-Root User
- All production Dockerfiles create and run as a secure non-privileged `velyxuser` account.
- Linux capabilities are restricted, and processes are isolated from host namespace resources.

## 3. Secret Exclusions
- Credentials or sensitive API secrets are never baked directly into images.
- Secrets must be injected dynamically at runtime via secure environmental parameters or sidecar managers.
