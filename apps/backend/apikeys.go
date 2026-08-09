package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"velyxora/packages/security"
)

var (
	globalKeyManager *security.KeyManager
	kmOnce           sync.Once
)

func GetKeyManager() *security.KeyManager {
	kmOnce.Do(func() {
		secretStr := os.Getenv("API_KEY_MASTER_SECRET")
		if secretStr == "" {
			secretStr = "velyxora-apikeys-master-key-32b"
		}

		versions := map[string]string{
			"1": secretStr,
		}

		// Load other dynamic key versions if configured for rotation
		for i := 2; i <= 10; i++ {
			verKey := os.Getenv(fmt.Sprintf("API_KEY_VERSION_%d", i))
			if verKey != "" {
				versions[fmt.Sprintf("%d", i)] = verKey
			}
		}

		activeVer := os.Getenv("API_KEY_ACTIVE_VERSION")
		if activeVer == "" {
			activeVer = "1"
		}

		km, err := security.NewKeyManager(versions, activeVer, "velyxora-apikeys-master-key-32b")
		if err != nil {
			panic(fmt.Sprintf("failed to initialize security cryptographic key manager: %v", err))
		}
		globalKeyManager = km
	})
	return globalKeyManager
}

// EncryptSecret encrypts a raw API secret using centralized KeyManager (supports versioning and rotation).
func EncryptSecret(secret string) (string, error) {
	return GetKeyManager().Encrypt(secret)
}

// DecryptSecret decrypts an encrypted API secret using centralized KeyManager (supports versioning and rotation).
func DecryptSecret(encryptedHex string) (string, error) {
	return GetKeyManager().Decrypt(encryptedHex)
}

// ParseAndValidateIPAllowlist parses a comma-separated list of IP addresses or CIDR blocks and normalizes them.
func ParseAndValidateIPAllowlist(allowlistStr string) ([]string, error) {
	if allowlistStr == "" {
		return nil, nil
	}
	parts := strings.Split(allowlistStr, ",")
	var normalized []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		// Check if it is a valid CIDR block
		_, _, err := net.ParseCIDR(trimmed)
		if err == nil {
			normalized = append(normalized, trimmed)
			continue
		}
		// Check if it is a valid standalone IP
		ip := net.ParseIP(trimmed)
		if ip != nil {
			normalized = append(normalized, trimmed)
			continue
		}
		return nil, fmt.Errorf("invalid IP address or CIDR range: %s", trimmed)
	}
	return normalized, nil
}

// IsIPAllowed checks if a client IP address matches any individual IP or CIDR block in the allowlist.
func IsIPAllowed(allowlist []string, clientIPStr string) bool {
	if len(allowlist) == 0 {
		return true
	}
	clientIP := net.ParseIP(clientIPStr)
	if clientIP == nil {
		return false
	}
	for _, entry := range allowlist {
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err == nil && ipNet.Contains(clientIP) {
				return true
			}
		} else {
			ip := net.ParseIP(entry)
			if ip != nil && ip.Equal(clientIP) {
				return true
			}
		}
	}
	return false
}

// IsNonceReused checks if the given nonce has already been used within the clock skew window.
func IsNonceReused(apiKey, nonce string) bool {
	if globalRedis == nil {
		return false
	}
	ctx := context.Background()
	key := "nonce:" + apiKey + ":" + nonce
	val, err := globalRedis.Get(ctx, key)
	if err == nil && val == "1" {
		return true
	}
	_ = globalRedis.Set(ctx, key, "1", 300*time.Second)
	return false
}

// generateSecureRandomString creates cryptographically secure hex strings.
func generateSecureRandomString(length int) string {
	b := make([]byte, length)
	_, _ = io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}

// RegisterAPIKeyHandlers binds the API Key endpoints under the REST router.
func RegisterAPIKeyHandlers(r *gin.RouterGroup) {
	apikeys := r.Group("/apikeys")
	apikeys.Use(authMiddleware(globalJWTSecret))
	{
		apikeys.POST("", handleCreateAPIKey)
		apikeys.GET("", handleListAPIKeys)
		apikeys.DELETE("/:id", handleRevokeAPIKey)
	}
}

func handleCreateAPIKey(c *gin.Context) {
	if globalDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection unavailable"})
		return
	}

	var req struct {
		Label         string `json:"label" binding:"required"`
		Permissions   string `json:"permissions" binding:"required"` // e.g. "wallet:read,trading:write"
		IPAllowlist   string `json:"ip_allowlist"`                   // e.g. "127.0.0.1,192.168.1.0/24"
		ExpiresInDays int    `json:"expires_in_days" binding:"required,gt=0"`
		MFACode       string `json:"mfa_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request fields", "details": err.Error()})
		return
	}

	claims, _ := c.Get("claims")
	userClaims := claims.(*security.Claims)

	// Validate and Normalize IP Allowlist
	normalizedIPs, err := ParseAndValidateIPAllowlist(req.IPAllowlist)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP allowlist format invalid", "details": err.Error()})
		return
	}
	normalizedIPListStr := strings.Join(normalizedIPs, ",")

	// P0003 MFA Integration Requirement: Creation of API key requires verification of MFA code
	var isMfaEnabled bool
	var mfaSecret string
	err = globalDB.Pool.QueryRow(context.Background(),
		"SELECT is_mfa_enabled, mfa_secret FROM users WHERE id = $1", userClaims.UserID).
		Scan(&isMfaEnabled, &mfaSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify security profile"})
		return
	}

	if isMfaEnabled && mfaSecret != "" {
		if !security.ValidateTOTP(mfaSecret, req.MFACode) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "MFA verification failed: invalid code"})
			return
		}
	}

	// Generate API Key & Secret pair
	apiKey := "velyx_key_" + generateSecureRandomString(16)
	apiSecret := "velyx_sec_" + generateSecureRandomString(32)

	encryptedSecret, err := EncryptSecret(apiSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt security credentials"})
		return
	}

	keyID := "key_" + fmt.Sprintf("%d", time.Now().UnixNano())
	expiresAt := time.Now().AddDate(0, 0, req.ExpiresInDays)

	_, err = globalDB.Pool.Exec(context.Background(),
		`INSERT INTO user_api_keys (id, user_id, label, api_key, api_secret_hash, permissions, ip_allowlist, expires_at, created_at, is_revoked)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), FALSE)`,
		keyID, userClaims.UserID, req.Label, apiKey, encryptedSecret, req.Permissions, normalizedIPListStr, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save API key credentials"})
		return
	}

	// Record audit log
	auditID := "aud_key_cre_" + fmt.Sprintf("%d", time.Now().UnixNano())
	_, _ = globalDB.Pool.Exec(context.Background(),
		`INSERT INTO security_audit_logs (id, event_type, severity, user_id, api_key_id, ip_address, user_agent, details, metadata)
		 VALUES ($1, 'API_KEY_CREATED', 'MEDIUM', $2, $3, $4, $5, 'User created a new API key', '{}')`,
		auditID, userClaims.UserID, keyID, c.ClientIP(), c.Request.UserAgent())

	// Return both raw credentials. The API secret will NEVER be exposed again!
	c.JSON(http.StatusCreated, gin.H{
		"id":           keyID,
		"label":        req.Label,
		"api_key":      apiKey,
		"api_secret":   apiSecret,
		"permissions":  req.Permissions,
		"ip_allowlist": normalizedIPListStr,
		"expires_at":   expiresAt,
	})
}

func handleListAPIKeys(c *gin.Context) {
	if globalDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection unavailable"})
		return
	}

	claims, _ := c.Get("claims")
	userClaims := claims.(*security.Claims)

	type APIKeyResp struct {
		ID          string    `json:"id"`
		Label       string    `json:"label"`
		APIKey      string    `json:"api_key"`
		Permissions string    `json:"permissions"`
		IPAllowlist string    `json:"ip_allowlist"`
		ExpiresAt   time.Time `json:"expires_at"`
		CreatedAt   time.Time `json:"created_at"`
		LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	}

	keys := make([]APIKeyResp, 0)

	rows, err := globalDB.Pool.Query(context.Background(),
		`SELECT id, label, api_key, permissions, ip_allowlist, expires_at, created_at, last_used_at
		 FROM user_api_keys WHERE user_id = $1 AND is_revoked = FALSE ORDER BY created_at DESC`,
		userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve API keys"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var k APIKeyResp
		errScan := rows.Scan(&k.ID, &k.Label, &k.APIKey, &k.Permissions, &k.IPAllowlist, &k.ExpiresAt, &k.CreatedAt, &k.LastUsedAt)
		if errScan == nil {
			keys = append(keys, k)
		}
	}

	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

func handleRevokeAPIKey(c *gin.Context) {
	if globalDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection unavailable"})
		return
	}

	claims, _ := c.Get("claims")
	userClaims := claims.(*security.Claims)
	keyID := c.Param("id")

	var ownerID string
	err := globalDB.Pool.QueryRow(context.Background(), "SELECT user_id FROM user_api_keys WHERE id = $1", keyID).Scan(&ownerID)
	if err != nil || ownerID != userClaims.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: you do not own this API key"})
		return
	}

	_, err = globalDB.Pool.Exec(context.Background(), "UPDATE user_api_keys SET is_revoked = TRUE WHERE id = $1", keyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke API key"})
		return
	}

	// Record audit log
	auditID := "aud_key_rev_" + fmt.Sprintf("%d", time.Now().UnixNano())
	_, _ = globalDB.Pool.Exec(context.Background(),
		`INSERT INTO security_audit_logs (id, event_type, severity, user_id, api_key_id, ip_address, user_agent, details, metadata)
		 VALUES ($1, 'API_KEY_REVOKED', 'LOW', $2, $3, $4, $5, 'User revoked an API key', '{}')`,
		auditID, userClaims.UserID, keyID, c.ClientIP(), c.Request.UserAgent())

	c.JSON(http.StatusOK, gin.H{"message": "API key successfully revoked"})
}

// APIKeyAuthMiddleware verifies programmatic HMAC-SHA256 request signatures.
func APIKeyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKeyHeader := c.GetHeader("X-API-KEY")
		sigHeader := c.GetHeader("X-SIGNATURE")
		tsHeader := c.GetHeader("X-TIMESTAMP")
		nonceHeader := c.GetHeader("X-NONCE")

		if apiKeyHeader == "" || sigHeader == "" || tsHeader == "" || nonceHeader == "" {
			c.Next()
			return
		}

		// 1. Clock skew validation (max 300 seconds)
		ts, err := time.Parse(time.RFC3339, tsHeader)
		if err != nil {
			var tsUnix int64
			_, err = fmt.Sscanf(tsHeader, "%d", &tsUnix)
			if err == nil {
				ts = time.Unix(tsUnix, 0)
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid X-TIMESTAMP format"})
				c.Abort()
				return
			}
		}

		skew := time.Since(ts)
		if skew < 0 {
			skew = -skew
		}
		if skew > 300*time.Second {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: request timestamp clock skew exceeds 300 seconds limit"})
			c.Abort()
			return
		}

		// 2. Prevent replay attacks using nonce tracking in Redis
		if IsNonceReused(apiKeyHeader, nonceHeader) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: replay attempt detected; nonce already used"})
			c.Abort()
			return
		}

		// 3. Database verification of API Key
		if globalDB == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection unavailable"})
			c.Abort()
			return
		}

		var userID string
		var email string
		var encryptedSecret string
		var rawPermissions string
		var rawAllowlist string
		var expiresAt time.Time
		var isRevoked bool

		err = globalDB.Pool.QueryRow(context.Background(),
			`SELECT k.user_id, u.email, k.api_secret_hash, k.permissions, k.ip_allowlist, k.expires_at, k.is_revoked
			 FROM user_api_keys k JOIN users u ON k.user_id = u.id WHERE k.api_key = $1`,
			apiKeyHeader).Scan(&userID, &email, &encryptedSecret, &rawPermissions, &rawAllowlist, &expiresAt, &isRevoked)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid API key"})
			c.Abort()
			return
		}

		if isRevoked {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: API key has been revoked"})
			c.Abort()
			return
		}

		if expiresAt.Before(time.Now()) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: API key has expired"})
			c.Abort()
			return
		}

		// 4. Strict IP whitelisting validation
		clientIP := c.ClientIP()
		allowedIPs := make([]string, 0)
		if rawAllowlist != "" {
			allowedIPs = strings.Split(rawAllowlist, ",")
		}
		if !IsIPAllowed(allowedIPs, clientIP) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: request originating from untrusted IP address"})
			c.Abort()
			return
		}

		// 5. Decrypt secret to calculate and verify HMAC-SHA256 signature
		apiSecret, err := DecryptSecret(encryptedSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode security credentials"})
			c.Abort()
			return
		}

		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		bodyStr := string(bodyBytes)

		// Canonical Signing Payload serialization:
		// timestamp + "\n" + nonce + "\n" + HTTP_METHOD + "\n" + REQUEST_PATH + "\n" + REQUEST_BODY
		canonicalPayload := tsHeader + "\n" + nonceHeader + "\n" + c.Request.Method + "\n" + c.Request.URL.Path + "\n" + bodyStr

		mac := hmac.New(sha256.New, []byte(apiSecret))
		mac.Write([]byte(canonicalPayload))
		expectedMAC := mac.Sum(nil)
		expectedSig := hex.EncodeToString(expectedMAC)

		if sigHeader != expectedSig {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: cryptographic signature mismatch"})
			c.Abort()
			return
		}

		// Update last used timestamp
		_, _ = globalDB.Pool.Exec(context.Background(),
			"UPDATE user_api_keys SET last_used_at = NOW() WHERE api_key = $1", apiKeyHeader)

		// 6. Synthesize valid JWT Claims payload inside Context to bypass downstream auth controls cleanly
		synthesizedClaims := &security.Claims{
			UserID:    userID,
			Email:     email,
			SessionID: "apikey_session",
		}
		c.Set("claims", synthesizedClaims)
		c.Set("apikey_permissions", rawPermissions)

		c.Next()
	}
}

// APIKeyRBACMiddleware enforces key scopes on cryptographic endpoints.
func APIKeyRBACMiddleware(requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopesVal, exists := c.Get("apikey_permissions")
		if !exists {
			c.Next()
			return
		}

		scopesStr, ok := scopesVal.(string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: invalid API key scopes format"})
			c.Abort()
			return
		}

		allowed := false
		for _, s := range strings.Split(scopesStr, ",") {
			if strings.TrimSpace(s) == requiredScope {
				allowed = true
				break
			}
		}

		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Forbidden: API key has insufficient scopes",
				"details": "Required scope: " + requiredScope,
			})
			c.Abort()
			return
		}

		c.AbortWithStatus(http.StatusOK)
	}
}
