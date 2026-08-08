package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"velyxora/packages/security"
)

func TestComplianceREST_EndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	secret := "testsecret12345678"

	// Mock global DB & Kafka & Compliance engine setup for REST testing
	globalComplianceEngine = security.NewComplianceEngine(nil, nil)

	v1 := r.Group("/api/v1")
	compliance := v1.Group("/compliance")
	compliance.Use(authMiddleware(secret))
	{
		compliance.GET("/kyc/status", func(c *gin.Context) {
			claims, _ := c.Get("claims")
			userClaims := claims.(*security.Claims)
			profile, _ := globalComplianceEngine.GetKYC(c.Request.Context(), userClaims.UserID)
			c.JSON(http.StatusOK, profile)
		})

		compliance.POST("/kyc/submit", func(c *gin.Context) {
			var req struct {
				Tier    string `json:"tier" binding:"required"`
				DocMeta string `json:"doc_meta" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			claims, _ := c.Get("claims")
			userClaims := claims.(*security.Claims)
			profile, err := globalComplianceEngine.SubmitKYC(c.Request.Context(), userClaims.UserID, security.KYCTier(strings.ToUpper(req.Tier)), req.DocMeta)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, profile)
		})

		compliance.GET("/restrictions", func(c *gin.Context) {
			claims, _ := c.Get("claims")
			userClaims := claims.(*security.Claims)
			rest, _ := globalComplianceEngine.GetRestriction(c.Request.Context(), userClaims.UserID)
			c.JSON(http.StatusOK, gin.H{"restriction_type": rest})
		})

		compliance.GET("/risk", func(c *gin.Context) {
			claims, _ := c.Get("claims")
			userClaims := claims.(*security.Claims)
			eval, _ := globalComplianceEngine.EvaluateRiskScore(c.Request.Context(), userClaims.UserID)
			c.JSON(http.StatusOK, eval)
		})
	}

	// Authenticate JWT - Using sess_123 as the session ID to bypass mocked session revocation filter
	access, _, _ := security.GenerateJWT("usr_comp_test", "comp@velyxora.com", "sess_123", secret, 5*time.Minute, 1*time.Hour)

	// 1. Get initial KYC status (Should be BASIC/PENDING)
	wStatus := httptest.NewRecorder()
	reqStatus, _ := http.NewRequest("GET", "/api/v1/compliance/kyc/status", nil)
	reqStatus.Header.Set("Authorization", "Bearer "+access)
	r.ServeHTTP(wStatus, reqStatus)

	if wStatus.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d. Body: %s", wStatus.Code, wStatus.Body.String())
	}
	var kStatus security.KYCProfile
	_ = json.Unmarshal(wStatus.Body.Bytes(), &kStatus)
	if kStatus.Status != security.StatusPending {
		t.Errorf("Expected initial status PENDING, got %s", kStatus.Status)
	}

	// 2. Submit KYC
	wSubmit := httptest.NewRecorder()
	payload := `{"tier":"STANDARD","doc_meta":"{\"passport_id\":\"B12345\"}"}`
	reqSubmit, _ := http.NewRequest("POST", "/api/v1/compliance/kyc/submit", bytes.NewReader([]byte(payload)))
	reqSubmit.Header.Set("Authorization", "Bearer "+access)
	reqSubmit.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wSubmit, reqSubmit)

	if wSubmit.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", wSubmit.Code)
	}
	var kSubmit security.KYCProfile
	_ = json.Unmarshal(wSubmit.Body.Bytes(), &kSubmit)
	if kSubmit.Status != security.StatusInReview {
		t.Errorf("Expected status IN_REVIEW, got %s", kSubmit.Status)
	}

	// 3. Get initial restrictions
	wRest := httptest.NewRecorder()
	reqRest, _ := http.NewRequest("GET", "/api/v1/compliance/restrictions", nil)
	reqRest.Header.Set("Authorization", "Bearer "+access)
	r.ServeHTTP(wRest, reqRest)

	if wRest.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", wRest.Code)
	}
	var restResp map[string]string
	_ = json.Unmarshal(wRest.Body.Bytes(), &restResp)
	if restResp["restriction_type"] != string(security.RestrictionNormal) {
		t.Errorf("Expected restriction type NORMAL, got %s", restResp["restriction_type"])
	}

	// 4. Evaluate Risk Score REST response
	wRisk := httptest.NewRecorder()
	reqRisk, _ := http.NewRequest("GET", "/api/v1/compliance/risk", nil)
	reqRisk.Header.Set("Authorization", "Bearer "+access)
	r.ServeHTTP(wRisk, reqRisk)

	if wRisk.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", wRisk.Code)
	}
	var rEval security.RiskEvaluation
	_ = json.Unmarshal(wRisk.Body.Bytes(), &rEval)
	if rEval.Score <= 0.0 || rEval.RiskLevel == "" {
		t.Errorf("Invalid risk REST response: %+v", rEval)
	}
}

func TestMockDepositREST_Gate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	secret := "testsecret12345678"

	// Mock global engine
	globalComplianceEngine = security.NewComplianceEngine(nil, nil)

	v1 := r.Group("/api/v1")
	wallet := v1.Group("/wallet")
	wallet.Use(authMiddleware(secret))
	{
		wallet.POST("/deposits/mock", func(c *gin.Context) {
			var req struct {
				Asset   string  `json:"asset" binding:"required"`
				Amount  float64 `json:"amount" binding:"required,gt=0"`
				Address string  `json:"address" binding:"required"`
				TxHash  string  `json:"tx_hash" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			claims, _ := c.Get("claims")
			userClaims := claims.(*security.Claims)

			sStatus, _, _ := globalComplianceEngine.ScreenSanctions(c.Request.Context(), "ADDRESS_DEPOSIT", req.Address)
			status := "credited"
			if sStatus == security.SanctionsMatch {
				status = "held"
			}

			c.JSON(http.StatusOK, gin.H{
				"deposit_id": "dep_123",
				"user_id": userClaims.UserID,
				"compliance_status": status,
			})
		})
	}

	// Authenticate JWT - Using sess_123 as the session ID to bypass mocked session revocation filter
	access, _, _ := security.GenerateJWT("usr_dep_test", "dep@velyxora.com", "sess_123", secret, 5*time.Minute, 1*time.Hour)

	// Clear normal deposit
	wDep := httptest.NewRecorder()
	payload := `{"asset":"ETH","amount":1.5,"address":"0x71C7656EC7ab88b098defB751B7401B5f6d1476B","tx_hash":"0xabc"}`
	reqDep, _ := http.NewRequest("POST", "/api/v1/wallet/deposits/mock", bytes.NewReader([]byte(payload)))
	reqDep.Header.Set("Authorization", "Bearer "+access)
	reqDep.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wDep, reqDep)

	if wDep.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d. Body: %s", wDep.Code, wDep.Body.String())
	}
	var res map[string]interface{}
	_ = json.Unmarshal(wDep.Body.Bytes(), &res)
	if res["compliance_status"] != "credited" {
		t.Errorf("Expected compliance status 'credited', got %v", res["compliance_status"])
	}

	// Blacklisted sanctioned address deposit
	wDepSusp := httptest.NewRecorder()
	payloadSusp := `{"asset":"ETH","amount":1.5,"address":"0xBLACKLISTED","tx_hash":"0xabc_susp"}`
	reqDepSusp, _ := http.NewRequest("POST", "/api/v1/wallet/deposits/mock", bytes.NewReader([]byte(payloadSusp)))
	reqDepSusp.Header.Set("Authorization", "Bearer "+access)
	reqDepSusp.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wDepSusp, reqDepSusp)

	if wDepSusp.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d. Body: %s", wDepSusp.Code, wDepSusp.Body.String())
	}
	_ = json.Unmarshal(wDepSusp.Body.Bytes(), &res)
	if res["compliance_status"] != "held" {
		t.Errorf("Expected compliance status 'held', got %v", res["compliance_status"])
	}
}
