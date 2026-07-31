package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
