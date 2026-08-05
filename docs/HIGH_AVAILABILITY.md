# Velyxora Exchange — High Availability & Failure Recovery Manual

## 1. Overview
The Velyxora platform is built with high availability (HA) at its core, enabling continuous, zero-downtime operations even under hardware, node, or network failures.

The system ensures continuous operations through:
* **Stateless Service Scale-out:** API gateways and wallet services operate in active-active, horizontally scalable configurations.
* **Consensus-based Leader Election:** Dynamic clustering utilizes Raft-like voting terms to choose primary leader engines.
* **Graceful connection draining:** Safely drains active websockets and REST sockets before node terminations.

---

## 2. Automatic Failover Mechanism
In a clustered setup, nodes regularly report heartbeat pings to the Cluster Manager:
1. **Heartbeat Monitoring:** Signalling occurs every 100ms.
2. **Timeout Detection:** If the primary leader fails to heartbeat within 300ms, it is flagged as UNHEALTHY.
3. **Automated Re-election:** The node manager launches an instant re-election term, dynamically appointing the next healthy node.
4. **Partition Recovery:** Safe partitions rebalances avoid double-processing on message buses.
