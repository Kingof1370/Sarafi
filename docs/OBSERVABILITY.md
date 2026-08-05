# Velyxora Exchange — Enterprise Observability & Tracing Platform

## 1. Overview
Enterprise monitoring requires complete observability of latencies, database connections, and trace transactions across the entire exchange pipeline.

---

## 2. Prometheus Metrics
Every microservice exposes a `/metrics` Prometheus target tracking:
* **System Metrics:** Host CPU load, resident memory size, and active Go goroutine counts.
* **Wallet Metrics:** Transactions throughput, average confirmations latencies, and limit checks volume.
* **Matching Metrics:** Core latency (sub-microsecond cycle rates), order volumes, and queue depth.

---

## 3. Distributed Tracing & OpenTelemetry
To audit request lifecycles across multiple decoupled services:
* Core endpoints propagate trace contexts via HTTP headers (W3C standard).
* OpenTelemetry tracers export trace span details to Jaeger or equivalent backends.
* Allows operators to isolate exact microsecond bottlenecks on transfers or orders matching pathways.
