# Risk Scoring & Assessment Framework

Velyxora's Risk Scoring Engine calculates real-time, deterministic, and fully explainable risk profiles for every identity.

## Deterministic Scoring Logic
Every evaluation compiles contributing factors from different compliance dimensions:
- **Baseline Risk Score**: 0.10.
- **KYC Status**:
  - `VERIFIED`: Subtracts `0.05` risk points.
  - `PENDING`: Adds `0.10` risk points.
  - `REJECTED`/`SUSPENDED`: Adds `0.25` risk points.
- **Account Restrictions**:
  - Any restriction status adds `0.30` risk points.
- **Compliance Alerts History**:
  - Each unresolved alert adds `0.15` risk points.

The final score is strictly bounded inside `[0.0, 1.0]`.

## Explained Contributing Factors
Scores are never arbitrary. Every risk check logs a clear JSON document detailing the exact rules and values applied during evaluation, assuring auditable reproduction.
- **LOW Risk**: Score `<= 0.25`.
- **MEDIUM Risk**: Score `(0.25, 0.50]`.
- **HIGH Risk**: Score `(0.50, 0.75]`.
- **CRITICAL Risk**: Score `> 0.75`.
