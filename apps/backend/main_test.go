package main

import (
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
