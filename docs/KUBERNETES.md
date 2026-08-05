# Velyxora Exchange — Kubernetes Production Orchestration

## 1. Overview
We orchestrate and scale our microservices inside production-grade Kubernetes clusters, using Horizontal Pod Autoscalers (HPA), Pod Disruption Budgets (PDB), and spread anti-affinity rules to enforce zero single points of failure.

---

## 2. Cluster Topology & HA Scaling

### 2.1 Horizontal Pod Autoscaling (HPA)
The platform autoscales nodes dynamically based on real-time loads:
* Trigger: CPU utilization exceeds **75%**.
* Scale Bounds: Minimum **2** replicas (ensuring HA), scaling up to **10** replicas under load.
* Target: Applied on API gateways and wallet services.

### 2.2 Pod Disruption Budgets (PDB)
Ensures core service availability during planned node upgrades or maintenance:
* Enforces `minAvailable: 1` for `wallet-service` and `api-gateway`, ensuring at least one healthy instance is always online.

### 2.3 Anti-Affinity Spreading
To eliminate hardware rack or node level single points of failure:
* Replicas leverage pod anti-affinity rules matching labels.
* Ensures identical microservices pods are never scheduled on the same host node.
