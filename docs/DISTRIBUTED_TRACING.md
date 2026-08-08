# Velyxora Request Tracing Propagation

To guarantee visibility across distributed services, Velyxora propagates trace correlation identifiers.

## Propagation Strategy
1. **Trace Capture**: A request enters Velyxora API Gateway, and the gateway searches for an incoming `X-Trace-ID` header. If absent, a unique correlation ID is generated.
2. **Context Passing**: The trace ID is stored in the Gin context and Go context as `trace_id`.
3. **Kafka Injection**: Trace IDs are passed inside downstream versioned Kafka payloads, allowing background workers to preserve tracing context.
