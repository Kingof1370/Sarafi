# VELYXORA ARCHITECTURE DESIGN

This document outlines the distributed microservice design of **Velyxora Exchange**.

```
  +------------------+
  |  Next.js Client  |
  +--------+---------+
           |
           | HTTP REST / WebSockets
           v
  +------------------+
  |   API Gateway    | <===========> Redis (Rate Limit & Auth Cache)
  +--------+---------+
           |
           | Publish Order Event
           v
     [ Kafka Queue ] (velyxora-orders)
           |
           v
  +------------------+
  | Matching Engine  |  (In-Memory Price-Time Order Book Engine)
  +--------+---------+
           |
           | Publish Trade Match Event
           v
     [ Kafka Queue ] (velyxora-trades)
           |
           v
  +------------------+
  |  Wallet Service  | <===========> PostgreSQL (Atomic Settle Ledger)
  +------------------+
```

## System Breakdown

### 1. API Gateway (REST Interface)
* Serves as the primary public entry point for client apps.
* Validates user authentication via JWT.
* Handles request rate-limiting.
* Enforces request validation using JSON schemas.
* Routes critical actions (like order placement) directly to Apache Kafka to isolate high HTTP request spikes from backend matching processes.

### 2. High-Performance Matching Engine
* Runs as an isolated Go process with its own internal state machine.
* Subscribes to the `velyxora-orders` topic.
* Keeps all active order books directly in physical memory, utilizing highly optimized Price-Time priority queues.
* Never interacts directly with database systems to guarantee high performance and sub-millisecond execution.
* Emits trade matches to `velyxora-trades`.

### 3. Wallet and Ledger Service
* Subscribes to `velyxora-trades`.
* Manages user wallets and processes asset balances.
* Employs ACID-compliant database transaction locks inside PostgreSQL to prevent race conditions during balances updates.
