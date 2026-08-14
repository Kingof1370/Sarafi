# Remediation Dependency Graph (PHASE 2)

This document maps out the logical and structural dependencies of all 15 remediation prompts. This sequence ensures no architectural conflict or compilation regression can occur.

```
                  +-----------------------------------------+
                  |  P04: Canonical Order Pipeline          |
                  +--------------------|--------------------+
                                       | (Required standard order struct)
                                       v
                  +-----------------------------------------+
                  |  P03: Multi-Market Matching Engine      |
                  +--------------------|--------------------+
                                       | (Provides isolated books)
                                       v
                  +-----------------------------------------+
                  |  P05: Canonical Settlement Path         |
                  +--------------------|--------------------+
                                       | (Allows atomic transaction matching)
                                       v
                  +-----------------------------------------+
                  |  P06: Durable Settlement Queue          |
                  +--------------------|--------------------+
                                       | (Secures persistent databaseoutbox)
                                       v
                  +-----------------------------------------+
                  |  P07: Ledger & Financial Precision      |
                  +--------------------|--------------------+
                                       | (Transitions core arithmetic to big.Rat)
                                       v
                  +-----------------------------------------+
                  |  P02: Wallet & Cryptographic Keys       |
                  +--------------------|--------------------+
                                       | (Enables BIP-44 key trees)
                                       v
                  +-----------------------------------------+
                  |  P08: Security & Production Hardening   |
                  +--------------------|--------------------+
                                       | (Secures dynamic TOTP & Fail-Closed)
                                       v
                  +-----------------------------------------+
                  |  P01: Architecture & Structural Cleanup |
                  +--------------------|--------------------+
                                       | (Dismantles dead/unused modules)
                                       v
                  +-----------------------------------------+
                  |  P09-P15: Integration, Frontend & E2E  |
                  +-----------------------------------------+
```

## Description of Sequencing

1. **P04 (Canonical Order Schema)** serves as the base since all matching, validations, and events route through types.Order structs.
2. **P03 (Multi-Symbol matching)** builds upon the aligned schema to handle concurrent, isolated order books and registries.
3. **P05 (Single canonical settlement)** aligns balance mutations with the isolated matcher executions.
4. **P06 (Durable Queue outbox)** persists the unified executions.
5. **P07 (Precision)** upgrades the balance, ledger, and fee arithmetic to rational fields.
6. **P02 (HD derivation)** secures private/public key mappings.
7. **P08 (Production Hardening)** secures configuration parameters.
8. **P01 (Architecture cleanup)** removes dead modules and circular imports.
9. **P09-P15 (Integrations)** connects backend WS APIs to frontend visual widgets and executes full automated regression test suites.
