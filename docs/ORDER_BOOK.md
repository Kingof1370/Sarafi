# VELYXORA EXCHANGE - ORDER BOOK DESIGN
Velyxora's high-speed order book supports real-time bid and ask queues managed via the `Matcher`.

## Levels and Queues
- **Bids Sorting:** Sorted descending (highest buy prices first).
- **Asks Sorting:** Sorted ascending (lowest sell prices first).
- **FIFO Queues:** Within each price level, orders are executed strictly in the order they arrived (first-in, first-out).
- **Empty Level Cleanup:** Limits/price levels are immediately cleaned up and removed from asks/bids slices when their orders queue is depleted.
