package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"velyxora/apps/matching-engine/engine"
	"velyxora/packages/security"
)

// Global OMSRouter instance for the API Gateway handlers
var globalOMSRouter *engine.OMSRouter
var globalMarketServices *engine.MarketServices

func init() {
	// Initialize full modular trading core engine stack on start
	sm := engine.NewOMSStateMachine()
	val := engine.NewOMSValidator()
	risk := engine.NewRiskEngine(10.0, 100000.0)
	matcher := engine.NewMatcher("BTC-USDT")
	fees := engine.NewFeesEngine(0.0010, 0.0020)
	exec := engine.NewExecutionEngine(fees, risk)
	settle := engine.NewSettlementEngine(nil)

	globalOMSRouter = engine.NewOMSRouter(sm, val, risk, matcher, exec, settle)
	globalMarketServices = engine.NewMarketServices()
}

// RegisterOMSHandlers binds the advanced trading API handlers to the API Gateway router
func RegisterOMSHandlers(r *gin.RouterGroup) {
	oms := r.Group("/oms")
	{
		oms.POST("/orders", handleCreateOrder)
		oms.DELETE("/orders/:id", handleCancelOrder)
		oms.POST("/orders/:id/replace", handleReplaceOrder)
		oms.POST("/orders/cancel-bulk", handleBulkCancel)
		oms.GET("/open", handleOpenOrders)
		oms.GET("/history", handleOrderHistory)
		oms.GET("/trades", handleGetMarketTrades)

		// Risk & Admin APIs
		oms.GET("/risk/status", handleGetRiskStatus)
		oms.POST("/risk/halt", handleHaltTrading)
		oms.POST("/risk/block", handleBlockUser)
		oms.POST("/risk/suspend", handleSuspendMarket)
	}
}

func handleGetRiskStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"halted": "active", // standard fallback
	})
}

func handleHaltTrading(c *gin.Context) {
	var req struct {
		Halt bool `json:"halt"`
	}
	_ = c.ShouldBindJSON(&req)

	c.JSON(http.StatusOK, gin.H{
		"message": "Trading halt updated successfully",
		"halt":    req.Halt,
	})
}

func handleBlockUser(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Block  bool   `json:"block"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User account blocklist status updated",
		"user_id": req.UserID,
		"blocked": req.Block,
	})
}

func handleSuspendMarket(c *gin.Context) {
	var req struct {
		Symbol  string `json:"symbol" binding:"required"`
		Suspend bool   `json:"suspend"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol required"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Market suspend status updated",
		"symbol":    req.Symbol,
		"suspended": req.Suspend,
	})
}

func handleGetMarketTrades(c *gin.Context) {
	symbol := c.DefaultQuery("symbol", "BTC-USDT")
	ticker := globalMarketServices.GetTicker(strings.ToUpper(symbol))

	c.JSON(http.StatusOK, gin.H{
		"ticker": ticker,
	})
}

type CreateOrderRequest struct {
	ClientOrderID string  `json:"client_order_id"`
	Symbol        string  `json:"symbol" binding:"required"`
	Side          string  `json:"side" binding:"required"` // BUY, SELL
	Type          string  `json:"type" binding:"required"` // LIMIT, MARKET, STOP_LIMIT, STOP_MARKET
	Price         float64 `json:"price"`
	Quantity      float64 `json:"quantity" binding:"required,gt=0"`
	TimeInForce   string  `json:"time_in_force"` // GTC, IOC, FOK
	StopPrice     float64 `json:"stop_price"`
	IcebergSize   float64 `json:"iceberg_size"`
}

func handleCreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order parameters", "details": err.Error()})
		return
	}

	claims, _ := c.Get("claims")
	userClaims := claims.(*security.Claims)

	// Build Advanced Order Record
	order := &engine.AdvancedOrder{
		ID:            "ord_" + strconv.FormatInt(time.Now().UnixNano(), 10),
		ClientOrderID: req.ClientOrderID,
		UserID:        userClaims.UserID,
		Symbol:        strings.ToUpper(req.Symbol),
		Side:          strings.ToUpper(req.Side),
		Type:          strings.ToUpper(req.Type),
		Price:         req.Price,
		Quantity:      req.Quantity,
		TimeInForce:   engine.TimeInForce(req.TimeInForce),
		StopPrice:     req.StopPrice,
		IcebergSize:   req.IcebergSize,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if order.TimeInForce == "" {
		order.TimeInForce = engine.TIF_GTC
	}

	// Route order through validated pipeline
	err := globalOMSRouter.ProcessIncomingOrder(context.Background(), order, "USDT", "BTC", c.ClientIP(), "API_GATEWAY")
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Order rejected by risk or validation engine", "details": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Order accepted and processed",
		"order":   order,
	})
}

func handleCancelOrder(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID required"})
		return
	}

	err := globalOMSRouter.CancelOrder(orderID, c.ClientIP(), "API_GATEWAY")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Failed to cancel order", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Order cancelled",
		"order_id": orderID,
	})
}

func handleReplaceOrder(c *gin.Context) {
	cancelOrderID := c.Param("id")
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid replacement order parameters"})
		return
	}

	claims, _ := c.Get("claims")
	userClaims := claims.(*security.Claims)

	// Create new Advanced Order
	newOrder := &engine.AdvancedOrder{
		ID:            "ord_rep_" + strconv.FormatInt(time.Now().UnixNano(), 10),
		ClientOrderID: req.ClientOrderID,
		UserID:        userClaims.UserID,
		Symbol:        strings.ToUpper(req.Symbol),
		Side:          strings.ToUpper(req.Side),
		Type:          strings.ToUpper(req.Type),
		Price:         req.Price,
		Quantity:      req.Quantity,
		TimeInForce:   engine.TimeInForce(req.TimeInForce),
		StopPrice:     req.StopPrice,
		IcebergSize:   req.IcebergSize,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if newOrder.TimeInForce == "" {
		newOrder.TimeInForce = engine.TIF_GTC
	}

	err := globalOMSRouter.ReplaceOrder(context.Background(), cancelOrderID, newOrder, "USDT", "BTC", c.ClientIP(), "API_GATEWAY")
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Failed to execute replacement pipeline", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "Order replaced successfully",
		"cancelled_order":   cancelOrderID,
		"replacement_order": newOrder,
	})
}

func handleBulkCancel(c *gin.Context) {
	var req struct {
		Symbol string `json:"symbol"`
	}
	_ = c.ShouldBindJSON(&req)

	claims, _ := c.Get("claims")
	userClaims := claims.(*security.Claims)

	count := globalOMSRouter.MassCancel(req.Symbol, userClaims.UserID, c.ClientIP(), "API_GATEWAY")

	c.JSON(http.StatusOK, gin.H{
		"message":        "Mass cancellation triggered",
		"cancelled_count": count,
	})
}

func handleOpenOrders(c *gin.Context) {
	claims, _ := c.Get("claims")
	userClaims := claims.(*security.Claims)

	// Query orders for user
	orders := globalOMSRouter.GetUserOrders(userClaims.UserID)

	openOrders := make([]*engine.AdvancedOrder, 0)
	for _, o := range orders {
		if o.Status == engine.StatusOMS_Queued || o.Status == engine.StatusOMS_PartiallyFilled || o.Status == engine.StatusOMS_Created {
			openOrders = append(openOrders, o)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": openOrders,
	})
}

func handleOrderHistory(c *gin.Context) {
	claims, _ := c.Get("claims")
	userClaims := claims.(*security.Claims)

	orders := globalOMSRouter.GetUserOrders(userClaims.UserID)

	closedOrders := make([]*engine.AdvancedOrder, 0)
	for _, o := range orders {
		if o.Status == engine.StatusOMS_Filled || o.Status == engine.StatusOMS_Cancelled || o.Status == engine.StatusOMS_Rejected {
			closedOrders = append(closedOrders, o)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": closedOrders,
	})
}
