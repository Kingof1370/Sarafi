# Prompt: P13 — Infrastructure & Manifests Hardening

## 1. Objective
Ensure that all microservice Dockerfiles, Kubernetes manifests, HPAs, and PodDisruptionBudgets are fully hardened and production-ready.

## 2. Current Problem
- Overall overarching orchestration configurations (e.g. docker-compose) were missing from the codebase.

## 3. Root Cause
Lack of a single consolidated developer sandbox compose setup.

## 4. Affected Modules
- `infrastructure/`

## 5. Affected Files
- `infrastructure/docker/docker-compose.yml` (if created) or Dockerfiles

## 6. Architectural & Technical Requirements
- Configure microservice Dockerfiles to run as a dedicated, non-privileged user and group (`velyxuser` and `velyxgroup`) on top of minimal Alpine environments.
- Define explicit ServiceAccounts, resource constraints, liveness/readiness/startup probes inside Kubernetes manifests.

## 7. Migration & Backward Compatibility
- Backwards compatible with standard Kubernetes and container schedulers.

## 8. Security & Financial Requirements
- Ensure container runtimes have no access to host namespace permissions.

## 9. Tests & Failure Scenarios
- Confirm Dockerfiles compile successfully and produce lightweight, secure images.

## 10. Acceptance Criteria & Definition of Done
- No container runs under root user.
- Resource limits are fully specified.
- Completed with Git commit.
