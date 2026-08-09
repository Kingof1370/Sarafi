# Velyxora - Observability Architecture

This document describes the unified observability architecture implemented across Velyxora core exchange services.

## Overview
Velyxora's observability framework provides comprehensive SRE insights into system health, API latency, transaction throughput, order-matching metrics, and compliance controls.

## Components
1. **Metrics Collection**: Powered by `github.com/prometheus/client_golang`, registering both SRE technical indicators and business-level metrics.
2. **Distributed Tracing**: Integrates with OpenTelemetry APIs for trace span and correlation context propagation across HTTP boundaries, Kafka events, and database actions.
3. **Structured Logging**: Uses Go's native `slog` library to log structured parameters with correlation context and automatic redaction of sensitive credentials.
4. **Disaster & Incident Alerting**: Background health-probes monitor PostgreSQL, Redis, and Kafka, raising live incidents inside `sre_incidents` tables on failure.
