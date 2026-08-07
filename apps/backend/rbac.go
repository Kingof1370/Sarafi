package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"velyxora/packages/security"
)

// GetUserRoleAndStatus fetches the real-time role and status of the user from the database.
func GetUserRoleAndStatus(userID string) (string, string, error) {
	if globalDB == nil {
		return "", "", errors.New("database connection pool not initialized")
	}
	var role string
	var status string
	err := globalDB.Pool.QueryRow(context.Background(), "SELECT role, status FROM users WHERE id = $1", userID).Scan(&role, &status)
	if err != nil {
		return "", "", err
	}
	return role, status, nil
}

// HasPermission checks if a given role has the requested fine-grained permission in the database.
func HasPermission(role string, permission string) (bool, error) {
	if globalDB == nil {
		return false, errors.New("database connection pool not initialized")
	}
	var exists bool
	err := globalDB.Pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM role_permissions WHERE role = $1 AND permission = $2)",
		role, permission).Scan(&exists)
	return exists, err
}

// RBACMiddleware enforces server-side database-backed role/permission checks on REST endpoints.
func RBACMiddleware(requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Get authenticated claims
		claimsVal, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: user identity not verified"})
			c.Abort()
			return
		}

		claims, ok := claimsVal.(*security.Claims)
		if !ok || claims.UserID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid claims payload"})
			c.Abort()
			return
		}

		// 2. Verify API Key Scopes if authenticated via API key
		if scopesVal, exists := c.Get("apikey_permissions"); exists {
			scopesStr, ok := scopesVal.(string)
			if !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: invalid API key scopes format"})
				c.Abort()
				return
			}
			allowed := false
			for _, s := range strings.Split(scopesStr, ",") {
				if strings.TrimSpace(s) == requiredPermission {
					allowed = true
					break
				}
			}
			if !allowed {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Forbidden: API key has insufficient scopes",
					"details": "Required scope: " + requiredPermission,
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// 3. Fetch real-time Role and Status for JWT session users
		role, status, err := GetUserRoleAndStatus(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: failed to verify security status"})
			c.Abort()
			return
		}

		// Prevent restricted/suspended/locked users from making requests
		if status == "SUSPENDED" || status == "LOCKED" || status == "DELETED" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: account is " + status})
			c.Abort()
			return
		}

		// Save the role to context so handlers can access it
		c.Set("user_role", role)

		// 4. Verify Fine-Grained Permission
		allowed, err := HasPermission(role, requiredPermission)
		if err != nil || !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Forbidden: insufficient permissions",
				"details": "Required permission: " + requiredPermission,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
