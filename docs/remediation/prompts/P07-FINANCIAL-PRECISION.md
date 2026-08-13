# Prompt: Financial Precision Audit & Decimal Migrations (PART-07)

## Objective
Remediet the extensive use of floating-point arithmetic (`float64`) for financial balances, trades, and order matching computations. Mismatches between floating point variables in Go and exact `DECIMAL(36,18)` columns in PostgreSQL create critical precision errors, dust accumulation, and rounding vulnerabilities. Transit the financial core to deterministic fixed-point or rational arithmetic.

## Scope
- Audit all instances of `float64` in critical financial pathways: `balances`, `prices`, `quantities`, `fees`, `PnL`, `margin`, `liquidations`, and `settlements`.
- Standardize high-precision mathematical operations inside the Matching Engine, Settlement Engine, and DB layer.
- Integrate precision calculation routines throughout execution and ledger records.

## Requirements & Specifications

### 1. High-Precision Variable Standard
- Unify high-precision calculations using either exact decimal arithmetic, fixed-point representations, or standard Go precision libraries (e.g., `github.com/shopspring/decimal`, integer-scaled representation, or `math/big.Rat` via `packages/common/precision.go`).
- Stop using float64 operations for basic additions, subtractions, and multiplications of balances, fees, or settlement values. All fee computations and trading rule validations (like ticks and step sizes) must employ exact rational/decimal arithmetic.

### 2. Database Integration
- Ensure all numbers fetched from the PostgreSQL `DECIMAL(36,18)` columns are parsed directly as high-precision types, rather than being parsed into Go float64 variables which lose precision.
- Ensure all balance modifications, double-entry ledger listings, and settle postings write the full, exact high-precision decimal representation directly into the database.

### 3. Dust Prevention & Edge Conditions
- Implement strict rounding guidelines (e.g., Round Down for buys, Round Down for sells, or deterministic truncation to 18 decimal places) to completely prevent negative balances or unexpected dust leaks.

## Testing Strategy
- Create rounding and financial precision tests:
  - Verify tick size validation: test with fractions like `0.000000000000000001` (1 Wei equivalent) and ensure no precision loss or incorrect rejects occur.
  - Test dust aggregation: execute 1,000 partial fills with non-divisible numbers (e.g., matching quantities of `0.333333333333333333`) and verify that the sum of all parts exactly matches the total balance without a single unit of desynchronization.
  - Assert that double-entry ledger checks verify down to the last decimal digit (18 decimal places).
