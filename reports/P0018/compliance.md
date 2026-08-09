# P0018 Compliance, KYC, and AML Controls

## Status: PASS

## 1. Compliance Engine Architecture
The compliance engine (`packages/security/compliance.go`) is fully operational and acts as a strict gateway for all trading and withdrawal operations.

## 2. Rule Audits & Gates
* **KYC Tiers**: Limits are assigned dynamically depending on the verified KYC level (Basic, Standard, Advanced, Institutional).
* **Sanctions Screening**: The sanctions screener runs as a strict fail-closed module. Any network failure or provider timeout immediately halts transaction flows rather than allowing them to bypass watchlists.
* **AML velocity rules**: Evaluates transaction frequencies and volumes dynamically. Exceeding 5 transactions within 1 hour results in an automatic account restriction and triggers an alert.
* **Travel Rule**: For transactions $\geq$ 1000 USD/units, originator and beneficiary names are mandatory. Missing information immediately halts the withdrawal.
* **Case Management**: Active cases are registered in PostgreSQL with investigator notes, allowing manual resolution or account blocks.
