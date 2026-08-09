# Production Deployment Guide
## Velyxora Enterprise Exchange

This document details the production-grade deployment process for the Velyxora Enterprise Exchange platform on Kubernetes.

### 1. Prerequisites
- Kubernetes Cluster >= 1.28
- Helm v3
- PostgreSQL Instance with connection encryption (SSL) enforced
- Apache Kafka Cluster
- Redis Cluster

### 2. Manifest Deployments
1. **ConfigMap & Secrets:**
   ```bash
   kubectl apply -f infrastructure/kubernetes/configmap.yaml
   kubectl apply -f infrastructure/kubernetes/secrets.yaml
   ```
2. **Network Security & Policies:**
   ```bash
   kubectl apply -f infrastructure/kubernetes/network-policy.yaml
   ```
3. **Application Workloads:**
   ```bash
   kubectl apply -f infrastructure/kubernetes/api-gateway.yaml
   kubectl apply -f infrastructure/kubernetes/matching-engine.yaml
   kubectl apply -f infrastructure/kubernetes/wallet-service.yaml
   kubectl apply -f infrastructure/kubernetes/frontend.yaml
   ```
4. **Ingress & Routing:**
   ```bash
   kubectl apply -f infrastructure/kubernetes/ingress.yaml
   ```
5. **Autoscaling & Budgets:**
   ```bash
   kubectl apply -f infrastructure/kubernetes/hpa.yaml
   kubectl apply -f infrastructure/kubernetes/pdb.yaml
   ```

### 3. Sizing and Resources Sizing
- **api-gateway:** 3 Replicas (Min 3, Max 10). Memory Limit: 1024Mi. CPU Limit: 1.0.
- **matching-engine:** 1 Replica (Strictly single-writer). Memory Limit: 2048Mi. CPU Limit: 2.0.
- **wallet-service:** 2 Replicas (Min 2, Max 5). Memory Limit: 1024Mi. CPU Limit: 1.0.
- **frontend:** 2 Replicas. Memory Limit: 512Mi. CPU Limit: 500m.
