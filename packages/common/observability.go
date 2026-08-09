package common

import (
	"context"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// ObservabilityManager coordinates SRE technical and business metrics, as well as tracing
type ObservabilityManager struct {
	Tracer trace.Tracer
	Registry *prometheus.Registry

	// SRE Metrics - Technical
	HTTPRequestsTotal      *prometheus.CounterVec
	HTTPErrorsTotal        *prometheus.CounterVec
	HTTPLatencySeconds     *prometheus.HistogramVec
	AuthFailuresTotal      *prometheus.CounterVec
	RateLimitEventsTotal   *prometheus.CounterVec
	APIKeyFailuresTotal    *prometheus.CounterVec
	WSConnectionsActive    prometheus.Gauge
	WSConnectionsTotal     *prometheus.CounterVec
	KafkaProducerFailures  *prometheus.CounterVec
	KafkaConsumerErrors    *prometheus.CounterVec
	KafkaConsumerLag       *prometheus.GaugeVec
	RedisFailuresTotal     *prometheus.CounterVec
	PostgresLatencySeconds *prometheus.HistogramVec
	PostgresErrorsTotal    *prometheus.CounterVec
	MatchingLatencySeconds *prometheus.HistogramVec
	BlockchainRPCFailures  *prometheus.CounterVec
	BlockchainConfLatency  *prometheus.HistogramVec
	ComplianceAlertsTotal  *prometheus.CounterVec
	SecurityIncidentsTotal *prometheus.CounterVec

	// Business Metrics
	OrdersSubmitted *prometheus.CounterVec
	OrdersAccepted  *prometheus.CounterVec
	OrdersRejected  *prometheus.CounterVec
	OrdersMatched   *prometheus.CounterVec
	OrdersCancelled *prometheus.CounterVec
	TradingVolume   *prometheus.CounterVec
	TradeCount      *prometheus.CounterVec
	ActiveSymbols   *prometheus.GaugeVec
	ActiveUsers     prometheus.Gauge
	WalletOpsTotal  *prometheus.CounterVec
	DepositStatus   *prometheus.CounterVec
	WithdrawalStatus *prometheus.CounterVec
	AMLAlertsTotal  *prometheus.CounterVec
	KYCStatusChanges *prometheus.CounterVec
	SecurityEvents  *prometheus.CounterVec

	// Resource Metrics
	CPUUsageGauge        prometheus.Gauge
	MemoryAllocGauge     prometheus.Gauge
	NumGoroutinesGauge   prometheus.Gauge
	FileDescriptorsGauge prometheus.Gauge
}

var (
	globalOM *ObservabilityManager
	omOnce   sync.Once
)

// GetObservabilityManager returns or initializes the global manager
func GetObservabilityManager() *ObservabilityManager {
	omOnce.Do(func() {
		reg := prometheus.NewRegistry()

		// 1. Technical SRE metrics
		httpRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_http_requests_total",
			Help: "Total number of HTTP requests processed",
		}, []string{"method", "path", "status"})

		httpErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_http_errors_total",
			Help: "Total number of HTTP requests returning error status",
		}, []string{"method", "path", "status_class"})

		httpLatency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "velyxora_http_latency_seconds",
			Help:    "Latency of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"})

		authFailures := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_auth_failures_total",
			Help: "Total number of failed authentication and MFA attempts",
		}, []string{"type"}) // "jwt", "mfa", "rbac"

		rateLimitEvents := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_rate_limit_events_total",
			Help: "Total number of requests blocked by rate limiter",
		}, []string{"route"})

		apiKeyFailures := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_api_key_failures_total",
			Help: "Total number of invalid API Key requests",
		}, []string{"reason"})

		wsActive := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "velyxora_ws_connections_active",
			Help: "Currently active WebSocket connections",
		})

		wsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_ws_connections_total",
			Help: "Total websocket connect/disconnect events",
		}, []string{"event"}) // "connect", "disconnect"

		kafkaProducerFailures := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_kafka_producer_failures_total",
			Help: "Total Kafka publish/producer failures",
		}, []string{"topic"})

		kafkaConsumerErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_kafka_consumer_errors_total",
			Help: "Total Kafka subscription/consumer errors",
		}, []string{"topic", "group_id"})

		kafkaConsumerLag := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "velyxora_kafka_consumer_lag_bytes",
			Help: "Current Kafka consumer lag in byte size or messages",
		}, []string{"topic", "group_id"})

		redisFailures := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_redis_failures_total",
			Help: "Total failures connecting to or querying Redis",
		}, []string{"operation"})

		postgresLatency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "velyxora_postgres_query_latency_seconds",
			Help:    "PostgreSQL query latency in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"query_type"}) // "select", "insert", "update", "delete", "tx"

		postgresErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_postgres_errors_total",
			Help: "Total database operation errors",
		}, []string{"query_type", "error_category"})

		matchingLatency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "velyxora_matching_latency_seconds",
			Help:    "Order intake, validation and matching latencies in the engine",
			Buckets: prometheus.DefBuckets,
		}, []string{"stage", "symbol"}) // "intake", "validation", "matching"

		blockchainRPCFailures := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_blockchain_rpc_failures_total",
			Help: "Total blockchain RPC node connection and query failures",
		}, []string{"network"})

		blockchainConfLatency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "velyxora_blockchain_confirmation_latency_seconds",
			Help:    "Blockchain block confirmation detection latency in seconds",
			Buckets: prometheus.LinearBuckets(1, 2, 10),
		}, []string{"network"})

		complianceAlerts := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_compliance_alerts_total",
			Help: "Total system compliance triggered alerts",
		}, []string{"rule_triggered", "severity"})

		securityIncidents := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_security_incidents_total",
			Help: "Total system security incidents flagged",
		}, []string{"category", "severity"})

		// 2. Business metrics
		ordersSubmitted := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_orders_submitted_total",
			Help: "Total orders submitted by users",
		}, []string{"symbol", "side", "type"})

		ordersAccepted := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_orders_accepted_total",
			Help: "Total orders accepted by matching engine",
		}, []string{"symbol", "side"})

		ordersRejected := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_orders_rejected_total",
			Help: "Total orders rejected by matching engine or risk validation",
		}, []string{"symbol", "reason"})

		ordersMatched := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_orders_matched_total",
			Help: "Total buy/sell orders fully or partially matched",
		}, []string{"symbol"})

		ordersCancelled := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_orders_cancelled_total",
			Help: "Total order cancellations",
		}, []string{"symbol", "reason"})

		tradingVolume := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_trading_volume_units_total",
			Help: "Total matched asset trading volume in unit precision",
		}, []string{"symbol"})

		tradeCount := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_trade_count_total",
			Help: "Total executed trades",
		}, []string{"symbol"})

		activeSymbols := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "velyxora_active_symbols",
			Help: "Currently listed active symbols",
		}, []string{"symbol"})

		activeUsers := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "velyxora_active_users_count",
			Help: "Currently active authenticated sessions in Postgres/Redis",
		})

		walletOps := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_wallet_operations_total",
			Help: "Total deposits or withdrawals requested",
		}, []string{"operation", "asset"}) // "deposit", "withdrawal"

		depositStatus := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_deposit_status_total",
			Help: "Total deposit state changes recorded",
		}, []string{"asset", "status"}) // "detected", "confirming", "credited", "held"

		withdrawalStatus := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_withdrawal_status_total",
			Help: "Total withdrawal state changes recorded",
		}, []string{"asset", "status"}) // "pending", "approved", "broadcasted", "confirmed", "failed"

		amlAlerts := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_aml_alerts_total",
			Help: "Total transaction monitoring AML alerts",
		}, []string{"rule_triggered"})

		kycStatusChanges := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_kyc_status_changes_total",
			Help: "Total KYC state transitions",
		}, []string{"tier", "status"})

		securityEvents := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "velyxora_security_events_total",
			Help: "Total high-critical security events recorded",
		}, []string{"event_type", "severity"})

		// 3. Resource metrics
		cpuGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "velyxora_process_cpu_usage_pct",
			Help: "Process CPU utilization percentage",
		})

		memGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "velyxora_process_memory_alloc_bytes",
			Help: "Process active allocated memory in bytes",
		})

		goroutinesGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "velyxora_process_goroutines_count",
			Help: "Active goroutines count in runtime",
		})

		fdGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "velyxora_process_file_descriptors_open",
			Help: "Open file descriptor count",
		})

		// Register standard metrics
		reg.MustRegister(httpRequests, httpErrors, httpLatency, authFailures, rateLimitEvents, apiKeyFailures, wsActive, wsTotal)
		reg.MustRegister(kafkaProducerFailures, kafkaConsumerErrors, kafkaConsumerLag, redisFailures, postgresLatency, postgresErrors)
		reg.MustRegister(matchingLatency, blockchainRPCFailures, blockchainConfLatency, complianceAlerts, securityIncidents)
		reg.MustRegister(ordersSubmitted, ordersAccepted, ordersRejected, ordersMatched, ordersCancelled, tradingVolume, tradeCount)
		reg.MustRegister(activeSymbols, activeUsers, walletOps, depositStatus, withdrawalStatus, amlAlerts, kycStatusChanges, securityEvents)
		reg.MustRegister(cpuGauge, memGauge, goroutinesGauge, fdGauge)

		// Setup local tracer provider
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
		)
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

		globalOM = &ObservabilityManager{
			Tracer:                 tp.Tracer("velyxora-observability"),
			Registry:               reg,
			HTTPRequestsTotal:      httpRequests,
			HTTPErrorsTotal:        httpErrors,
			HTTPLatencySeconds:     httpLatency,
			AuthFailuresTotal:      authFailures,
			RateLimitEventsTotal:   rateLimitEvents,
			APIKeyFailuresTotal:    apiKeyFailures,
			WSConnectionsActive:    wsActive,
			WSConnectionsTotal:     wsTotal,
			KafkaProducerFailures:  kafkaProducerFailures,
			KafkaConsumerErrors:    kafkaConsumerErrors,
			KafkaConsumerLag:       kafkaConsumerLag,
			RedisFailuresTotal:     redisFailures,
			PostgresLatencySeconds: postgresLatency,
			PostgresErrorsTotal:    postgresErrors,
			MatchingLatencySeconds: matchingLatency,
			BlockchainRPCFailures:  blockchainRPCFailures,
			BlockchainConfLatency:  blockchainConfLatency,
			ComplianceAlertsTotal:  complianceAlerts,
			SecurityIncidentsTotal: securityIncidents,
			OrdersSubmitted:        ordersSubmitted,
			OrdersAccepted:         ordersAccepted,
			OrdersRejected:         ordersRejected,
			OrdersMatched:          ordersMatched,
			OrdersCancelled:        ordersCancelled,
			TradingVolume:          tradingVolume,
			TradeCount:             tradeCount,
			ActiveSymbols:          activeSymbols,
			ActiveUsers:            activeUsers,
			WalletOpsTotal:         walletOps,
			DepositStatus:          depositStatus,
			WithdrawalStatus:       withdrawalStatus,
			AMLAlertsTotal:         amlAlerts,
			KYCStatusChanges:       kycStatusChanges,
			SecurityEvents:         securityEvents,
			CPUUsageGauge:          cpuGauge,
			MemoryAllocGauge:       memGauge,
			NumGoroutinesGauge:     goroutinesGauge,
			FileDescriptorsGauge:   fdGauge,
		}
	})

	return globalOM
}

// Handler returns standard HTTP prometheus metrics server handler
func (om *ObservabilityManager) Handler() http.Handler {
	return promhttp.HandlerFor(om.Registry, promhttp.HandlerOpts{
		Registry: om.Registry,
	})
}

// StartTrace starts a named OpenTelemetry trace span with parent propagation from ctx
func (om *ObservabilityManager) StartTrace(ctx context.Context, name string) (context.Context, trace.Span) {
	return om.Tracer.Start(ctx, name)
}

// InjectTrace propagation helper injects context to carrier format (e.g. MapCarrier)
func (om *ObservabilityManager) InjectTrace(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// ExtractTrace propagation helper extracts context from carrier format
func (om *ObservabilityManager) ExtractTrace(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// MapCarrier is a simple helper implementing OpenTelemetry propagation.TextMapCarrier
type MapCarrier map[string]string

func (m MapCarrier) Get(key string) string {
	return m[key]
}

func (m MapCarrier) Set(key string, value string) {
	m[key] = value
}

func (m MapCarrier) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
