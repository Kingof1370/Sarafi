package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"velyxora/packages/security"
)

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	secret := "testsecret12345678"
	r.Use(authMiddleware(secret))
	r.GET("/protected", func(c *gin.Context) {
		claims, _ := c.Get("claims")
		cl := claims.(*security.Claims)
		c.JSON(http.StatusOK, gin.H{"user_id": cl.UserID})
	})

	// Test missing header
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}

	// Test invalid token
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/protected", nil)
	req2.Header.Set("Authorization", "Bearer invalidtoken")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w2.Code)
	}

	// Test valid token
	access, _, _ := security.GenerateJWT("usr_123", "test@test.com", "sess_123", secret, 5*time.Minute, 1*time.Hour)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/protected", nil)
	req3.Header.Set("Authorization", "Bearer "+access)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w3.Code)
	}

	var resp map[string]string
	_ = json.Unmarshal(w3.Body.Bytes(), &resp)
	if resp["user_id"] != "usr_123" {
		t.Errorf("Expected user_id 'usr_123', got %s", resp["user_id"])
	}
}

func TestWalletWithdrawalValidationAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	secret := "testsecret12345678"

	v1 := r.Group("/api/v1")
	wallet := v1.Group("/wallet")
	wallet.Use(authMiddleware(secret))
	{
		wallet.POST("/withdraw", func(c *gin.Context) {
			var req struct {
				Asset   string  `json:"asset" binding:"required"`
				Amount  float64 `json:"amount" binding:"required,gt=0"`
				Address string  `json:"address" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// Validate ETH Address
			if req.Asset == "ETH" && len(req.Address) != 42 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid destination blockchain address format"})
				return
			}

			c.JSON(http.StatusAccepted, gin.H{"message": "Withdrawal request registered, pending risk audit"})
		})
	}

	// Submit withdrawal request with invalid address format
	access, _, _ := security.GenerateJWT("usr_123", "test@test.com", "sess_123", secret, 5*time.Minute, 1*time.Hour)
	w := httptest.NewRecorder()
	payload := `{"asset":"ETH","amount":1.5,"address":"invalid_addr"}`
	req, _ := http.NewRequest("POST", "/api/v1/wallet/withdraw", bytes.NewReader([]byte(payload)))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request, got %d", w.Code)
	}

	// Submit withdrawal request with valid ETH mock format
	w2 := httptest.NewRecorder()
	payload2 := `{"asset":"ETH","amount":1.5,"address":"0x71C7656EC7ab88b098defB751B7401B5f6d1476B"}`
	req2, _ := http.NewRequest("POST", "/api/v1/wallet/withdraw", bytes.NewReader([]byte(payload2)))
	req2.Header.Set("Authorization", "Bearer "+access)
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusAccepted {
		t.Errorf("Expected status 202 Accepted, got %d", w2.Code)
	}
}

func TestGetRequestWeight(t *testing.T) {
	// 1. Test GET /api/v1/market/depth with no context or no query
	w1 := GetRequestWeight("GET", "/api/v1/market/depth", nil)
	if w1 != 50 {
		t.Errorf("Expected weight 50, got %d", w1)
	}

	// 2. Test GET /api/v1/market/depth with limit query <= 20
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request, _ = http.NewRequest("GET", "/api/v1/market/depth?limit=10", nil)
	w2 := GetRequestWeight("GET", "/api/v1/market/depth", c2)
	if w2 != 20 {
		t.Errorf("Expected weight 20, got %d", w2)
	}

	// 3. Test GET /api/v1/market/depth with limit query > 20
	c3, _ := gin.CreateTestContext(httptest.NewRecorder())
	c3.Request, _ = http.NewRequest("GET", "/api/v1/market/depth?limit=50", nil)
	w3 := GetRequestWeight("GET", "/api/v1/market/depth", c3)
	if w3 != 50 {
		t.Errorf("Expected weight 50, got %d", w3)
	}

	// 4. Test GET /api/v1/market/trades
	w4 := GetRequestWeight("GET", "/api/v1/market/trades", nil)
	if w4 != 5 {
		t.Errorf("Expected weight 5, got %d", w4)
	}

	// 5. Test POST /api/v1/oms/orders
	w5 := GetRequestWeight("POST", "/api/v1/oms/orders", nil)
	if w5 != 2 {
		t.Errorf("Expected weight 2, got %d", w5)
	}

	// 6. Test GET /api/v1/system/health
	w6 := GetRequestWeight("GET", "/api/v1/system/health", nil)
	if w6 != 1 {
		t.Errorf("Expected weight 1, got %d", w6)
	}

	// 7. Test default weight
	w7 := GetRequestWeight("GET", "/api/v1/some/random/endpoint", nil)
	if w7 != 1 {
		t.Errorf("Expected weight 1, got %d", w7)
	}
}

func TestRequestWeightLimiter_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Override hook to simulate an allowed request with cumulative weight 100
	redisEvalHook = func(ctx context.Context, key string, now, window, limit, weight int64, member string) ([]interface{}, error) {
		return []interface{}{int64(1), int64(100), int64(0)}, nil
	}
	defer func() { redisEvalHook = nil }()

	r.Use(RequestWeightLimiter())
	r.GET("/api/v1/system/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/system/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}

	usedWeight := w.Header().Get("X-MBX-USED-WEIGHT-(1m)")
	if usedWeight != "100" {
		t.Errorf("Expected X-MBX-USED-WEIGHT-(1m) header to be '100', got '%s'", usedWeight)
	}
}

func TestRequestWeightLimiter_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Override hook to simulate a blocked request with cumulative weight 1205 and retry after 45s
	redisEvalHook = func(ctx context.Context, key string, now, window, limit, weight int64, member string) ([]interface{}, error) {
		return []interface{}{int64(0), int64(1205), int64(45)}, nil
	}
	defer func() { redisEvalHook = nil }()

	r.Use(RequestWeightLimiter())
	r.GET("/api/v1/market/depth", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/market/depth", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}

	usedWeight := w.Header().Get("X-MBX-USED-WEIGHT-(1m)")
	if usedWeight != "1205" {
		t.Errorf("Expected X-MBX-USED-WEIGHT-(1m) header to be '1205', got '%s'", usedWeight)
	}

	retryAfter := w.Header().Get("Retry-After")
	if retryAfter != "45" {
		t.Errorf("Expected Retry-After header to be '45', got '%s'", retryAfter)
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "Too many requests. API weight limit exceeded." {
		t.Errorf("Expected error message 'Too many requests. API weight limit exceeded.', got '%v'", resp["error"])
	}
}
