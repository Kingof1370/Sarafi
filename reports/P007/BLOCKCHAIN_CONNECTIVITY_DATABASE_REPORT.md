# Velyxora Exchange — P007 Blockchain Connectivity Database Report

## Metadata
* **Execution Timestamp:** 2026-08-02 20:00:00 UTC
* **Git Commit Hash:** 29153eb

## Schema Design (Migrations 49 - 53)
We expanded the database with the following normalized relational schemas:
* `blockchain_nodes` (id (PK), network, url, type, is_active)
* `rpc_endpoints` (id (PK), node_id (FK), method_name, is_supported)
* `network_status` (network (PK), latest_block, status, latency_ms, updated_at)
* `blockchain_sync_history` (id (PK), network, block_height, block_hash, status, timestamp)
* `node_metrics` (id (PK), node_id (FK), latency_ms, failed_requests, health_score, timestamp)

## Relational Constraints & Indexes
* **Foreign Keys:** All dependent tables feature foreign keys linking to `blockchain_nodes.id` with `ON DELETE CASCADE` cascading deletes.
* **Indices:**
  - `idx_blockchain_nodes_net` on `blockchain_nodes (network)`
  - `idx_rpc_endpoints_node` on `rpc_endpoints (node_id)`
  - `idx_blockchain_sync_net` on `blockchain_sync_history (network)`
  - `idx_node_metrics_node` on `node_metrics (node_id)`
