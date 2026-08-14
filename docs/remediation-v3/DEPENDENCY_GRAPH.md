# Dependency Graph — Remediation v3

```text
P01 Architecture / runtime wiring
 ├─> P03 Multi-symbol matching ──> P04 Order pipeline ──> P05 Settlement
 │                                                          ├─> P06 Durable settlement
 │                                                          └─> P07 Ledger + precision
 │                                                                 └─> P10 Risk/Margin
 ├─> P08 Security & MFA ─────────────────────────────────────────────┐
 └─> P11 Backend API/WS <────────────────────── P10 ─────────────────┘
        └─> P12 Frontend integration

P02 Wallet & cryptography ──> P09 Blockchain ──> P10 Risk/Margin

P13 Infrastructure  (depends on the final service graph from P01/P11)
P14 Integration / E2E / failure testing (depends on P01..P13)
P15 Final audit (depends on P14)
```

Parallelisable once P01 lands: {P02 → P09} and {P08} are independent of the
matching/settlement chain {P03 → P04 → P05 → P06 → P07 → P10}.
Serialisation points: P05 before P06/P07; P07 before P10; P11 before P12; P14 before P15.
