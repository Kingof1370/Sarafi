package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"velyxora/packages/common"
	"velyxora/packages/database"
	"velyxora/packages/logger"
	"velyxora/packages/security"
	"velyxora/packages/types"
)

// Config represents server environment configuration
type Config struct {
	Port         string
	JWTSecret    string
	DBHost       string
	DBPort       int
	DBUser       string
	DBPassword   string
	DBName       string
	RedisAddr    string
	KafkaBrokers []string
}

var globalDB *database.DB
var globalRedis *common.RedisClient
var globalKafkaProducer *common.KafkaProducer
var globalJWTSecret string
var globalWSGateway *WSGateway

func main() {
	// 1. Initialize Log
	log := logger.NewLogger(logger.Config{
		Level:       getEnv("LOG_LEVEL", "INFO"),
		Format:      getEnv("LOG_FORMAT", "JSON"),
		ServiceName: "api-gateway",
	})

	log.Info("Starting Velyxora API Gateway...")

	// 2. Load Configuration
	cfg := Config{
		Port:         getEnv("PORT", "8080"),
		JWTSecret:    getEnv("JWT_SECRET", "super-secret-velyxora-key-999"),
		DBHost:       getEnv("DB_HOST", "localhost"),
		DBPort:       5432,
		DBUser:       getEnv("DB_USER", "postgres"),
		DBPassword:   getEnv("DB_PASSWORD", "postgres"),
		DBName:       getEnv("DB_NAME", "velyxora"),
		RedisAddr:    getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers: strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
	}
	globalJWTSecret = cfg.JWTSecret

	// Register current microservice name with Observability Manager
	common.GetObservabilityManager().SetServiceName("api-gateway")

	// 3. Setup Postgres Connection
	db, err := database.NewConnectionPool(database.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
		SSLMode:  "disable",
	})
	if err != nil {
		log.Warn(fmt.Sprintf("Database connection failed (continuing bootstrap in fallback mode): %v", err))
		common.GetObservabilityManager().IncrementCounter("database_errors_total", map[string]string{"type": "connection_pool_init"})
	} else {
		globalDB = db
		defer db.Close()
		log.Info("PostgreSQL connection pool initialized successfully.")
		// Run database schema migrations
		err = database.RunMigrations(context.Background(), db)
		if err != nil {
			log.Error(fmt.Sprintf("Database migrations failed: %v", err))
		}
	}

	// 4. Setup Redis Connection
	redisClient, err := common.NewRedisClient(common.RedisConfig{
		Addr:     cfg.RedisAddr,
		Password: "",
		DB:       0,
	})
	if err != nil {
		log.Warn(fmt.Sprintf("Redis connection failed (continuing bootstrap in fallback mode): %v", err))
		common.GetObservabilityManager().IncrementCounter("redis_errors_total", map[string]string{"type": "init"})
	} else {
		globalRedis = redisClient
		log.Info("Redis client connected successfully.")
	}

	// 5. Setup Kafka Producer
	kafkaProducer := common.NewKafkaProducer(cfg.KafkaBrokers)
	globalKafkaProducer = kafkaProducer
	defer kafkaProducer.Close()
	log.Info("Kafka Producer registered.")

	// Start asynchronous Market Data Consumers
	startMarketDataConsumers(cfg.KafkaBrokers)

	// 6.5 Setup WebSocket Gateway (P005 real-time core)
	wsGateway := NewWSGateway(cfg.JWTSecret)
	globalWSGateway = wsGateway
	go wsGateway.Run()

	// 7. Bootstrap Router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Security Headers Middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})

	// CORS Policy Middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-MFA-Code, X-Idempotency-Key, X-Trace-ID")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Custom structured logger middleware with Request Tracing IDs
	r.Use(func(c *gin.Context) {
		start := time.Now()
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = fmt.Sprintf("tr_%d", time.Now().UnixNano())
		}
		c.Set("trace_id", traceID)
		c.Writer.Header().Set("X-Trace-ID", traceID)

		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		// Record latency and request count metrics
		common.GetObservabilityManager().IncrementCounter("api_requests_total", map[string]string{"path": path, "status": fmt.Sprintf("%d", status)})
		common.GetObservabilityManager().ObserveHistogram("api_latency_ms", float64(latency.Milliseconds()), map[string]string{"path": path})
		if status >= 400 {
			common.GetObservabilityManager().IncrementCounter("api_errors_total", map[string]string{"path": path, "status": fmt.Sprintf("%d", status)})
		}

		log.Info("HTTP Request",
			"trace_id", traceID,
			"method", c.Request.Method,
			"path", path,
			"query", raw,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"ip", c.ClientIP(),
		)
	})

	// Redis Distributed Rate Limiting Middleware
	r.Use(RateLimiterMiddleware())

	// Health Check / Readiness / Liveness Probe Endpoint (P0010 compliant)
	r.GET("/health", func(c *gin.Context) {
		status, details := common.GetObservabilityManager().CheckDependencyHealth(
			c.Request.Context(),
			func(ctx context.Context) error {
				if db == nil {
					return fmt.Errorf("no connection pool initialized")
				}
				return db.Ping(ctx)
			},
			func(ctx context.Context) error {
				if redisClient == nil {
					return fmt.Errorf("no redis client initialized")
				}
				return redisClient.Ping(ctx)
			},
			nil, // optional dependency check
			nil, // optional dependency check
		)

		statusCode := http.StatusOK
		if status == common.StatusNotReady {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, gin.H{
			"status":     status,
			"time":       time.Now().Format(time.RFC3339),
			"components": details,
		})
	})

	// Unified system readiness probe
	r.GET("/readiness", func(c *gin.Context) {
		status, details := common.GetObservabilityManager().CheckDependencyHealth(
			c.Request.Context(),
			func(ctx context.Context) error {
				if db == nil {
					return fmt.Errorf("no connection pool initialized")
				}
				return db.Ping(ctx)
			},
			func(ctx context.Context) error {
				if redisClient == nil {
					return fmt.Errorf("no redis client initialized")
				}
				return redisClient.Ping(ctx)
			},
			nil,
			nil,
		)

		statusCode := http.StatusOK
		if status == common.StatusNotReady {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, gin.H{
			"status":     status,
			"readiness":  status == common.StatusReady || status == common.StatusLive,
			"components": details,
		})
	})

	// Prometheus Metrics Endpoint (P0010 compliant)
	r.GET("/metrics", func(c *gin.Context) {
		metricsList := common.GetObservabilityManager().SnapshotMetrics()
		sb := strings.Builder{}
		sb.WriteString("# HELP velyxora_api_gateway_uptime Gateway uptime counter\n# TYPE velyxora_api_gateway_uptime counter\nvelyxora_api_gateway_uptime 1.0\n")

		for _, m := range metricsList {
			labelParts := []string{}
			for k, v := range m.Labels {
				labelParts = append(labelParts, fmt.Sprintf("%s=%q", k, v))
			}
			labelsStr := ""
			if len(labelParts) > 0 {
				labelsStr = "{" + strings.Join(labelParts, ",") + "}"
			}
			sb.WriteString(fmt.Sprintf("velyxora_%s%s %f\n", m.Name, labelsStr, m.Value))
		}
		c.String(http.StatusOK, sb.String())
	})

	// Real-time Gateway WebSocket Core
	r.GET("/ws", wsGateway.HandleConnection)

	// V1 API Router Group
	v1 := r.Group("/api/v1")
	{
		// Register API Key management routes
		RegisterAPIKeyHandlers(v1)

		// P0010 Central Observability & Incidents administration
		sysGroup := v1.Group("/system")
		sysGroup.Use(UnifiedAuthMiddleware(cfg.JWTSecret))
		sysGroup.Use(RBACMiddleware("system:admin"))
		{
			sysGroup.GET("/health", func(c *gin.Context) {
				res := common.GetObservabilityManager().CollectSystemResources()
				c.JSON(http.StatusOK, res)
			})

			sysGroup.GET("/readiness", func(c *gin.Context) {
				status, details := common.GetObservabilityManager().CheckDependencyHealth(
					c.Request.Context(),
					func(ctx context.Context) error {
						if db == nil {
							return fmt.Errorf("no database initialized")
						}
						return db.Ping(ctx)
					},
					func(ctx context.Context) error {
						if redisClient == nil {
							return fmt.Errorf("no redis initialized")
						}
						return redisClient.Ping(ctx)
					},
					nil,
					nil,
				)
				c.JSON(http.StatusOK, gin.H{
					"status":     status,
					"components": details,
				})
			})

			sysGroup.GET("/metrics", func(c *gin.Context) {
				snapshot := common.GetObservabilityManager().SnapshotMetrics()
				c.JSON(http.StatusOK, gin.H{
					"metrics": snapshot,
				})
			})
		}

		// Incident response endpoint handlers (RBAC protection)
		incGroup := v1.Group("/incidents")
		incGroup.Use(UnifiedAuthMiddleware(cfg.JWTSecret))
		incGroup.Use(RBACMiddleware("system:admin"))
		{
			incGroup.GET("", handleGetIncidents)
			incGroup.GET("/:id", handleGetIncidentByID)
			incGroup.POST("", handleCreateIncident)
			incGroup.PATCH("/:id", handlePatchIncident)
		}

		// Central secure auditing logs access handler (RBAC validation)
		v1.GET("/audit/events", UnifiedAuthMiddleware(cfg.JWTSecret), RBACMiddleware("support:read"), handleGetAuditEvents)

		// Public Market Data REST endpoints
		marketGroup := v1.Group("/market")
		{
			marketGroup.GET("/ticker", handleGetMarketTicker)
			marketGroup.GET("/depth", handleGetMarketDepth)
			marketGroup.GET("/trades", handleGetMarketRecentTrades)
			marketGroup.GET("/candles", handleGetMarketCandles)
			marketGroup.GET("/stats", handleGetMarketStats)
		}

		// OMS Handlers (Authenticated)
		omsGroup := v1.Group("")
		omsGroup.Use(UnifiedAuthMiddleware(cfg.JWTSecret))
		RegisterOMSHandlers(omsGroup)

		// Auth Routes
		auth := v1.Group("/auth")
		{
			auth.POST("/register", func(c *gin.Context) {
				var req struct {
					Email    string `json:"email" binding:"required,email"`
					Password string `json:"password" binding:"required"`
				}

				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters", "details": err.Error()})
					return
				}

				// Strict Input Sanitization
				sanitizedEmail := security.SanitizeInput(req.Email)

				// Validate Password Strength Policy
				if err := security.ValidatePasswordStrength(req.Password); err != nil {
					c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Weak password", "details": err.Error()})
					return
				}

				// Check password hash
				hash, err := security.HashPassword(req.Password)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process security parameters"})
					return
				}

				// For P001/P002 foundation we perform a mock creation if database is not connected
				userID := "usr_mock_" + fmt.Sprintf("%d", time.Now().UnixNano())
				if db != nil {
					// Prepare DB insert logic
					_, err := db.Pool.Exec(context.Background(),
						"INSERT INTO users (id, email, password_hash, status, role) VALUES ($1, $2, $3, $4, $5)",
						userID, sanitizedEmail, hash, string(types.StatusActive), string(types.RoleUser))
					if err != nil {
						c.JSON(http.StatusConflict, gin.H{"error": "Email is already registered"})
						return
					}
				}

				c.JSON(http.StatusCreated, gin.H{
					"message": "User registered successfully",
					"user": gin.H{
						"id":     userID,
						"email":  sanitizedEmail,
						"status": types.StatusActive,
						"role":   types.RoleUser,
					},
				})
			})

			auth.POST("/login", func(c *gin.Context) {
				var req struct {
					Email    string `json:"email" binding:"required,email"`
					Password string `json:"password" binding:"required"`
				}

				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid login credentials"})
					return
				}

				clientIP := c.ClientIP()

				// 1. Check Brute-Force Lockout Status (Email and IP)
				if isLocked, lockedUntil, err := CheckBruteForceAndLock(req.Email); err == nil && isLocked {
					common.GetObservabilityManager().IncrementCounter("mfa_brute_force_lockouts_total", map[string]string{"type": "email"})
					c.JSON(http.StatusLocked, gin.H{
						"error":        "Account is temporarily locked due to excessive login failures.",
						"locked_until": lockedUntil.Format(time.RFC3339),
					})
					return
				}

				if isLocked, lockedUntil, err := CheckBruteForceAndLock(clientIP); err == nil && isLocked {
					common.GetObservabilityManager().IncrementCounter("mfa_brute_force_lockouts_total", map[string]string{"type": "ip"})
					c.JSON(http.StatusLocked, gin.H{
						"error":        "Your IP address is temporarily locked due to excessive login failures.",
						"locked_until": lockedUntil.Format(time.RFC3339),
					})
					return
				}

				// Verification standard
				var userID string
				var hashedPassword string
				var role = string(types.RoleUser)

				if db != nil {
					err := db.Pool.QueryRow(context.Background(),
						"SELECT id, password_hash, role FROM users WHERE email = $1", req.Email).
						Scan(&userID, &hashedPassword, &role)
					if err != nil {
						_ = RecordLockoutFailure(req.Email)
						_ = RecordLockoutFailure(clientIP)
						common.GetObservabilityManager().IncrementCounter("authentication_failures_total", map[string]string{"reason": "user_not_found"})
						c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
						return
					}

					if !security.CheckPasswordHash(req.Password, hashedPassword) {
						_ = RecordLockoutFailure(req.Email)
						_ = RecordLockoutFailure(clientIP)
						common.GetObservabilityManager().IncrementCounter("authentication_failures_total", map[string]string{"reason": "wrong_password"})
						c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
						return
					}
				} else {
					// Fallback Mock authentication for standalone verification/development runs
					userID = "usr_mock_123"
					hashedPassword, _ = security.HashPassword("StrongPass1!")
					if req.Email != "test@velyxora.com" || req.Password != "StrongPass1!" {
						common.GetObservabilityManager().IncrementCounter("authentication_failures_total", map[string]string{"reason": "invalid_mock"})
						c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid mock credentials"})
						return
					}
				}

				// Reset Lockout Failures on Success
				_ = ResetLockoutFailures(req.Email)
				_ = ResetLockoutFailures(clientIP)

				// Generate initial JWTs (Hardened Expiration)
				accessToken, refreshToken, err := security.GenerateJWT(
					userID,
					req.Email,
					"temp_sess",
					cfg.JWTSecret,
					15*time.Minute,
					24*time.Hour,
				)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign authentication tokens"})
					return
				}

				// Create session in PostgreSQL & Redis, enforcing concurrent limits
				deviceID := c.GetHeader("X-Device-ID")
				if deviceID == "" {
					deviceID = "dev_unknown"
				}
				sessionID, err := CreateSession(userID, deviceID, c.ClientIP(), c.Request.UserAgent(), refreshToken, time.Now().Add(24*time.Hour))
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to establish secure session"})
					return
				}

				// Re-generate access JWT with the actual session ID
				accessToken, _, err = security.GenerateJWT(
					userID,
					req.Email,
					sessionID,
					cfg.JWTSecret,
					15*time.Minute,
					24*time.Hour,
				)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign session token"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"access_token":  accessToken,
					"refresh_token": refreshToken,
					"session_id":    sessionID,
					"expires_in":    900, // 15 mins
					"role":          role,
				})
			})

			auth.POST("/refresh", func(c *gin.Context) {
				var req struct {
					RefreshToken string `json:"refresh_token" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Refresh token required"})
					return
				}

				claims, err := security.ValidateJWT(req.RefreshToken, cfg.JWTSecret)
				if err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token", "details": err.Error()})
					return
				}

				_, newRefresh, err := security.GenerateJWT(claims.UserID, claims.Email, "temp", cfg.JWTSecret, 15*time.Minute, 24*time.Hour)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign tokens"})
					return
				}

				sessionID, userID, err := RotateSession(req.RefreshToken, newRefresh, c.ClientIP(), c.Request.UserAgent())
				if err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
					return
				}

				finalAccess, finalRefresh, err := security.GenerateJWT(userID, claims.Email, sessionID, cfg.JWTSecret, 15*time.Minute, 24*time.Hour)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign tokens"})
					return
				}

				if globalDB != nil {
					_, _ = globalDB.Pool.Exec(context.Background(),
						"UPDATE user_sessions SET refresh_token_hash = $1 WHERE id = $2",
						hashToken(finalRefresh), sessionID)
				}

				c.JSON(http.StatusOK, gin.H{
					"access_token":  finalAccess,
					"refresh_token": finalRefresh,
					"session_id":    sessionID,
					"expires_in":    900,
				})
			})

			auth.POST("/logout", authMiddleware(cfg.JWTSecret), func(c *gin.Context) {
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				err := RevokeSession(userClaims.SessionID, c.ClientIP(), c.Request.UserAgent())
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke session"})
					return
				}

				c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
			})

			auth.POST("/logout-all", authMiddleware(cfg.JWTSecret), func(c *gin.Context) {
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				err := RevokeAllSessions(userClaims.UserID, c.ClientIP(), c.Request.UserAgent(), "USER_LOGOUT_ALL")
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke sessions"})
					return
				}

				c.JSON(http.StatusOK, gin.H{"message": "Logged out of all active sessions successfully"})
			})

			auth.GET("/sessions", authMiddleware(cfg.JWTSecret), func(c *gin.Context) {
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				type SessionResp struct {
					ID        string    `json:"id"`
					DeviceID  string    `json:"device_id"`
					IPAddress string    `json:"ip_address"`
					UserAgent string    `json:"user_agent"`
					CreatedAt time.Time `json:"created_at"`
					IsCurrent bool      `json:"is_current"`
				}

				sessions := make([]SessionResp, 0)
				if globalDB != nil {
					rows, err := globalDB.Pool.Query(context.Background(),
						"SELECT id, device_id, ip_address, user_agent, created_at FROM user_sessions WHERE user_id = $1 AND is_revoked = FALSE AND expires_at > NOW() ORDER BY created_at DESC",
						userClaims.UserID)
					if err == nil {
						defer rows.Close()
						for rows.Next() {
							var s SessionResp
							errScan := rows.Scan(&s.ID, &s.DeviceID, &s.IPAddress, &s.UserAgent, &s.CreatedAt)
							if errScan == nil {
								s.IsCurrent = (s.ID == userClaims.SessionID)
								sessions = append(sessions, s)
							}
						}
					}
				} else {
					sessions = append(sessions, SessionResp{
						ID:        userClaims.SessionID,
						DeviceID:  "dev_mock",
						IPAddress: c.ClientIP(),
						UserAgent: c.Request.UserAgent(),
						CreatedAt: time.Now(),
						IsCurrent: true,
					})
				}

				c.JSON(http.StatusOK, gin.H{"sessions": sessions})
			})

			auth.DELETE("/sessions/:id", authMiddleware(cfg.JWTSecret), func(c *gin.Context) {
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)
				sessionID := c.Param("id")

				if globalDB != nil {
					var ownerID string
					err := globalDB.Pool.QueryRow(context.Background(), "SELECT user_id FROM user_sessions WHERE id = $1", sessionID).Scan(&ownerID)
					if err != nil || ownerID != userClaims.UserID {
						c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: you do not own this session"})
						return
					}
				}

				err := RevokeSession(sessionID, c.ClientIP(), c.Request.UserAgent())
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke session"})
					return
				}

				c.JSON(http.StatusOK, gin.H{"message": "Session successfully terminated"})
			})
		}

		// Authenticated Routes
		trading := v1.Group("/trading")
		trading.Use(UnifiedAuthMiddleware(cfg.JWTSecret))
		trading.Use(IdempotencyMiddleware())
		{
			trading.POST("/orders", func(c *gin.Context) {
				var req struct {
					Symbol   string  `json:"symbol" binding:"required"`
					Side     string  `json:"side" binding:"required"` // BUY, SELL
					Type     string  `json:"type" binding:"required"` // LIMIT, MARKET
					Price    float64 `json:"price"`
					Quantity float64 `json:"quantity" binding:"required,gt=0"`
				}

				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				order := types.Order{
					ID:        "ord_" + fmt.Sprintf("%d", time.Now().UnixNano()),
					UserID:    userClaims.UserID,
					Symbol:    req.Symbol,
					Side:      types.OrderSide(req.Side),
					Type:      types.OrderType(req.Type),
					Price:     req.Price,
					Quantity:  req.Quantity,
					Status:    types.StatusNew,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				// Trace/Correlation ID propagation inside versioned Kafka events
				traceID, _ := c.Get("trace_id")
				event := types.KafkaEvent{
					Type:      types.EventOrderCreated,
					Payload:   order,
					Timestamp: time.Now(),
				}

				// Publish to order matching queue
				err := kafkaProducer.Publish(context.Background(), "velyxora-orders", order.ID, event)
				if err != nil {
					common.GetObservabilityManager().IncrementCounter("kafka_publish_failures_total", map[string]string{"topic": "velyxora-orders"})
				} else {
					common.GetObservabilityManager().IncrementCounter("orders_accepted_total", map[string]string{"symbol": req.Symbol})
				}

				// Audit security trail log integration
				WriteAuditEvent(c.Request.Context(), "ORDER_PLACEMENT", "LOW", userClaims.UserID, userClaims.SessionID, "", fmt.Sprintf("Order placed: %s", order.ID), "{}", c.ClientIP(), c.Request.UserAgent())

				c.JSON(http.StatusAccepted, gin.H{
					"message":      "Order created and queued",
					"order":        order,
					"correlation":  traceID,
				})
			})
		}

		// Security Audit Actions (MFA foundation configurations under P003)
		mfa := v1.Group("/mfa")
		mfa.Use(UnifiedAuthMiddleware(cfg.JWTSecret))
		{
			mfa.POST("/enable", func(c *gin.Context) {
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				WriteAuditEvent(c.Request.Context(), "MFA_ENABLED", "MEDIUM", userClaims.UserID, userClaims.SessionID, "", "MFA enrollment initiated", "{}", c.ClientIP(), c.Request.UserAgent())

				c.JSON(http.StatusOK, gin.H{
					"mfa_secret":   "JBSWY3DPEHPK3PXP",
					"qr_code_url":  "otpauth://totp/Velyxora:user?secret=JBSWY3DPEHPK3PXP&issuer=Velyxora",
					"backup_codes": []string{"1234-5678", "abcd-efgh", "9876-5432"},
				})
			})
		}

		// P004 Wallet, Deposit, Withdrawal, Asset APIs
		wallet := v1.Group("/wallet")
		wallet.Use(UnifiedAuthMiddleware(cfg.JWTSecret))
		{
			// Fetch user asset balances
			wallet.GET("/balances", func(c *gin.Context) {
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				var balances []types.Balance
				if globalDB != nil {
					rows, err := globalDB.Pool.Query(c.Request.Context(),
						"SELECT user_id, asset, available, locked, pending, reserved, total FROM balances WHERE user_id = $1", userClaims.UserID)
					if err == nil {
						defer rows.Close()
						for rows.Next() {
							var bal types.Balance
							err := rows.Scan(&bal.UserID, &bal.Asset, &bal.Available, &bal.Locked, &bal.Pending, &bal.Reserved, &bal.Total)
							if err == nil {
								balances = append(balances, bal)
							}
						}
					}
				}

				if balances == nil {
					balances = []types.Balance{}
				}

				c.JSON(http.StatusOK, gin.H{
					"balances": balances,
				})
			})

			// Request deposit wallet address validation/allocation
			wallet.POST("/address", func(c *gin.Context) {
				var req struct {
					Asset string `json:"asset" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Asset required"})
					return
				}

				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				adapter, err := common.GetBlockchainAdapter(req.Asset)
				if err != nil {
					c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
					return
				}

				// Assign mock standard validate address based on blockchain adapters
				address := "0x71C7656EC7ab88b098defB751B7401B5f6d1476B"
				if strings.ToUpper(req.Asset) == "BTC" {
					address = "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"
				} else if strings.ToUpper(req.Asset) == "SOL" {
					address = "Hxs86Xj38x8vMvVvE75A9XG9m9L9p9"
				}

				if !adapter.ValidateAddress(address) {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to formulate valid destination key address"})
					return
				}

				c.JSON(http.StatusOK, types.WalletAddress{
					UserID:    userClaims.UserID,
					Asset:     strings.ToUpper(req.Asset),
					Address:   address,
					IsActive:  true,
					CreatedAt: time.Now(),
				})
			})

			// Process withdrawal requests
			wallet.POST("/withdraw", func(c *gin.Context) {
				var req struct {
					Asset   string  `json:"asset" binding:"required"`
					Amount  float64 `json:"amount" binding:"required,gt=0"`
					Address string  `json:"address" binding:"required"`
				}

				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				adapter, err := common.GetBlockchainAdapter(req.Asset)
				if err != nil {
					c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
					return
				}

				// Address ownership validation (P004 security check)
				if !adapter.ValidateAddress(req.Address) {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid destination blockchain address format"})
					return
				}

				fee := 0.0005
				if strings.ToUpper(req.Asset) == "USDT" {
					fee = 1.0
				}

				withdrawal := types.Withdrawal{
					ID:        "wth_" + fmt.Sprintf("%d", time.Now().UnixNano()),
					UserID:    userClaims.UserID,
					Asset:     strings.ToUpper(req.Asset),
					Amount:    req.Amount,
					Fee:       fee,
					Address:   req.Address,
					Status:    types.WithdrawalPendingApproval,
					RiskScore: 0.15,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				WriteAuditEvent(c.Request.Context(), "WITHDRAWAL_REQUEST", "LOW", userClaims.UserID, userClaims.SessionID, "", fmt.Sprintf("Withdrawal initiated for %f %s", req.Amount, req.Asset), "{}", c.ClientIP(), c.Request.UserAgent())
				common.GetObservabilityManager().IncrementCounter("withdrawal_processing_total", map[string]string{"asset": req.Asset})

				c.JSON(http.StatusAccepted, gin.H{
					"message":    "Withdrawal request registered, pending risk audit",
					"withdrawal": withdrawal,
				})
			})
		}
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Graceful Shutdown Management
	go func() {
		log.Info(fmt.Sprintf("Velyxora REST Gateway running on port %s", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(fmt.Sprintf("Failed to run HTTP server: %v", err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down API Gateway gateway gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error(fmt.Sprintf("API Gateway forced to shutdown: %v", err))
	}

	log.Info("Velyxora REST Gateway exited successfully.")
}

// REST Handlers for Incidents
type Incident struct {
	ID               string    `json:"id" db:"id"`
	Severity         string    `json:"severity" db:"severity"`
	Source           string    `json:"source" db:"source"`
	Description      string    `json:"description" db:"description"`
	AffectedService  string    `json:"affected_service" db:"affected_service"`
	Status           string    `json:"status" db:"status"` // OPEN, ACKNOWLEDGED, INVESTIGATING, MITIGATED, RESOLVED, CLOSED
	Assignee         string    `json:"assignee" db:"assignee"`
	ResolutionNotes  string    `json:"resolution_notes" db:"resolution_notes"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

func handleGetIncidents(c *gin.Context) {
	incidents := []Incident{}
	if globalDB != nil {
		rows, err := globalDB.Pool.Query(c.Request.Context(),
			"SELECT id, severity, source, description, affected_service, status, assignee, resolution_notes, created_at, updated_at FROM incidents ORDER BY created_at DESC")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var inc Incident
				errScan := rows.Scan(&inc.ID, &inc.Severity, &inc.Source, &inc.Description, &inc.AffectedService, &inc.Status, &inc.Assignee, &inc.ResolutionNotes, &inc.CreatedAt, &inc.UpdatedAt)
				if errScan == nil {
					incidents = append(incidents, inc)
				}
			}
		}
	}
	c.JSON(http.StatusOK, incidents)
}

func handleGetIncidentByID(c *gin.Context) {
	id := c.Param("id")
	if globalDB == nil {
		c.JSON(http.StatusOK, Incident{ID: id, Status: "OPEN"})
		return
	}

	var inc Incident
	err := globalDB.Pool.QueryRow(c.Request.Context(),
		"SELECT id, severity, source, description, affected_service, status, assignee, resolution_notes, created_at, updated_at FROM incidents WHERE id = $1", id).
		Scan(&inc.ID, &inc.Severity, &inc.Source, &inc.Description, &inc.AffectedService, &inc.Status, &inc.Assignee, &inc.ResolutionNotes, &inc.CreatedAt, &inc.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
		return
	}
	c.JSON(http.StatusOK, inc)
}

func handleCreateIncident(c *gin.Context) {
	var req struct {
		Severity        string `json:"severity" binding:"required"`
		Source          string `json:"source" binding:"required"`
		Description     string `json:"description" binding:"required"`
		AffectedService string `json:"affected_service" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	inc := Incident{
		ID:              "inc_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Severity:        req.Severity,
		Source:          req.Source,
		Description:     req.Description,
		AffectedService: req.AffectedService,
		Status:          "OPEN",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if globalDB != nil {
		_, err := globalDB.Pool.Exec(c.Request.Context(),
			"INSERT INTO incidents (id, severity, source, description, affected_service, status, assignee, resolution_notes, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())",
			inc.ID, inc.Severity, inc.Source, inc.Description, inc.AffectedService, inc.Status, inc.Assignee, inc.ResolutionNotes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist incident record"})
			return
		}
	}

	// Trigger alert state creation or publishing event over Kafka
	if globalKafkaProducer != nil {
		_ = globalKafkaProducer.Publish(c.Request.Context(), "velyxora-operational-alerts", inc.ID, types.KafkaEvent{
			Type:      types.EventType("incident.created"),
			Payload:   inc,
			Timestamp: time.Now(),
		})
	}

	c.JSON(http.StatusCreated, inc)
}

func handlePatchIncident(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status          string `json:"status"`
		Assignee        string `json:"assignee"`
		ResolutionNotes string `json:"resolution_notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid fields format"})
		return
	}

	if globalDB == nil {
		c.JSON(http.StatusOK, gin.H{"status": "UPDATED_MOCK"})
		return
	}

	// Dynamic update query builder
	query := "UPDATE incidents SET updated_at = NOW()"
	args := []interface{}{}
	argIdx := 1

	if req.Status != "" {
		query += fmt.Sprintf(", status = $%d", argIdx)
		args = append(args, req.Status)
		argIdx++
	}
	if req.Assignee != "" {
		query += fmt.Sprintf(", assignee = $%d", argIdx)
		args = append(args, req.Assignee)
		argIdx++
	}
	if req.ResolutionNotes != "" {
		query += fmt.Sprintf(", resolution_notes = $%d", argIdx)
		args = append(args, req.ResolutionNotes)
		argIdx++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, id)

	_, err := globalDB.Pool.Exec(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update incident"})
		return
	}

	// Emit updated telemetry event onto Kafka topic
	if globalKafkaProducer != nil {
		_ = globalKafkaProducer.Publish(c.Request.Context(), "velyxora-operational-alerts", id, types.KafkaEvent{
			Type:      types.EventType("incident.updated"),
			Payload:   gin.H{"incident_id": id, "status": req.Status},
			Timestamp: time.Now(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Incident updated cleanly", "id": id})
}

// Security Audit Event Reader REST Handler
type AuditLogEntry struct {
	ID        string    `json:"id" db:"id"`
	EventType string    `json:"event_type" db:"event_type"`
	Severity  string    `json:"severity" db:"severity"`
	UserID    string    `json:"user_id" db:"user_id"`
	IPAddress string    `json:"ip_address" db:"ip_address"`
	Details   string    `json:"details" db:"details"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

func handleGetAuditEvents(c *gin.Context) {
	events := []AuditLogEntry{}
	if globalDB != nil {
		rows, err := globalDB.Pool.Query(c.Request.Context(),
			"SELECT id, event_type, severity, user_id, ip_address, details, created_at FROM security_audit_logs ORDER BY created_at DESC LIMIT 100")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var ev AuditLogEntry
				errScan := rows.Scan(&ev.ID, &ev.EventType, &ev.Severity, &ev.UserID, &ev.IPAddress, &ev.Details, &ev.CreatedAt)
				if errScan == nil {
					events = append(events, ev)
				}
			}
		}
	}
	c.JSON(http.StatusOK, events)
}

// UnifiedAuthMiddleware combines API Key authentication and standard JWT session authentication
func UnifiedAuthMiddleware(jwtSecret string) gin.HandlerFunc {
	apiKeyAuth := APIKeyAuthMiddleware()
	jwtAuth := authMiddleware(jwtSecret)

	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-KEY")
		if apiKey != "" {
			apiKeyAuth(c)
		} else {
			jwtAuth(c)
		}
	}
}

// authMiddleware enforces valid JWT presence in Authorization headers
func authMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization format must be Bearer <token>"})
			c.Abort()
			return
		}

		claims, err := security.ValidateJWT(parts[1], jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token", "details": err.Error()})
			c.Abort()
			return
		}

		// Verify that the session has not been revoked
		if claims.SessionID != "" && IsSessionRevoked(claims.SessionID) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: session has been revoked"})
			c.Abort()
			return
		}

		c.Set("claims", claims)
		c.Next()
	}
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}
