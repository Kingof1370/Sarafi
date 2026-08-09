# Velyxora - Distributed Tracing Specification

Velyxora propagates execution contexts across multiple services to enable request/event tracing.

## Trace context propagation
1. **HTTP Requests**: Handlers fetch and pass `X-Trace-ID` headers downstream.
2. **OpenTelemetry Span Integration**: Initiates OpenTelemetry trace spans with composite TextMapPropagators.
3. **Kafka Event Tracing**: Correlation context is injected inside Kafka message headers.
4. **Database & Memory Transactions**: Core actions pass `context.Context` carrying trace contexts down to relational query operations.
