# P0009 AML Transaction Monitoring Report

## Active Detections
The transaction-monitoring engine implements configurable rules checking:
- **Large Transactions**: Triggered at transfers >= $5,000.
- **Velocity Thresholds**: Exceeding 5 transactions within 1 hour raises high severity alerts, publishing a Kafka message and restricting the account status to `MONITOR`.
- **Structuring Patterns**: Logged and associated with compliance cases.

All triggered alerts are fully immutable and written to the database with detailed evidence.
