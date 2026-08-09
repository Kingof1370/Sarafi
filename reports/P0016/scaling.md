# Velyxora Horizontal Scaling Architecture

## Scaling Taxonomy

Each service in Velyxora is categorized by its statefulness and scaling capabilities:

| Microservice | State | Horizontal Scale | Mechanism & Guardrails |
| --- | --- | --- | --- |
| API Gateway | Stateless | Yes | Can scale horizontally behind standard reverse proxy load balancer (Caddy, NGINX). |
| WebSocket Gateway | Stateless | Yes | Can scale horizontally with pub-sub backend (Redis) to propagate market messages to all endpoints. |
| Matching Engine | Stateful | No | Sharded by trading pairs/symbol. Multiple engines can run concurrently but each engine is the single writer for its allocated symbol. |
| Wallet Service | Stateful | No | Strictly follows P0014 single-writer ledger execution and recovery to avoid split-brain balances. |

## Guardrails Against Multi-Writer Hazards
To prevent double matching, duplicate settlements, or double withdrawals, all database mutations on user balances utilize transaction-scoped advisory locks and `SELECT FOR UPDATE` atomic locks.
