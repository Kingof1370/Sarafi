# CONTAINER CONFIGURATION SECURITY REPORT (P0012)

## 1. Minimal Alpine Bases
- Standard Alpine images (`alpine:3.19`) are leveraged to reduce attack surface and avoid unnecessary system toolkits.

## 2. Non-Root Operations
- Hardened all microservice Dockerfiles:
  - Created a dedicated `velyxgroup` and non-root `velyxuser`.
  - Switched standard build run-stages to use `USER velyxuser`.

## 3. Findings
- Container configuration meets SOC2 & industry best practices.
