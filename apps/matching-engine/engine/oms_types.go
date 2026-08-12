package engine

import (
	"time"
)

// AdvancedOrderStatus represents the full lifecycle states of an order in OMS
type AdvancedOrderStatus string

const (
	StatusOMS_Created           AdvancedOrderStatus = "CREATED"
	StatusOMS_PendingValidation AdvancedOrderStatus = "PENDING_VALIDATION"
	StatusOMS_Validated         AdvancedOrderStatus = "VALIDATED"
	StatusOMS_Accepted          AdvancedOrderStatus = "ACCEPTED"
	StatusOMS_Queued            AdvancedOrderStatus = "QUEUED"
	StatusOMS_PartiallyFilled   AdvancedOrderStatus = "PARTIALLY_FILLED"
	StatusOMS_Filled            AdvancedOrderStatus = "FILLED"
	StatusOMS_Cancelled         AdvancedOrderStatus = "CANCELLED"
	StatusOMS_Expired           AdvancedOrderStatus = "EXPIRED"
	StatusOMS_Rejected          AdvancedOrderStatus = "REJECTED"
	StatusOMS_Failed            AdvancedOrderStatus = "FAILED"
	StatusOMS_Archived          AdvancedOrderStatus = "ARCHIVED"
)

// TimeInForce represents the lifespan constraint of an order
type TimeInForce string

const (
	TIF_GTC TimeInForce = "GTC" // Good 'Til Cancelled
	TIF_IOC TimeInForce = "IOC" // Immediate Or Cancel
	TIF_FOK TimeInForce = "FOK" // Fill Or Kill
	TIF_GTD TimeInForce = "GTD" // Good 'Til Date
	TIF_GTT TimeInForce = "GTT" // Good 'Til Time
)

// AdvancedOrder represents a comprehensive enterprise order record
type AdvancedOrder struct {
	ID                  string              `json:"id"`                    // Internal Order ID
	ClientOrderID       string              `json:"client_order_id"`       // Client Order ID
	ExternalRefID       string              `json:"external_reference_id"` // External Reference ID
	ExecutionID         string              `json:"execution_id"`          // Current execution leg ID
	CorrelationID       string              `json:"correlation_id"`        // Trace request ID
	UserID              string              `json:"user_id"`
	Symbol              string              `json:"symbol"`
	Side                string              `json:"side"`       // BUY, SELL
	Type                string              `json:"type"`       // LIMIT, MARKET, STOP_LIMIT, STOP_MARKET, TRAILING_STOP, TWAP, VWAP, ICEBERG
	Price               float64             `json:"price"`
	Quantity            float64             `json:"quantity"`
	FilledQty           float64             `json:"filled_quantity"`
	Status              AdvancedOrderStatus `json:"status"`
	TimeInForce         TimeInForce         `json:"time_in_force"`
	ExpireTime          time.Time           `json:"expire_time"`

	// Advanced Algorithmic Fields
	StopPrice           float64             `json:"stop_price,omitempty"`
	TrailingDelta       float64             `json:"trailing_delta,omitempty"` // for trailing stops (as decimal percentage, e.g. 0.01)
	IcebergSize         float64             `json:"iceberg_size,omitempty"`   // visible peak size for iceberg orders
	PostOnly            bool                `json:"post_only,omitempty"`
	ReduceOnly          bool                `json:"reduce_only,omitempty"`

	// Composite and Bracket Flags
	IsOCO               bool                `json:"is_oco,omitempty"`
	OCOLinkOrderID      string              `json:"oco_link_order_id,omitempty"`
	IsBracket           bool                `json:"is_bracket,omitempty"`
	BracketTakeProfit   float64             `json:"bracket_take_profit,omitempty"`
	BracketStopLoss     float64             `json:"bracket_stop_loss,omitempty"`

	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

// AuditLogEntry is the immutable audit structure for all state modifications
type AuditLogEntry struct {
	ID            string              `json:"id"`
	OrderID       string              `json:"order_id"`
	UserID        string              `json:"user_id"`
	Action        string              `json:"action"` // "CREATE", "VALIDATE", "CANCEL", "REPLACE", "MATCH"
	PreviousState AdvancedOrderStatus `json:"previous_state"`
	NewState      AdvancedOrderStatus `json:"new_state"`
	IPAddress     string              `json:"ip_address"`
	DeviceID      string              `json:"device_id"`
	Result        string              `json:"result"` // "SUCCESS", "REJECTED", "FAILED"
	Timestamp     time.Time           `json:"timestamp"`
}

// TradingPairConfig stores validation limits for specific trading pairs
type TradingPairConfig struct {
	Symbol               string  `json:"symbol"`
	MinOrderSize         float64 `json:"min_order_size"`
	MaxOrderSize         float64 `json:"max_order_size"`
	MinNotional          float64 `json:"min_notional"`
	MaxNotional          float64 `json:"max_notional"`
	TickSize             float64 `json:"tick_size"`         // price step
	StepSize             float64 `json:"step_size"`         // quantity step
	PricePrecision       int     `json:"price_precision"`
	QuantityPrecision    int     `json:"quantity_precision"`
	PriceBandPercentage  float64 `json:"price_band_percentage"`
	PriceBrandPercentage float64 `json:"price_brand_percentage"`
	IsActive             bool    `json:"is_active"`
}
