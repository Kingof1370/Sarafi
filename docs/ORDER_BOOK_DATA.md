# Order Book Consistency and Sequences

Velyxora enforces a strict sequence tracking protocol across the Matching Engine, database, and market feeds.

## Monotonic Sequence Tracking
Every trading symbol maintains a unique, monotonically increasing order-book sequence number. This sequence number increments on:
- Order placement
- Match/Trade execution
- Order cancellation

## Resynchronization
If a client detects a sequence number gap (e.g., sequence jumps from `N` to `N+2` instead of `N+1`), they must discard current increments, pull a fresh full snapshot via REST, and resume applying subsequent updates.
