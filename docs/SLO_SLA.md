# Velyxora - SRE SLO/SLA Definitions

This document defines the Service Level Objectives (SLOs) and Service Level Agreements (SLAs) for Velyxora.

## SRE Targets

| Indicator | Metric Target | Measurement Window | Failure Threshold |
|---|---|---|---|
| **API Availability** | >= 99.99% successful requests | 30 Days rolling | > 0.01% error rate |
| **API Latency** | <= 50ms (p95), <= 100ms (p99) | 24 Hours rolling | > 100ms p99 latency |
| **Order Acceptance** | <= 5ms (p99) latency | 1 Hour rolling | > 10ms p99 latency |
| **Matching Engine Latency** | <= 1ms (p99) latency | 1 Hour rolling | > 2ms p99 latency |
| **Kafka Processing Lag** | <= 50 messages delay | Real-time | > 200 messages delay |
| **Database Availability** | >= 99.999% uptime | 30 Days rolling | > 10s database offline |
