# VELYXORA TECHNICAL STACK

Velyxora is engineered with a strict, performance-first enterprise stack designed to meet the strict demands of high-frequency cryptocurrency execution.

## Core Language Stack

### 1. Go (Golang) — Performance Core Services
* **Where**: API Gateway, Matching Engine, and Wallet Ledger Service.
* **Why**: High concurrency throughput via goroutines, tiny memory footprints, memory safety, and predictable garbage collection cycles.

### 2. TypeScript / React / Next.js — Client Applications
* **Where**: Frontend Core Dashboard.
* **Why**: Strict compile-time typing, advanced server-side rendering for quick load times, and dynamic single-page features.

---

## Infrastructure and Persistence Layer

### 1. PostgreSQL (relational DB)
* **Usage**: Storing relational domain states (Users, API Keys, static balance ledger entries, and historical orders).
* **Driver**: `pgx/v5` connection pool (no heavy ORM) for high speed and raw query control.

### 2. Redis Cache
* **Usage**: High-speed lookup, rate limiting, and temporary state storage.
* **Driver**: `go-redis/v9`.

### 3. Apache Kafka (Distributed Messaging Broker)
* **Usage**: Event-driven streaming. Orders are immediately ingested into Kafka topics, assuring the matching engine never stalls.
* **Driver**: `kafka-go`.

---

## UI Component Layer
* **Tailwind CSS**: Rapid utility-first styling.
* **Zustand**: Fast, lightweight client-side state management.
* **React Query**: Robust server-side state synchronization with automatic caching.
