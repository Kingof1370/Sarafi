# Scaling Policies
## Velyxora Enterprise Exchange

Guidelines for horizontal and vertical scaling.

### 1. Horizontal Scaling (Stateless)
- **API Gateway & Wallet Service:** Configured to dynamically scale out via Horizontal Pod Autoscaler (HPA) when CPU utilization averages >= 75-80%.
- **Frontend:** Statistically scaled across multiple nodes to maintain sub-second response times globally.

### 2. Vertical Scaling (Stateful)
- **Matching Engine:** Due to the single-writer sequencing requirements of high-frequency trading books, the Matching Engine scales *vertically* by allocating higher CPU and memory limits (Min: 1 CPU, Max: 2 CPUs).
