package types

import (
	"time"
)

// User represents a user account on Velyxora
type User struct {
	ID           string    `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	IsMFAEnabled bool      `json:"is_mfa_enabled" db:"is_mfa_enabled"`
	MFASecret    string    `json:"-" db:"mfa_secret"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// UserAPIKey represents API key credentials for programmatic trading access
type UserAPIKey struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Label       string    `json:"label" db:"label"`
	ApiKey      string    `json:"api_key" db:"api_key"`
	ApiSecret   string    `json:"-" db:"api_secret"`
	Permissions string    `json:"permissions" db:"permissions"` // e.g. "read", "trade", "withdraw"
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	ExpiresAt   time.Time `json:"expires_at" db:"expires_at"`
}

// OrderSide represents whether an order is a buy or sell order
type OrderSide string

const (
	SideBuy  OrderSide = "BUY"
	SideSell OrderSide = "SELL"
)

// OrderType represents whether an order is a limit, market, etc.
type OrderType string

const (
	TypeLimit  OrderType = "LIMIT"
	TypeMarket OrderType = "MARKET"
)

// OrderStatus represents the current state of an order
type OrderStatus string

const (
	StatusNew             OrderStatus = "NEW"
	StatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	StatusFilled          OrderStatus = "FILLED"
	StatusCancelled       OrderStatus = "CANCELLED"
	StatusRejected        OrderStatus = "REJECTED"
)

// Order represents an order placed by a user
type Order struct {
	ID        string      `json:"id" db:"id"`
	UserID    string      `json:"user_id" db:"user_id"`
	Symbol    string      `json:"symbol" db:"symbol"` // e.g. "BTC-USDT"
	Side      OrderSide   `json:"side" db:"side"`
	Type      OrderType   `json:"type" db:"type"`
	Price     float64     `json:"price" db:"price"`
	Quantity  float64     `json:"quantity" db:"quantity"`
	FilledQty float64     `json:"filled_quantity" db:"filled_quantity"`
	Status    OrderStatus `json:"status" db:"status"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt time.Time   `json:"updated_at" db:"updated_at"`
}

// Trade represents a execution match between a buyer and seller
type Trade struct {
	ID          string    `json:"id" db:"id"`
	Symbol      string    `json:"symbol" db:"symbol"`
	BuyerID     string    `json:"buyer_id" db:"buyer_id"`
	SellerID    string    `json:"seller_id" db:"seller_id"`
	BuyOrderID  string    `json:"buy_order_id" db:"buy_order_id"`
	SellOrderID string    `json:"sell_order_id" db:"sell_order_id"`
	Price       float64   `json:"price" db:"price"`
	Quantity    float64   `json:"quantity" db:"quantity"`
	Timestamp   time.Time `json:"timestamp" db:"timestamp"`
}

// Balance represents a user's wallet balance for a specific asset
type Balance struct {
	UserID    string    `json:"user_id" db:"user_id"`
	Asset     string    `json:"asset" db:"asset"` // e.g. "BTC", "USDT"
	Free      float64   `json:"free" db:"free"`
	Locked    float64   `json:"locked" db:"locked"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// EventType represents the category of the message sent on Kafka
type EventType string

const (
	EventOrderCreated  EventType = "ORDER_CREATED"
	EventOrderMatch    EventType = "ORDER_MATCH"
	EventOrderCancel   EventType = "ORDER_CANCEL"
	EventOrderRejected EventType = "ORDER_REJECTED"
	EventBalanceUpdate EventType = "BALANCE_UPDATE"
)

// KafkaEvent is the standard payload wrapper for all Kafka messages
type KafkaEvent struct {
	Type      EventType   `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}
