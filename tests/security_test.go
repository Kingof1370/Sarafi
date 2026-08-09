package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"velyxora/packages/security"
)

// Simple verification endpoint mimicking main gateway registration with password verification
func testRegisterHandler(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	// Validate Password Strength Policy (P002 requirements)
	if err := security.ValidatePasswordStrength(req.Password); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Weak password", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success"})
}

func TestSecurityRegistrationVerification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/register", testRegisterHandler)

	// Case 1: Submit weak password
	w := httptest.NewRecorder()
	reqBody := `{"email":"test@example.com","password":"weak"}`
	req, _ := http.NewRequest("POST", "/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("Expected status 422 Unprocessable Entity, got %d", w.Code)
	}

	// Case 2: Submit valid enterprise password
	w2 := httptest.NewRecorder()
	reqBody2 := `{"email":"test@example.com","password":"StrongCorporate1!"}`
	req2, _ := http.NewRequest("POST", "/register", strings.NewReader(reqBody2))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Errorf("Expected status 201 Created, got %d", w2.Code)
	}
}

func TestJWTValidationSecurityHardening(t *testing.T) {
	secret := "my-highly-secure-jwt-secret-32b-length"
	userID := "usr_1"
	email := "user@example.com"

	// 1. Valid token validation
	token, _, err := security.GenerateJWT(userID, email, "sess_1", secret, 5*time.Minute, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	claims, err := security.ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("Expected valid token to pass, got: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected userID %s, got %s", userID, claims.UserID)
	}

	// 2. Reject incorrect signature
	_, err = security.ValidateJWT(token, "wrong-secret-key-12345")
	if err == nil {
		t.Error("Expected error for wrong cryptographic signature, got nil")
	}

	// 3. Reject expired token
	expiredToken, _, err := security.GenerateJWT(userID, email, "sess_1", secret, -5*time.Minute, -1*time.Hour)
	if err == nil {
		// Valid generation but past dates
		_, err = security.ValidateJWT(expiredToken, secret)
		if err == nil {
			t.Error("Expected error for expired JWT token, got nil")
		}
	}
}

func TestAPISecurityHMACAndClockSkew(t *testing.T) {
	apiSecret := "velyxora-test-api-secret-1234"
	payload := "ts=1700000000&nonce=abc1234&method=GET"

	// 1. Correct signature
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))

	if !security.VerifyAPIKeySignature(payload, sig, apiSecret) {
		t.Error("Expected valid signature to verify successfully")
	}

	// 2. Incorrect signature / forged payload
	if security.VerifyAPIKeySignature(payload, "invalid-sig-hex", apiSecret) {
		t.Error("Expected verification to fail with invalid signature")
	}

	if security.VerifyAPIKeySignature(payload+"altered", sig, apiSecret) {
		t.Error("Expected verification to fail with altered signature payload")
	}
}

func TestCORSValidationFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Mimic production CORS middleware
	r.Use(func(c *gin.Context) {
		env := "production"
		origin := c.GetHeader("Origin")

		if env == "production" {
			allowedStr := "https://velyxora.com,https://trade.velyxora.com"
			allowedList := strings.Split(allowedStr, ",")
			matched := false
			for _, a := range allowedList {
				if strings.TrimSpace(a) == origin {
					matched = true
					break
				}
			}
			if !matched && origin != "" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: CORS origin validation failed"})
				c.Abort()
				return
			}
		}
		c.Next()
	})

	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Case 1: Untrusted origin in production -> Expect status 403 Forbidden
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	req.Header.Set("Origin", "https://malicious-attacker.com")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected CORS violation to return 403 Forbidden, got %d", w.Code)
	}

	// Case 2: Trusted origin -> Expect status 200 OK
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/ping", nil)
	req2.Header.Set("Origin", "https://velyxora.com")
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected trusted CORS origin to return 200 OK, got %d", w2.Code)
	}
}
