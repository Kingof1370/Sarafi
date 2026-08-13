package types

import (
	"time"
)

// UserStatus represents user account verification and compliance lifecycle states
type UserStatus string

const (
	StatusPendingVerification UserStatus = "PENDING_VERIFICATION"
	StatusActive              UserStatus = "ACTIVE"
	StatusRestricted          UserStatus = "RESTRICTED"
	StatusSuspended           UserStatus = "SUSPENDED"
	StatusLocked              UserStatus = "LOCKED"
	StatusDeleted             UserStatus = "DELETED"
)

// UserRole represents permissions levels assigned to identities
type UserRole string

const (
	RoleUser               UserRole = "USER"
	RoleVerifiedUser       UserRole = "VERIFIED_USER"
	RoleVIPUser            UserRole = "VIP_USER"
	RoleSupportAgent       UserRole = "SUPPORT_AGENT"
	RoleComplianceOfficer  UserRole = "COMPLIANCE_OFFICER"
	RoleSecurityOfficer    UserRole = "SECURITY_OFFICER"
	RoleAdmin              UserRole = "ADMIN"
	RoleSuperAdmin         UserRole = "SUPER_ADMIN"
)

// User represents user credential records (from P001/P002) with advanced status/role mappings
type User struct {
	ID           string     `json:"id" db:"id"`
	Email        string     `json:"email" db:"email"`
	PasswordHash string     `json:"-" db:"password_hash"`
	IsMFAEnabled bool       `json:"is_mfa_enabled" db:"is_mfa_enabled"`
	MFASecret    string     `json:"-" db:"mfa_secret"`
	Status       UserStatus `json:"status" db:"status"`
	Role         UserRole   `json:"role" db:"role"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

// UserProfile represents personal compliance/KYC data
type UserProfile struct {
	UserID      string    `json:"user_id" db:"user_id"`
	FirstName   string    `json:"first_name" db:"first_name"`
	LastName    string    `json:"last_name" db:"last_name"`
	Country     string    `json:"country" db:"country"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// UserPreferences stores platform display configurations
type UserPreferences struct {
	UserID         string `json:"user_id" db:"user_id"`
	Theme          string `json:"theme" db:"theme"` // e.g. "dark", "light"
	Notifications  bool   `json:"notifications" db:"notifications"`
	PreferredAsset string `json:"preferred_asset" db:"preferred_asset"` // e.g. "BTC"
	PayFeesInVLX   bool   `json:"pay_fees_in_vlx" db:"pay_fees_in_vlx"`
}

// UserSession tracks authentication sessions, device fingerprints and rotation keys
type UserSession struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	DeviceID     string    `json:"device_id" db:"device_id"`
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	UserAgent    string    `json:"user_agent" db:"user_agent"`
	RefreshToken string    `json:"-" db:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// SecurityEvent records access logs, permission adjustments and password triggers
type SecurityEvent struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	EventType string    `json:"event_type" db:"event_type"` // e.g. "LOGIN_SUCCESS", "MFA_ENABLED"
	IPAddress string    `json:"ip_address" db:"ip_address"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
	Details   string    `json:"details" db:"details"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
}

// UserAPIKey represents API key credentials (from P001/P002)
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

// OrderBookLevel represents a single price level in the L2 stream
type OrderBookLevel struct {
	Price        float64 `json:"price"`
	Quantity     float64 `json:"quantity"`
	IsAggregated bool    `json:"is_aggregated"`
	Source       string  `json:"source"`
}

// OrderBookL2 represents Level 2 market depth snapshots
type OrderBookL2 struct {
	Symbol    string            `json:"symbol"`
	Bids      []OrderBookLevel  `json:"bids"`
	Asks      []OrderBookLevel  `json:"asks"`
	Sequence  int64             `json:"sequence"`
	Timestamp time.Time         `json:"timestamp"`
}

// LedgerEntryType defines debit or credit states
type LedgerEntryType string

const (
	Debit  LedgerEntryType = "DEBIT"
	Credit LedgerEntryType = "CREDIT"
)

// LedgerEntry represents a single transaction entry in the double-entry accounting ledger
type LedgerEntry struct {
	ID          string          `json:"id" db:"id"`
	LedgerTxID  string          `json:"ledger_tx_id" db:"ledger_tx_id"`
	UserID      string          `json:"user_id" db:"user_id"`
	Asset       string          `json:"asset" db:"asset"`
	Type        LedgerEntryType `json:"type" db:"type"`
	Amount      float64         `json:"amount" db:"amount"`
	Description string          `json:"description" db:"description"`
	Timestamp   time.Time       `json:"timestamp" db:"timestamp"`
}

// LedgerTransaction groups balanced debit and credit entries
type LedgerTransaction struct {
	ID        string         `json:"id" db:"id"`
	Entries   []*LedgerEntry `json:"entries"`
	Timestamp time.Time      `json:"timestamp"`
}

// Balance represents a user's wallet balance for a specific asset (Extended for P004 Available, Locked, Pending, Reserved)
type Balance struct {
	UserID    string    `json:"user_id" db:"user_id"`
	Asset     string    `json:"asset" db:"asset"` // e.g. "BTC", "USDT"
	Available float64   `json:"available" db:"available"`
	Locked    float64   `json:"locked" db:"locked"`
	Pending   float64   `json:"pending" db:"pending"`
	Reserved  float64   `json:"reserved" db:"reserved"`
	Total     float64   `json:"total" db:"total"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Asset represents supported currency registry parameters
type Asset struct {
	Symbol      string  `json:"symbol" db:"symbol"` // e.g. "BTC", "USDT"
	Name        string  `json:"name" db:"name"`
	Precision   int     `json:"precision" db:"precision"`
	MinDeposit  float64 `json:"min_deposit" db:"min_deposit"`
	MinWithdraw float64 `json:"min_withdraw" db:"min_withdraw"`
	WithdrawFee float64 `json:"withdraw_fee" db:"withdraw_fee"`
	IsActive    bool    `json:"is_active" db:"is_active"`
}

// DepositStatus tracks deposit lifecycle
type DepositStatus string

const (
	DepositPending   DepositStatus = "PENDING"
	DepositConfirmed DepositStatus = "CONFIRMED"
	DepositCompleted DepositStatus = "COMPLETED"
	DepositRejected  DepositStatus = "REJECTED"
	DepositExpired   DepositStatus = "EXPIRED"
)

// Deposit represents an asset inbound transaction record
type Deposit struct {
	ID            string        `json:"id" db:"id"`
	UserID        string        `json:"user_id" db:"user_id"`
	Asset         string        `json:"asset" db:"asset"`
	Amount        float64       `json:"amount" db:"amount"`
	Fee           float64       `json:"fee" db:"fee"`
	Address       string        `json:"address" db:"address"`
	TxHash        string        `json:"tx_hash" db:"tx_hash"`
	Confirmations int           `json:"confirmations" db:"confirmations"`
	Status        DepositStatus `json:"status" db:"status"`
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at" db:"updated_at"`
}

// WithdrawalStatus tracks outbound asset payments
type WithdrawalStatus string

const (
	WithdrawalPendingApproval WithdrawalStatus = "PENDING_APPROVAL"
	WithdrawalApproved        WithdrawalStatus = "APPROVED"
	WithdrawalBroadcasting    WithdrawalStatus = "BROADCASTING"
	WithdrawalConfirmed       WithdrawalStatus = "CONFIRMED"
	WithdrawalCompleted       WithdrawalStatus = "COMPLETED"
	WithdrawalRejected        WithdrawalStatus = "REJECTED"
	WithdrawalFailed          WithdrawalStatus = "FAILED"
)

// Withdrawal represents an outbound transaction request
type Withdrawal struct {
	ID          string           `json:"id" db:"id"`
	UserID      string           `json:"user_id" db:"user_id"`
	Asset       string           `json:"asset" db:"asset"`
	Amount      float64          `json:"amount" db:"amount"`
	Fee         float64          `json:"fee" db:"fee"`
	Address     string           `json:"address" db:"address"`
	TxHash      string           `json:"tx_hash" db:"tx_hash"`
	Status      WithdrawalStatus `json:"status" db:"status"`
	RiskScore   float64          `json:"risk_score" db:"risk_score"`
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at" db:"updated_at"`
}

// WalletAddress defines target addresses assigned to users
type WalletAddress struct {
	UserID    string    `json:"user_id" db:"user_id"`
	Asset     string    `json:"asset" db:"asset"`
	Address   string    `json:"address" db:"address"`
	Memo      string    `json:"memo" db:"memo"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
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
	StatusPartiallyFilled OrderStatus = "FILPAR"
	StatusFilled          OrderStatus = "FILLED"
	StatusCancelled       OrderStatus = "CANCELLED"
	StatusRejected        OrderStatus = "REJECTED"
)

// Order represents an order placed by a user
type Order struct {
	ID            string      `json:"id" db:"id"`
	UserID        string      `json:"user_id" db:"user_id"`
	Symbol        string      `json:"symbol" db:"symbol"` // e.g. "BTC-USDT"
	Side          OrderSide   `json:"side" db:"side"`
	Type          OrderType   `json:"type" db:"type"`
	Price         float64     `json:"price" db:"price"`
	Quantity      float64     `json:"quantity" db:"quantity"`
	FilledQty     float64     `json:"filled_quantity" db:"filled_quantity"`
	Status        OrderStatus `json:"status" db:"status"`
	TimeInForce   string      `json:"time_in_force" db:"time_in_force"`
	PostOnly      bool        `json:"post_only" db:"post_only"`
	CreatedAt     time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at" db:"updated_at"`

	// Advanced & Algorithmic Fields (Canonical schema mapping alignment)
	ClientOrderID string      `json:"client_order_id" db:"client_order_id"`
	ExternalRefID string      `json:"external_reference_id" db:"external_reference_id"`
	ExecutionID   string      `json:"execution_id" db:"execution_id"`
	CorrelationID string      `json:"correlation_id" db:"correlation_id"`
	StopPrice     float64     `json:"stop_price" db:"stop_price"`
	TrailingDelta float64     `json:"trailing_delta" db:"trailing_delta"`
	IcebergSize   float64     `json:"iceberg_size" db:"iceberg_size"`
	ReduceOnly    bool        `json:"reduce_only" db:"reduce_only"`
}

// Trade represents a execution match between a buyer and seller
type Trade struct {
	ID         string    `json:"id" db:"id"`
	Symbol     string    `json:"symbol" db:"symbol"`
	BuyerID    string    `json:"buyer_id" db:"buyer_id"`
	SellerID   string    `json:"seller_id" db:"seller_id"`
	BuyOrderID string    `json:"buy_order_id" db:"buy_order_id"`
	SellOrderID string   `json:"sell_order_id" db:"sell_order_id"`
	Price      float64   `json:"price" db:"price"`
	Quantity   float64   `json:"quantity" db:"quantity"`
	Timestamp  time.Time `json:"timestamp" db:"timestamp"`
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
