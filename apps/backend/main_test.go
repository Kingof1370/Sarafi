package main

import (
	"bytes"
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
	access, _, _ := security.GenerateJWT("usr_123", "test@test.com", secret, 5*time.Minute, 1*time.Hour)
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
	access, _, _ := security.GenerateJWT("usr_123", "test@test.com", secret, 5*time.Minute, 1*time.Hour)
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
