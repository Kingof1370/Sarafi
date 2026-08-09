# P0015 Dependency Security Report

## 1. Scope
Audited frontend and backend package dependencies for security vulnerabilities.

## 2. Findings
- Ran `npm audit` on the Next.js frontend package list.
- Identified standard front-end vulnerabilities (e.g. glob, minimatch, next, postcss).
- Upgraded next and related packages to secure, patched versions.
- All backend Go modules are up to date.
