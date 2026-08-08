package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"velyxora/packages/security"
)

func TestIncidentManagementEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		// Mock authentication trace/correlation ID
		c.Set("trace_id", "tr_test_incident")
		c.Next()
	})

	v1 := r.Group("/api/v1")
	incGroup := v1.Group("/incidents")
	{
		incGroup.GET("", handleGetIncidents)
		incGroup.GET("/:id", handleGetIncidentByID)
		incGroup.POST("", handleCreateIncident)
		incGroup.PATCH("/:id", handlePatchIncident)
	}

	// 1. Create an incident via POST
	w1 := httptest.NewRecorder()
	payload := `{"severity":"CRITICAL","source":"kafka-monitor","description":"Kafka connection lost","affected_service":"matching-engine"}`
	req1, _ := http.NewRequest("POST", "/api/v1/incidents", bytes.NewReader([]byte(payload)))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Errorf("Expected status 201 Created, got %d", w1.Code)
	}

	var incident Incident
	_ = json.Unmarshal(w1.Body.Bytes(), &incident)
	if incident.Severity != "CRITICAL" || incident.AffectedService != "matching-engine" || incident.Status != "OPEN" {
		t.Errorf("Unexpected incident fields: %+v", incident)
	}

	// 2. Fetch specific incident by ID
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/incidents/"+incident.ID, nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", w2.Code)
	}

	// 3. Patch incident
	w3 := httptest.NewRecorder()
	patchPayload := `{"status":"INVESTIGATING","assignee":"admin_jules","resolution_notes":"Triage under progress"}`
	req3, _ := http.NewRequest("PATCH", "/api/v1/incidents/"+incident.ID, bytes.NewReader([]byte(patchPayload)))
	req3.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", w3.Code)
	}

	// 4. Retrieve all incidents list
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("GET", "/api/v1/incidents", nil)
	r.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", w4.Code)
	}
}

func TestGetAuditEventsMock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/api/v1/audit/events", handleGetAuditEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit/events", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for mock audit events list, got %d", w.Code)
	}
}

func TestRBACMiddlewareEnforcement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Setup mock DB for role verification
	r.GET("/protected", func(c *gin.Context) {
		// Mock claims context
		claims := &security.Claims{
			UserID: "usr_mock_admin",
		}
		c.Set("claims", claims)
		c.Next()
	}, RBACMiddleware("system:admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "accessed"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	// DB is absent, so it should abort/fail validation smoothly
	if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Errorf("Expected status Unauthorized or Forbidden when DB is absent, got %d", w.Code)
	}
}
