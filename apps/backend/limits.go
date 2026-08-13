package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"velyxora/packages/common"
	"velyxora/packages/security"
)

const slidingWindowLua = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local weight = tonumber(ARGV[4])
local member = ARGV[5]

redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

local entries = redis.call('ZRANGEBYSCORE', key, now - window, now)
local current_weight = 0

for _, entry in ipairs(entries) do
    local first_colon = string.find(entry, ":")
    if first_colon then
        local second_colon = string.find(entry, ":", first_colon + 1)
        if second_colon then
            local w_str = string.sub(entry, first_colon + 1, second_colon - 1)
            local w = tonumber(w_str)
            if w then
                current_weight = current_weight + w
            end
        end
    end
end

if current_weight + weight > limit then
    local oldest = redis.call('ZRANGEBYSCORE', key, now - window, now, 'LIMIT', 0, 1)
    local oldest_ts = now - window
    if oldest[1] then
        local first_colon = string.find(oldest[1], ":")
        if first_colon then
            oldest_ts = tonumber(string.sub(oldest[1], 1, first_colon - 1)) or oldest_ts
        end
    end
    local retry_after = math.max(1, math.ceil(((oldest_ts + window) - now) / 1000))
    return {0, current_weight, retry_after}
else
    redis.call('ZADD', key, now, member)
    redis.call('EXPIRE', key, math.ceil(window / 1000) + 1)
    return {1, current_weight + weight, 0}
end
`

// GetRequestWeight calculates the weight of the request based on the path, method, and queries.
func GetRequestWeight(method, path string, c *gin.Context) int {
	method = strings.ToUpper(method)
	path = strings.TrimSuffix(path, "/")

	if method == "GET" && (path == "/api/v1/market/depth" || path == "/market/depth") {
		if c != nil {
			limitStr := c.Query("limit")
			if limitStr == "" {
				return 50 // default/max levels
			}
			limit, err := strconv.Atoi(limitStr)
			if err != nil || limit > 20 {
				return 50 // max levels
			}
			return 20
		}
		return 50
	}

	if method == "GET" && (path == "/api/v1/market/trades" || path == "/market/trades") {
		return 5
	}

	if method == "POST" && (path == "/api/v1/oms/orders" || path == "/oms/orders" || path == "/api/v1/trading/orders" || path == "/trading/orders") {
		return 2
	}

	if method == "GET" && (path == "/api/v1/system/health" || path == "/api/v1/oms/system/health" || path == "/system/health" || path == "/health" || path == "/health/live" || path == "/health/ready") {
		return 1
	}

	return 1 // Default weight for other endpoints
}

// Package level hook for testing
var redisEvalHook func(ctx context.Context, key string, now, window, limit, weight int64, member string) ([]interface{}, error)

func redisEval(ctx context.Context, key string, now, window, limit, weight int64, member string) ([]interface{}, error) {
	if redisEvalHook != nil {
		return redisEvalHook(ctx, key, now, window, limit, weight, member)
	}
	if globalRedis == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	res, err := globalRedis.Client.Eval(ctx, slidingWindowLua, []string{key}, now, window, limit, weight, member).Result()
	if err != nil {
		return nil, err
	}
	resSlice, ok := res.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response type")
	}
	return resSlice, nil
}

// RequestWeightLimiter applies dynamic Redis-backed rate limiting with API Request Weights.
func RequestWeightLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if globalRedis == nil && redisEvalHook == nil {
			// Fail open if Redis is offline to prevent system outage
			c.Next()
			return
		}

		// 1. Identify client
		var identifier string
		// Check X-API-KEY header first
		apiKey := c.GetHeader("X-API-KEY")
		if apiKey != "" {
			identifier = "apikey:" + apiKey
		} else {
			// Check if JWT auth has already run and claims are available
			claimsVal, exists := c.Get("claims")
			if exists {
				if claims, ok := claimsVal.(*security.Claims); ok {
					identifier = "user:" + claims.UserID
				}
			}
		}

		// Fallback to client IP if no authenticated ID found
		if identifier == "" {
			identifier = "ip:" + c.ClientIP()
		}

		key := "rl:weight:" + identifier

		// 2. Query dynamic weight
		weight := GetRequestWeight(c.Request.Method, c.Request.URL.Path, c)

		// 3. Execute Lua sliding window rate limiter
		ctx := c.Request.Context()
		now := time.Now().UnixMilli()
		window := int64(60000) // 1 minute in ms
		limit := int64(1200)

		// Prepare unique member
		randVal := time.Now().UnixNano()
		member := fmt.Sprintf("%d:%d:%d", now, weight, randVal)

		// Run Lua script
		resSlice, err := redisEval(ctx, key, now, window, limit, int64(weight), member)
		if err != nil {
			// Fail open on Redis error
			c.Next()
			return
		}

		if len(resSlice) < 3 {
			c.Next()
			return
		}

		allowed := resSlice[0].(int64) == 1
		usedWeight := resSlice[1].(int64)
		retryAfter := resSlice[2].(int64)

		// Set Binance-compatible header representing current used weight in the current minute
		c.Header("X-MBX-USED-WEIGHT-(1m)", fmt.Sprintf("%d", usedWeight))

		if !allowed {
			// Increment observability counter
			common.GetObservabilityManager().RateLimitEventsTotal.WithLabelValues(c.Request.URL.Path).Inc()

			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many requests. API weight limit exceeded.",
				"limit":       limit,
				"used_weight": usedWeight,
				"retry_after": retryAfter,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

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
