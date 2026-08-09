# Velyxora Horizontal Scaling Guide

This document covers best practices for horizontally scaling Velyxora services while avoiding split-brain or double-spending states.

## Horizontal Scaling Matrix

| Service | State Type | Scale Horizontally? | Coordination System |
| --- | --- | --- | --- |
| `api-gateway` | Stateless | Yes | NGINX/Caddy load balancing |
| `frontend` | Stateless | Yes | CDN / Standard routing |
| `matching-engine` | Stateful | Yes (by symbol) | Sharded by trading pairs in Kafka topics |
| `wallet-service` | Stateful | No (Single writer) | Uses transaction advisory locking |

## Safeguards against state collision
- All balance transfers and reservations utilize `SELECT FOR UPDATE` locks.
- Idempotency records tracking prevents duplicate execution of withdrawal or trading operations.
