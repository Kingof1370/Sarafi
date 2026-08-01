# VELYXORA EXCHANGE - MATCHING ENGINE DESIGN
This manual details the deterministic Price-Time Priority matching engine core of Velyxora Exchange.

## Core Rules & Modifiers
1. **Price-Time Priority Queues:** Bids are sorted highest-to-lowest, and asks are sorted lowest-to-highest.
2. **Immediate Or Cancel (IOC):** Executes any immediate matching portion and cancels the remaining.
3. **Fill Or Kill (FOK):** Requires full immediate matching of the total order size, or cancels the entire order.
4. **Post Only:** Rejects any order that would execute immediately to ensure maker-only status.
5. **Stop Loss & Stop Limit:** Untriggered stops are held in a monitoring loop and entering limit books once the last trade price crosses the stop price.
6. **Recovery & Replay:** Every submitted or cancelled action is logged to an immutable sequence journal on disk, allowing complete crash state reconstructions.
