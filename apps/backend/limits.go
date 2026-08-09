package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"velyxora/packages/common"
	"velyxora/packages/security"
)

// RedisRateLimit checks if a key exceeds the specified rate limit over a sliding window duration.
func RedisRateLimit(key string, limit int, window time.Duration) bool {
	if globalRedis == nil {
		// Fallback to open state if Redis is offline to prevent system outage
		return true
	}

	ctx := context.Background()
	now := time.Now()
	nowNano := now.UnixNano()
	cutoff := now.Add(-window).UnixNano()
	nowStr := fmt.Sprintf("%d", nowNano)

	// Sliding window implementation using Redis Sorted Sets (ZSET)
	pipe := globalRedis.Client.TxPipeline()
	// 1. Remove old requests outside the sliding window
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", cutoff))
	// 2. Count current elements inside the sliding window
	pipe.ZCard(ctx, key)
	// 3. Add the current request
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(nowNano), Member: nowStr})
	// 4. Update the expiration of the sorted set to keep Redis clean
	pipe.Expire(ctx, key, window)

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		// Fail open in case of Redis command errors
		return true
	}

	countCmd, ok := cmds[1].(*redis.IntCmd)
	if !ok {
		return true
	}

	count, err := countCmd.Result()
	if err != nil {
		return true
	}

	if count > int64(limit) {
		// Remove the last added element if rate limit was exceeded
		_ = globalRedis.Client.ZRem(ctx, key, nowStr)
		return false
	}

	return true
}

// RateLimiterMiddleware applies dynamic rate limits based on auth state and endpoint category.
func RateLimiterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		path := c.Request.URL.Path

		var key string
		var limit int
		var window time.Duration

		// Determine endpoint categories and limits
		if path == "/api/v1/auth/login" || path == "/api/v1/auth/register" {
			// Auth login/register limits: Max 10 requests per minute to prevent abuse
			key = "rl:auth:" + clientIP
			limit = 10
			window = time.Minute
		} else {
			// Check if authenticated
			claimsVal, exists := c.Get("claims")
			if exists {
				// Authenticated endpoints: Check if using API Key or JWT session
				if scopes, isApiKey := c.Get("apikey_permissions"); isApiKey {
					// API Keys limits: High-speed programmatic access (120 reqs/min)
					key = "rl:apikey:" + c.GetHeader("X-API-KEY")
					limit = 120
					_ = scopes // keep compiler happy
				} else {
					// Standard session users limits: (60 reqs/min)
					claims := claimsVal.(*security.Claims)
					key = "rl:user:" + claims.UserID
					limit = 60
				}
				window = time.Minute
			} else {
				// Unauthenticated generic public access: (30 reqs/min)
				key = "rl:public:" + clientIP
				limit = 30
				window = time.Minute
			}
		}

		if !RedisRateLimit(key, limit, window) {
			common.GetObservabilityManager().RateLimitEventsTotal.WithLabelValues(path).Inc()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Too many requests. Please slow down.",
				"limit":   limit,
				"window":  window.String(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CheckBruteForceAndLock verifies if an identity (account or IP) is temporarily locked.
func CheckBruteForceAndLock(identityKey string) (bool, time.Time, error) {
	if globalDB == nil {
		return false, time.Time{}, nil
	}

	ctx := context.Background()
	var lockedUntil *time.Time
	err := globalDB.Pool.QueryRow(ctx,
		"SELECT locked_until FROM brute_force_lockouts WHERE identity_key = $1",
		identityKey).Scan(&lockedUntil)

	if err != nil {
		// Identity not found in lockout tracking, therefore not locked
		return false, time.Time{}, nil
	}

	if lockedUntil != nil && lockedUntil.After(time.Now()) {
		return true, *lockedUntil, nil
	}

	return false, time.Time{}, nil
}

// RecordLockoutFailure records a failed login/auth attempt and applies lockout if threshold of 5 is met.
func RecordLockoutFailure(identityKey string) error {
	if globalDB == nil {
		return nil
	}

	ctx := context.Background()
	var failedAttempts int
	err := globalDB.Pool.QueryRow(ctx,
		"SELECT failed_attempts FROM brute_force_lockouts WHERE identity_key = $1",
		identityKey).Scan(&failedAttempts)

	if err != nil {
		// Insert initial failure row
		id := "lko_" + fmt.Sprintf("%d", time.Now().UnixNano())
		_, err = globalDB.Pool.Exec(ctx,
			"INSERT INTO brute_force_lockouts (id, identity_key, failed_attempts, locked_until, updated_at) VALUES ($1, $2, 1, NULL, NOW())",
			id, identityKey)
		return err
	}

	failedAttempts++
	var lockedUntil *time.Time
	if failedAttempts >= 5 {
		lockTime := time.Now().Add(15 * time.Minute)
		lockedUntil = &lockTime

		// Record the lockout event in durable audit logs
		auditID := "aud_lko_" + fmt.Sprintf("%d", time.Now().UnixNano())
		_, _ = globalDB.Pool.Exec(ctx,
			`INSERT INTO security_audit_logs (id, event_type, severity, details, metadata)
			 VALUES ($1, 'BRUTE_FORCE_LOCKOUT', 'HIGH', $2, '{}')`,
			auditID, "Lockout triggered for identity: "+identityKey+" due to 5 consecutive failures")
	}

	_, err = globalDB.Pool.Exec(ctx,
		"UPDATE brute_force_lockouts SET failed_attempts = $1, locked_until = $2, updated_at = NOW() WHERE identity_key = $3",
		failedAttempts, lockedUntil, identityKey)
	return err
}

// ResetLockoutFailures clears any failure tracking and lockouts for a successful identity action.
func ResetLockoutFailures(identityKey string) error {
	if globalDB == nil {
		return nil
	}
	ctx := context.Background()
	_, err := globalDB.Pool.Exec(ctx,
		"UPDATE brute_force_lockouts SET failed_attempts = 0, locked_until = NULL, updated_at = NOW() WHERE identity_key = $1",
		identityKey)
	return err
}
