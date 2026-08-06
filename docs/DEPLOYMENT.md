# VELYXORA DEPLOYMENT MANUAL

Velyxora is designed with containerization at its core, allowing for flexible cloud deployments.

## 1. Single Server / Dev Deployments (Docker Compose)
To set up a local testing environment containing PostgreSQL, Kafka, and the required microservices, run:
```bash
docker compose -f infrastructure/docker/docker-compose.yml up --build -d
```

## 2. Enterprise Kubernetes Deployment

We provide highly scalable Kubernetes deployment manifests in `infrastructure/kubernetes/`.

### Namespace Setup
We recommend placing all services inside a dedicated namespace:
```bash
kubectl create namespace velyxora
```

### Apply Manifests
Deploy the API Gateway, Matching Engine, Wallet Service, and Frontend interface:
```bash
kubectl apply -f infrastructure/kubernetes/
```

### Helm Charts
Alternatively, you can package and deploy via Helm:
```bash
helm upgrade --install velyxora ./infrastructure/kubernetes/helm-chart
```
