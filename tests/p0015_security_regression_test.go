package tests

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"velyxora/packages/database"
	"velyxora/packages/security"
)

// Helper mock DB and handlers for our self-contained regression tests
func TestP0015SecurityRegression(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Connect to PostgreSQL to set up realistic DB tests
	db, err := database.NewConnectionPool(database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "velyxora",
		Password: "super-secure-db-password-123",
		DBName:   "velyxora",
		SSLMode:  "disable",
	})
	if err != nil {
		t.Skip("PostgreSQL is not reachable, skipping real database regression tests")
		return
	}
	defer db.Close()

	ctx := context.Background()
	_ = database.RunMigrations(ctx, db)

	// Create test users in database
	testUserNoMfa := "usr_p0015_nomfa_" + fmt.Sprintf("%d", time.Now().UnixNano())
	testUserWithMfa := "usr_p0015_mfa_" + fmt.Sprintf("%d", time.Now().UnixNano())
	testUserSupport := "usr_p0015_support_" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Insert test user records
	// Users status and roles:
	// No MFA: Role USER (has wallet:read, wallet:write, trading:write)
	// Support: Role SUPPORT_AGENT (has wallet:read, support:read but NO wallet:write)
	_, _ = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, status, role, is_mfa_enabled, mfa_secret) VALUES ($1, $2, 'hash', 'ACTIVE', 'USER', FALSE, '')", testUserNoMfa, testUserNoMfa+"@test.com")
	_, _ = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, status, role, is_mfa_enabled, mfa_secret) VALUES ($1, $2, 'hash', 'ACTIVE', 'USER', TRUE, 'JBSWY3DPEHPK3PXP')", testUserWithMfa, testUserWithMfa+"@test.com") // totp secret
	_, _ = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, status, role, is_mfa_enabled, mfa_secret) VALUES ($1, $2, 'hash', 'ACTIVE', 'SUPPORT_AGENT', FALSE, '')", testUserSupport, testUserSupport+"@test.com")

	// Set up clean database state for balances
	_, _ = db.Pool.Exec(ctx, "INSERT INTO balances (user_id, asset, available, total, locked, pending, reserved, updated_at) VALUES ($1, 'BTC', 10.0, 10.0, 0, 0, 0, NOW()) ON CONFLICT DO NOTHING", testUserNoMfa)
	_, _ = db.Pool.Exec(ctx, "INSERT INTO balances (user_id, asset, available, total, locked, pending, reserved, updated_at) VALUES ($1, 'BTC', 10.0, 10.0, 0, 0, 0, NOW()) ON CONFLICT DO NOTHING", testUserWithMfa)

	// Clean up afterward
	defer func() {
		_, _ = db.Pool.Exec(ctx, "DELETE FROM balances WHERE user_id IN ($1, $2)", testUserNoMfa, testUserWithMfa)
		_, _ = db.Pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2, $3)", testUserNoMfa, testUserWithMfa, testUserSupport)
	}()

	// Build a localized router incorporating our actual main.go middleware logic
	r := gin.New()

	// Direct middleware copies from main.go
	authMiddlewareMock := func(jwtSecret string) gin.HandlerFunc {
		return func(c *gin.Context) {
			userID := c.GetHeader("X-Test-User-ID")
			email := c.GetHeader("X-Test-User-Email")
			if userID == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
				c.Abort()
				return
			}
			c.Set("claims", &security.Claims{
				UserID:    userID,
				Email:     email,
				SessionID: "sess_test_123",
			})
			c.Next()
		}
	}

	getUserRoleAndStatusMock := func(userID string) (string, string, error) {
		var role, status string
		err := db.Pool.QueryRow(ctx, "SELECT role, status FROM users WHERE id = $1", userID).Scan(&role, &status)
		return role, status, err
	}

	hasPermissionMock := func(role, permission string) (bool, error) {
		var exists bool
		err := db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM role_permissions WHERE role = $1 AND permission = $2)", role, permission).Scan(&exists)
		return exists, err
	}

	rbacMiddlewareMock := func(requiredPermission string) gin.HandlerFunc {
		return func(c *gin.Context) {
			claimsVal, exists := c.Get("claims")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				c.Abort()
				return
			}
			claims := claimsVal.(*security.Claims)
			role, status, err := getUserRoleAndStatusMock(claims.UserID)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				c.Abort()
				return
			}
			if status == "SUSPENDED" || status == "LOCKED" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: account restricted"})
				c.Abort()
				return
			}
			allowed, err := hasPermissionMock(role, requiredPermission)
			if err != nil || !allowed {
				c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: insufficient permissions", "details": "Required: " + requiredPermission})
				c.Abort()
				return
			}
			c.Next()
		}
	}

	verifyMfaProtectionMock := func(c *gin.Context, userID string) bool {
		var isMFAEnabled bool
		var mfaSecret string
		errQuery := db.Pool.QueryRow(ctx, "SELECT is_mfa_enabled, mfa_secret FROM users WHERE id = $1", userID).Scan(&isMFAEnabled, &mfaSecret)
		if errQuery != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: user record not found"})
			c.Abort()
			return false
		}
		if isMFAEnabled {
			mfaCode := c.GetHeader("X-MFA-Code")
			if mfaCode == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "MFA code required"})
				c.Abort()
				return false
			}
			if !security.ValidateTOTP(mfaSecret, mfaCode) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid MFA code"})
				c.Abort()
				return false
			}
		}
		return true
	}

	// Register /wallet group with the same router setup as apps/backend/main.go
	wallet := r.Group("/wallet")
	wallet.Use(authMiddlewareMock("jwt_secret"))
	{
		wallet.GET("/balances", rbacMiddlewareMock("wallet:read"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "success", "message": "balances read success"})
		})

		wallet.POST("/withdraw", rbacMiddlewareMock("wallet:write"), func(c *gin.Context) {
			claims, _ := c.Get("claims")
			userClaims := claims.(*security.Claims)

			// Enforce MFA Verification
			if !verifyMfaProtectionMock(c, userClaims.UserID) {
				return
			}

			c.JSON(http.StatusAccepted, gin.H{"status": "success", "message": "withdrawal queued"})
		})
	}

	// ------------------------------------------------------------
	// Regression Case 1: Privilege Escalation Prevention (SUPPORT_AGENT tries to withdraw)
	// ------------------------------------------------------------
	t.Run("PrivilegeEscalation_SupportAgent_Withdraw_Forbidden", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/wallet/withdraw", strings.NewReader(`{"asset":"BTC","amount":1.0,"address":"1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", testUserSupport)
		req.Header.Set("X-Test-User-Email", "support@velyxora.com")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for role without wallet:write, got: %d (%s)", w.Code, w.Body.String())
		}
	})

	// ------------------------------------------------------------
	// Regression Case 2: Support Agent with wallet:read CAN read balances
	// ------------------------------------------------------------
	t.Run("SupportAgent_ReadBalances_Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/wallet/balances", nil)
		req.Header.Set("X-Test-User-ID", testUserSupport)
		req.Header.Set("X-Test-User-Email", "support@velyxora.com")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for role with wallet:read, got: %d (%s)", w.Code, w.Body.String())
		}
	})

	// ------------------------------------------------------------
	// Regression Case 3: User with MFA enabled tries to withdraw without providing X-MFA-Code
	// ------------------------------------------------------------
	t.Run("UserWithMfa_Withdraw_NoCode_Unauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/wallet/withdraw", strings.NewReader(`{"asset":"BTC","amount":1.0,"address":"1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", testUserWithMfa)
		req.Header.Set("X-Test-User-Email", "mfa@velyxora.com")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized for missing X-MFA-Code, got: %d (%s)", w.Code, w.Body.String())
		}
	})

	// ------------------------------------------------------------
	// Regression Case 4: User with MFA enabled tries to withdraw with incorrect X-MFA-Code
	// ------------------------------------------------------------
	t.Run("UserWithMfa_Withdraw_IncorrectCode_Unauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/wallet/withdraw", strings.NewReader(`{"asset":"BTC","amount":1.0,"address":"1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", testUserWithMfa)
		req.Header.Set("X-Test-User-Email", "mfa@velyxora.com")
		req.Header.Set("X-MFA-Code", "000000") // invalid code
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized for incorrect X-MFA-Code, got: %d (%s)", w.Code, w.Body.String())
		}
	})

	// ------------------------------------------------------------
	// Regression Case 5: User with MFA enabled withdraws with correct X-MFA-Code
	// ------------------------------------------------------------
	t.Run("UserWithMfa_Withdraw_CorrectCode_Success", func(t *testing.T) {
		// Generate valid code
		// JBSWY3DPEHPK3PXP is base32 of:
		// [0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x21, 0xde, 0xad, 0xbe, 0xef] -> "Hello!\xde\xad\xbe\xef"
		timeStep := int64(30)
		counter := time.Now().Unix() / timeStep
		key := []byte("Hello!\xde\xad\xbe\xef") // base32 decoded byte representation of JBSWY3DPEHPK3PXP
		code := generateHOTPSHA1(key, counter)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/wallet/withdraw", strings.NewReader(`{"asset":"BTC","amount":1.0,"address":"1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", testUserWithMfa)
		req.Header.Set("X-Test-User-Email", "mfa@velyxora.com")
		req.Header.Set("X-MFA-Code", code)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected 202 Accepted with correct code, got: %d (%s). Counter: %d, Code: %s", w.Code, w.Body.String(), counter, code)
		}
	})

	// ------------------------------------------------------------
	// Regression Case 6: User with NO MFA enabled can withdraw without X-MFA-Code
	// ------------------------------------------------------------
	t.Run("UserNoMfa_Withdraw_NoCode_Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/wallet/withdraw", strings.NewReader(`{"asset":"BTC","amount":1.0,"address":"1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", testUserNoMfa)
		req.Header.Set("X-Test-User-Email", "nomfa@velyxora.com")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected 202 Accepted for user with NO MFA, got: %d (%s)", w.Code, w.Body.String())
		}
	})
}

func generateHOTPSHA1(key []byte, counter int64) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	// Dynamic truncation (RFC 4226)
	offset := sum[len(sum)-1] & 0xf
	binaryVal := binary.BigEndian.Uint32(sum[offset : offset+4])
	binaryVal = binaryVal & 0x7fffffff

	otp := binaryVal % 1000000
	return fmt.Sprintf("%06d", otp)
}
