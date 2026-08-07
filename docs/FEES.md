# VELYXORA EXCHANGE - FEES ENGINE
Calculates, routes, and accumulates dynamic trade fees.

## Rules and Rounding
- **Classification:** Categorizes trades into Maker and Taker execution roles.
- **VIP Discounts:** Automatically scales maker/taker fee rates across 4 VIP Tiers based on 30-day volume.
- **Affiliate Splits:** Allocates standard 20% referrer cuts and 80% platform revenues.
- **Rounding:** Strictly rounds calculations to 8 decimal places using decimal-safe rounding.
