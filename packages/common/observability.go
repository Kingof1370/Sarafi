package common

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// DependencyStatus represents status of a single dependency
type DependencyStatus string

const (
	StatusLive     DependencyStatus = "LIVE"
	StatusReady    DependencyStatus = "READY"
	StatusDegraded DependencyStatus = "DEGRADED"
	StatusNotReady DependencyStatus = "NOT_READY"
)

// MetricType represents counter, gauge or histogram type
type MetricType string

const (
	MetricCounter   MetricType = "COUNTER"
	MetricGauge     MetricType = "GAUGE"
	MetricHistogram MetricType = "HISTOGRAM"
)

// MetricValue represents a single metric sample point
type MetricValue struct {
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// ObservabilityManager coordinates unified metrics, health statuses, and request-tracing
type ObservabilityManager struct {
	mu            sync.RWMutex
	metrics       map[string]*MetricValue
	counters      map[string]*uint64
	serviceName   string
	startedAt     time.Time
}

var (
	globalObsInstance *ObservabilityManager
	globalObsOnce     sync.Once
)

// GetObservabilityManager returns or initializes a global singleton for performance metrics tracking
func GetObservabilityManager() *ObservabilityManager {
	globalObsOnce.Do(func() {
		globalObsInstance = &ObservabilityManager{
			metrics:     make(map[string]*MetricValue),
			counters:    make(map[string]*uint64),
			serviceName: "velyxora-core",
			startedAt:   time.Now(),
		}
	})
	return globalObsInstance
}

// SetServiceName sets service identification for current instance
func (om *ObservabilityManager) SetServiceName(name string) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.serviceName = name
}

// IncrementCounter atomically increments a counter metric safely
func (om *ObservabilityManager) IncrementCounter(name string, labels map[string]string) {
	om.mu.Lock()
	defer om.mu.Unlock()

	key := name
	for k, v := range labels {
		key += fmt.Sprintf("_%s_%s", k, v)
	}

	ptr, exists := om.counters[key]
	if !exists {
		var val uint64
		ptr = &val
		om.counters[key] = ptr
	}
	atomic.AddUint64(ptr, 1)

	om.metrics[key] = &MetricValue{
		Name:      name,
		Type:      MetricCounter,
		Value:     float64(atomic.LoadUint64(ptr)),
		Labels:    labels,
		Timestamp: time.Now(),
	}
}

// SetGauge updates a gauge metric
func (om *ObservabilityManager) SetGauge(name string, val float64, labels map[string]string) {
	om.mu.Lock()
	defer om.mu.Unlock()

	key := name
	for k, v := range labels {
		key += fmt.Sprintf("_%s_%s", k, v)
	}

	om.metrics[key] = &MetricValue{
		Name:      name,
		Type:      MetricGauge,
		Value:     val,
		Labels:    labels,
		Timestamp: time.Now(),
	}
}

// ObserveHistogram adds a sample value to a metric distribution
func (om *ObservabilityManager) ObserveHistogram(name string, val float64, labels map[string]string) {
	om.mu.Lock()
	defer om.mu.Unlock()

	key := name
	for k, v := range labels {
		key += fmt.Sprintf("_%s_%s", k, v)
	}

	// For simplicity and performance, keep the last observed value
	om.metrics[key] = &MetricValue{
		Name:      name,
		Type:      MetricHistogram,
		Value:     val,
		Labels:    labels,
		Timestamp: time.Now(),
	}
}

// SnapshotMetrics retrieves a read-only list of current metric records
func (om *ObservabilityManager) SnapshotMetrics() []MetricValue {
	om.mu.RLock()
	defer om.mu.RUnlock()

	list := make([]MetricValue, 0, len(om.metrics))
	for _, mv := range om.metrics {
		list = append(list, *mv)
	}
	return list
}

// CheckDependencyHealth aggregates standard check rules for key platform integrations
func (om *ObservabilityManager) CheckDependencyHealth(ctx context.Context, dbCheck, redisCheck, kafkaCheck, bchainCheck func(context.Context) error) (DependencyStatus, map[string]string) {
	details := make(map[string]string)
	degraded := false
	notReady := false

	// 1. PostgreSQL DB Check (Mandatory dependency)
	if dbCheck != nil {
		if err := dbCheck(ctx); err != nil {
			details["database"] = fmt.Sprintf("DOWN: %v", err)
			notReady = true
		} else {
			details["database"] = "UP"
		}
	} else {
		details["database"] = "OPTIONAL_NOT_CONFIGURED"
	}

	// 2. Redis Cache Check (Mandatory dependency)
	if redisCheck != nil {
		if err := redisCheck(ctx); err != nil {
			details["redis"] = fmt.Sprintf("DOWN: %v", err)
			notReady = true
		} else {
			details["redis"] = "UP"
		}
	} else {
		details["redis"] = "OPTIONAL_NOT_CONFIGURED"
	}

	// 3. Kafka Queue Check (Optional dependency, degrades system but doesn't halt completely)
	if kafkaCheck != nil {
		if err := kafkaCheck(ctx); err != nil {
			details["kafka"] = fmt.Sprintf("DOWN: %v", err)
			degraded = true
		} else {
			details["kafka"] = "UP"
		}
	} else {
		details["kafka"] = "OPTIONAL_NOT_CONFIGURED"
	}

	// 4. Blockchain Conn Check (Optional dependency, degrades system but doesn't halt completely)
	if bchainCheck != nil {
		if err := bchainCheck(ctx); err != nil {
			details["blockchain"] = fmt.Sprintf("DOWN: %v", err)
			degraded = true
		} else {
			details["blockchain"] = "UP"
		}
	} else {
		details["blockchain"] = "OPTIONAL_NOT_CONFIGURED"
	}

	status := StatusLive
	if notReady {
		status = StatusNotReady
	} else if degraded {
		status = StatusDegraded
	} else {
		status = StatusReady
	}

	return status, details
}

// CollectSystemResources gathers system level CPU/Memory telemetry metrics
func (om *ObservabilityManager) CollectSystemResources() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"service":            om.serviceName,
		"uptime_seconds":     time.Since(om.startedAt).Seconds(),
		"memory_alloc_bytes": m.Alloc,
		"num_goroutines":     runtime.NumGoroutine(),
		"num_cpu":            runtime.NumCPU(),
	}
}

// TraceSpan defines a lightweight transaction execution trace span
type TraceSpan struct {
	TraceID   string    `json:"trace_id"`
	SpanID    string    `json:"span_id"`
	ParentID  string    `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	StartTime time.Time `json:"start_time"`
	Duration  string    `json:"duration,omitempty"`
}

// ErrorType categories internal classifications
type ErrorType string

const (
	ErrorValidation     ErrorType = "VALIDATION"
	ErrorAuthentication ErrorType = "AUTHENTICATION"
	ErrorAuthorization  ErrorType = "AUTHORIZATION"
	ErrorRateLimiting   ErrorType = "RATE_LIMITING"
	ErrorDatabase       ErrorType = "DATABASE"
	ErrorKafka          ErrorType = "KAFKA"
	ErrorRedis          ErrorType = "REDIS"
	ErrorBlockchain     ErrorType = "BLOCKCHAIN"
	ErrorCompliance     ErrorType = "COMPLIANCE"
	ErrorRisk           ErrorType = "RISK"
	ErrorMatching       ErrorType = "MATCHING"
	ErrorSettlement     ErrorType = "SETTLEMENT"
	ErrorInternal       ErrorType = "INTERNAL"
)

// CategorizedError encapsulates standard classified platform error objects
type CategorizedError struct {
	Type    ErrorType `json:"type"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Err     error     `json:"-"`
}

func (ce *CategorizedError) Error() string {
	if ce.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", ce.Type, ce.Message, ce.Err)
	}
	return fmt.Sprintf("[%s] %s", ce.Type, ce.Message)
}

// NewCategorizedError instantiates standard classified system errors
func NewCategorizedError(t ErrorType, code string, msg string, err error) *CategorizedError {
	return &CategorizedError{
		Type:    t,
		Code:    code,
		Message: msg,
		Err:     err,
	}
}
