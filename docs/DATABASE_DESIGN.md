# VELYXORA EXCHANGE - DATABASE DESIGN
Our extended relational schema supports complete trading lifecycle data storage with high consistency constraints.

## SQL Relational Tables
1. **`orders`**: Active order parameters (StopPrice, Iceberg size, TIF).
2. **`order_history`**: Finalized historical orders.
3. **`order_events`**: High-frequency trade events.
4. **`order_audits`**: Immutable audit logs of state transitions.
