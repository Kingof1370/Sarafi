// Package tradingcore defines the single client contract every service uses to
// talk to the authoritative trading core (the matching-engine service).
//
// No service other than the matching-engine may instantiate matching, risk,
// execution or settlement engines. Everything else is a client of this package.
package tradingcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"velyxora/packages/types"
)

// Sentinel errors returned by the client so callers can map them to transport
// status codes without string matching.
var (
	// ErrNotFound is returned when the trading core has no such order.
	ErrNotFound = errors.New("tradingcore: resource not found")
	// ErrUnavailable is returned when the trading core cannot be reached.
	ErrUnavailable = errors.New("tradingcore: trading core unavailable")
	// ErrUnauthorized is returned when the internal service token is rejected.
	ErrUnauthorized = errors.New("tradingcore: internal service token rejected")
	// ErrNotConfigured is returned when no trading core endpoint is configured.
	ErrNotConfigured = errors.New("tradingcore: MATCHING_ENGINE_URL is not configured")
)

// RejectedError is returned when the trading core deliberately rejected the
// operation (validation, risk, compliance or state machine rejection).
type RejectedError struct {
	Reason string
}

func (e *RejectedError) Error() string { return "tradingcore: rejected: " + e.Reason }

// MassCancelResult reports how many resting orders were cancelled.
type MassCancelResult struct {
	Cancelled int `json:"cancelled"`
}

// Client is the complete contract of the authoritative trading core.
type Client interface {
	SubmitOrder(ctx context.Context, order *types.Order) (*types.Order, error)
	CancelOrder(ctx context.Context, orderID string) error
	ReplaceOrder(ctx context.Context, cancelOrderID string, newOrder *types.Order) (*types.Order, error)
	MassCancel(ctx context.Context, symbol, userID string) (int, error)
	GetUserOrders(ctx context.Context, userID string) ([]types.Order, error)
	GetOrder(ctx context.Context, orderID string) (*types.Order, error)
	GetDepth(ctx context.Context, symbol string, limit int) (*types.OrderBookL2, error)
	SetHalt(ctx context.Context, halted bool) error
	BlockAccount(ctx context.Context, userID string, blocked bool) error
	SuspendMarket(ctx context.Context, symbol string, suspended bool) error
	// Passthrough performs a raw read against the trading core for payloads the
	// gateway only forwards (ticker, candles, stats, recent trades).
	Passthrough(ctx context.Context, path string, query url.Values) ([]byte, error)
	Health(ctx context.Context) error
}

// Config configures the HTTP client for the trading core.
type Config struct {
	BaseURL string
	Token   string
	Timeout time.Duration
}

// HTTPClient is the production implementation of Client, talking to the
// matching-engine internal API over HTTP.
type HTTPClient struct {
	baseURL string
	token   string
	http    *http.Client
}

var _ Client = (*HTTPClient)(nil)

// NewHTTPClient builds a trading core client. It fails closed when no endpoint
// or no internal service token is configured.
func NewHTTPClient(cfg Config) (*HTTPClient, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return nil, ErrNotConfigured
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("tradingcore: INTERNAL_SERVICE_TOKEN is not configured")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPClient{
		baseURL: base,
		token:   cfg.Token,
		http:    &http.Client{Timeout: timeout},
	}, nil
}

type errorBody struct {
	Error string `json:"error"`
}

func (c *HTTPClient) do(ctx context.Context, method, path string, query url.Values, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("tradingcore: encode request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	full := c.baseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, full, reader)
	if err != nil {
		return fmt.Errorf("tradingcore: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return ErrUnauthorized
	case resp.StatusCode == http.StatusUnprocessableEntity || resp.StatusCode == http.StatusBadRequest:
		var eb errorBody
		_ = json.Unmarshal(payload, &eb)
		if eb.Error == "" {
			eb.Error = strings.TrimSpace(string(payload))
		}
		return &RejectedError{Reason: eb.Error}
	case resp.StatusCode >= 500:
		var eb errorBody
		_ = json.Unmarshal(payload, &eb)
		return fmt.Errorf("%w: status %d: %s", ErrUnavailable, resp.StatusCode, eb.Error)
	}

	if out != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, out); err != nil {
			return fmt.Errorf("tradingcore: decode response: %w", err)
		}
	}
	return nil
}

// SubmitOrder submits a canonical order to the authoritative trading core.
func (c *HTTPClient) SubmitOrder(ctx context.Context, order *types.Order) (*types.Order, error) {
	var out types.Order
	if err := c.do(ctx, http.MethodPost, "/internal/v1/orders", nil, order, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelOrder cancels a resting order by ID.
func (c *HTTPClient) CancelOrder(ctx context.Context, orderID string) error {
	return c.do(ctx, http.MethodDelete, "/internal/v1/orders/"+url.PathEscape(orderID), nil, nil, nil)
}

// ReplaceOrder atomically cancels an order and submits its replacement.
func (c *HTTPClient) ReplaceOrder(ctx context.Context, cancelOrderID string, newOrder *types.Order) (*types.Order, error) {
	var out types.Order
	path := "/internal/v1/orders/" + url.PathEscape(cancelOrderID) + "/replace"
	if err := c.do(ctx, http.MethodPost, path, nil, newOrder, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MassCancel cancels every resting order of a user, optionally per symbol.
func (c *HTTPClient) MassCancel(ctx context.Context, symbol, userID string) (int, error) {
	var out MassCancelResult
	body := map[string]string{"symbol": symbol, "user_id": userID}
	if err := c.do(ctx, http.MethodPost, "/internal/v1/orders/mass-cancel", nil, body, &out); err != nil {
		return 0, err
	}
	return out.Cancelled, nil
}

// GetUserOrders returns every order tracked for a user.
func (c *HTTPClient) GetUserOrders(ctx context.Context, userID string) ([]types.Order, error) {
	var out []types.Order
	q := url.Values{"user_id": []string{userID}}
	if err := c.do(ctx, http.MethodGet, "/internal/v1/orders", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetOrder returns a single order by ID.
func (c *HTTPClient) GetOrder(ctx context.Context, orderID string) (*types.Order, error) {
	var out types.Order
	if err := c.do(ctx, http.MethodGet, "/internal/v1/orders/"+url.PathEscape(orderID), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDepth returns the aggregated L2 book of a market.
func (c *HTTPClient) GetDepth(ctx context.Context, symbol string, limit int) (*types.OrderBookL2, error) {
	var out types.OrderBookL2
	q := url.Values{"symbol": []string{symbol}, "limit": []string{fmt.Sprintf("%d", limit)}}
	if err := c.do(ctx, http.MethodGet, "/internal/v1/depth", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetHalt toggles the global trading halt in the risk engine.
func (c *HTTPClient) SetHalt(ctx context.Context, halted bool) error {
	return c.do(ctx, http.MethodPost, "/internal/v1/admin/halt", nil, map[string]bool{"halt": halted}, nil)
}

// BlockAccount blocks or unblocks a trading account.
func (c *HTTPClient) BlockAccount(ctx context.Context, userID string, blocked bool) error {
	body := map[string]interface{}{"user_id": userID, "blocked": blocked}
	return c.do(ctx, http.MethodPost, "/internal/v1/admin/block-account", nil, body, nil)
}

// SuspendMarket suspends or resumes a market.
func (c *HTTPClient) SuspendMarket(ctx context.Context, symbol string, suspended bool) error {
	body := map[string]interface{}{"symbol": symbol, "suspended": suspended}
	return c.do(ctx, http.MethodPost, "/internal/v1/admin/suspend-market", nil, body, nil)
}

// Passthrough returns a raw JSON document from the trading core.
func (c *HTTPClient) Passthrough(ctx context.Context, path string, query url.Values) ([]byte, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, path, query, nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// Health probes trading core reachability.
func (c *HTTPClient) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/internal/v1/health", nil, nil, nil)
}
