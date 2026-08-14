package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"velyxora/apps/matching-engine/engine"
	"velyxora/packages/logger"
	"velyxora/packages/tradingcore"
	"velyxora/packages/types"
)

// internalAPI exposes the authoritative trading core over HTTP so that other
// services (API gateway, wallet service, operations tooling) can drive it
// without ever instantiating an engine of their own.
type internalAPI struct {
	router    *engine.OMSRouter
	markets   *engine.MarketServices
	candles   *engine.CandleEngine
	liquidity *engine.LiquidityEngine
	token     string
	log       *logger.Logger
}

func newInternalAPI(router *engine.OMSRouter, markets *engine.MarketServices, candles *engine.CandleEngine, token string, log *logger.Logger) *internalAPI {
	return &internalAPI{
		router:    router,
		markets:   markets,
		candles:   candles,
		liquidity: engine.NewLiquidityEngine(),
		token:     token,
		log:       log,
	}
}

func (a *internalAPI) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/v1/health", a.authed(a.handleHealth))
	mux.HandleFunc("/internal/v1/orders", a.authed(a.handleOrdersCollection))
	mux.HandleFunc("/internal/v1/orders/", a.authed(a.handleOrderItem))
	mux.HandleFunc("/internal/v1/depth", a.authed(a.handleDepth))
	mux.HandleFunc("/internal/v1/ticker", a.authed(a.handleTicker))
	mux.HandleFunc("/internal/v1/trades", a.authed(a.handleTrades))
	mux.HandleFunc("/internal/v1/stats", a.authed(a.handleStats))
	mux.HandleFunc("/internal/v1/candles", a.authed(a.handleCandles))
	mux.HandleFunc("/internal/v1/admin/halt", a.authed(a.handleHalt))
	mux.HandleFunc("/internal/v1/admin/block-account", a.authed(a.handleBlockAccount))
	mux.HandleFunc("/internal/v1/admin/suspend-market", a.authed(a.handleSuspendMarket))
	// Unauthenticated liveness probe for orchestrators only; exposes no data.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

// authed enforces the shared internal service token on every data path.
// It fails closed: a missing or mismatching token is rejected.
func (a *internalAPI) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Internal-Token")
		if a.token == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(a.token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid internal service token"})
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func (a *internalAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"markets": a.router.GetMarketRegistry().GetRegisteredSymbols(),
		"time":    time.Now().UTC(),
	})
}

func (a *internalAPI) handleOrdersCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		a.submitOrder(w, r)
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
			return
		}
		writeJSON(w, http.StatusOK, engine.OrdersToCanonical(a.router.GetUserOrders(userID)))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (a *internalAPI) submitOrder(w http.ResponseWriter, r *http.Request) {
	var dto types.Order
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order payload"})
		return
	}
	order := engine.OrderFromCanonical(&dto)
	if order == nil || order.ID == "" || order.Symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order id and symbol are required"})
		return
	}

	quote, base := engine.SplitSymbol(order.Symbol)
	if err := a.router.ProcessIncomingOrder(r.Context(), order, quote, base, clientIP(r), "INTERNAL_API"); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, engine.OrderToCanonical(order))
}

func (a *internalAPI) handleOrderItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/internal/v1/orders/")

	if rest == "mass-cancel" {
		a.handleMassCancel(w, r)
		return
	}

	if strings.HasSuffix(rest, "/replace") {
		a.handleReplace(w, r, strings.TrimSuffix(rest, "/replace"))
		return
	}

	orderID := rest
	if orderID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order id is required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		order, err := a.router.GetOrder(orderID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, engine.OrderToCanonical(order))
	case http.MethodDelete:
		if err := a.router.CancelOrder(orderID, clientIP(r), "INTERNAL_API"); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"order_id": orderID, "status": "CANCELLED"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (a *internalAPI) handleReplace(w http.ResponseWriter, r *http.Request, cancelOrderID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var dto types.Order
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order payload"})
		return
	}
	newOrder := engine.OrderFromCanonical(&dto)
	if newOrder == nil || newOrder.ID == "" || newOrder.Symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order id and symbol are required"})
		return
	}

	quote, base := engine.SplitSymbol(newOrder.Symbol)
	if err := a.router.ReplaceOrder(r.Context(), cancelOrderID, newOrder, quote, base, clientIP(r), "INTERNAL_API"); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, engine.OrderToCanonical(newOrder))
}

func (a *internalAPI) handleMassCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Symbol string `json:"symbol"`
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}
	count := a.router.MassCancel(strings.ToUpper(req.Symbol), req.UserID, clientIP(r), "INTERNAL_API")
	writeJSON(w, http.StatusOK, tradingcore.MassCancelResult{Cancelled: count})
}

func (a *internalAPI) handleDepth(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(r.URL.Query().Get("symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol is required"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	writeJSON(w, http.StatusOK, a.router.GetAggregatedL2DepthForSymbol(symbol, limit))
}

func (a *internalAPI) handleTicker(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(r.URL.Query().Get("symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol is required"})
		return
	}
	writeJSON(w, http.StatusOK, a.markets.GetTicker(symbol))
}

func (a *internalAPI) handleTrades(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(r.URL.Query().Get("symbol"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	executions := a.router.GetRecentTrades()
	filtered := make([]*engine.Execution, 0, len(executions))
	for _, ex := range executions {
		if symbol != "" && strings.ToUpper(ex.Symbol) != symbol {
			continue
		}
		filtered = append(filtered, ex)
	}
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (a *internalAPI) handleStats(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(r.URL.Query().Get("symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol is required"})
		return
	}
	matcher := a.router.GetMatcherForSymbol(symbol)
	if matcher == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown market"})
		return
	}
	writeJSON(w, http.StatusOK, a.liquidity.AnalyzeDepth(matcher))
}

func (a *internalAPI) handleCandles(w http.ResponseWriter, r *http.Request) {
	if a.candles == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "candle storage is not configured"})
		return
	}
	symbol := strings.ToUpper(r.URL.Query().Get("symbol"))
	interval := r.URL.Query().Get("interval")
	if symbol == "" || interval == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol and interval are required"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	candles, err := a.candles.GetCandles(r.Context(), symbol, interval, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if candles == nil {
		candles = []engine.Candle{}
	}
	writeJSON(w, http.StatusOK, candles)
}

func (a *internalAPI) handleHalt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Halt bool `json:"halt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	a.router.SetHaltStatus(req.Halt)
	writeJSON(w, http.StatusOK, map[string]bool{"halt": req.Halt})
}

func (a *internalAPI) handleBlockAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID  string `json:"user_id"`
		Blocked bool   `json:"blocked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}
	a.router.BlockAccount(req.UserID, req.Blocked)
	writeJSON(w, http.StatusOK, map[string]interface{}{"user_id": req.UserID, "blocked": req.Blocked})
}

func (a *internalAPI) handleSuspendMarket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Symbol    string `json:"symbol"`
		Suspended bool   `json:"suspended"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol is required"})
		return
	}
	a.router.SuspendMarket(strings.ToUpper(req.Symbol), req.Suspended)
	writeJSON(w, http.StatusOK, map[string]interface{}{"symbol": strings.ToUpper(req.Symbol), "suspended": req.Suspended})
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}
	return host
}

// serve starts the internal API server and stops it when ctx is cancelled.
func (a *internalAPI) serve(ctx context.Context, addr string) *http.Server {
	srv := &http.Server{
		Addr:              addr,
		Handler:           a.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.log.Error("internal trading core API stopped: " + err.Error())
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	return srv
}
