# VELYXORA EXCHANGE

Welcome to the enterprise foundation build of **Velyxora Exchange** — a high-performance, ultra-secure, production-grade cryptocurrency exchange platform designed with architecture standards comparable to major global exchanges.

## Architecture & Design Goals

Velyxora is engineered using an autonomous, loosely-coupled microservice architecture designed to handle high-frequency trading with sub-millisecond execution times.

For a detailed exploration of our core systems, refer to:
* **[Architecture & Design Principles](docs/ARCHITECTURE.md)**
* **[Security Architecture Guidelines](docs/SECURITY.md)**
* **[Technical Stack Selection](docs/TECH_STACK.md)**

---

## Directory Layout

```bash
velyxora-exchange/
├── apps/
│   ├── backend/           # API Gateway / REST gateway using Gin-Gonic (Go)
│   ├── matching-engine/   # High-performance order-matching core (Go)
│   └── wallet-service/    # Double-entry transaction settling ledger (Go)
│   └── frontend/          # Advanced Next.js dashboard UI with React Query & Zustand
├── packages/
│   ├── common/            # Custom Kafka and Redis wrapped library pools
│   ├── database/          # pgx/v5 connection pool abstractions
│   ├── logger/            # slog structured contextual JSON logger
│   ├── security/          # Argon2/Bcrypt and JWT security tokens core
│   └── types/             # Domain entities and Kafka event models
├── infrastructure/
│   ├── docker/            # Dockerfiles & production compose setups
│   ├── kubernetes/        # Kubernetes resource manifests & Helm Charts
│   └── ci-cd/             # Continuous integration linting & build targets
└── docs/                  # Detailed engineering manuals
```

---

## Getting Started

### Local Development Prerequisites
Ensure the following components are installed locally on your development system:
* **Go** (v1.24.3+)
* **Node.js** (v22+) & **npm**
* **Docker** & **Docker Compose**

### Running via Docker Compose
To boot up the complete enterprise network containing Kafka, PostgreSQL, Redis, all backend microservices, and the frontend dashboard UI, execute:

```bash
docker compose -f infrastructure/docker/docker-compose.yml up --build -d
```

---

## Technical Documentation Manifest

Review the full suite of Velyxora engineering manuals in our `docs/` module directory:
* **[DEVELOPMENT.md](docs/DEVELOPMENT.md)**: Setup, build, test, and contribution workflows.
* **[DEPLOYMENT.md](docs/DEPLOYMENT.md)**: Standard operating instructions for Kubernetes and AWS environments.
* **[API_DOCUMENTATION.md](docs/API_DOCUMENTATION.md)**: Enterprise JWT session endpoints and trading core definitions.
