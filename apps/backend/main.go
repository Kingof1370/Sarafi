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
var globalComplianceEngine *security.ComplianceEngine
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
	} else {
		globalRedis = redisClient
		log.Info("Redis client connected successfully.")
	}

	// 5. Setup Kafka Producer
	kafkaProducer := common.NewKafkaProducer(cfg.KafkaBrokers)
	globalKafkaProducer = kafkaProducer
	defer kafkaProducer.Close()
	log.Info("Kafka Producer registered.")

	// Initialize Compliance and Customer Verification Engine (P0009)
	globalComplianceEngine = security.NewComplianceEngine(globalDB, globalKafkaProducer)
	log.Info("Global Compliance & Risk Control Engine initialized.")

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
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
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

	// Health Check / Readiness / Liveness Probe Endpoint
	r.GET("/health", func(c *gin.Context) {
		dbStatus := "UP"
		if db == nil || db.Ping(c.Request.Context()) != nil {
			dbStatus = "DOWN"
		}

		redisStatus := "UP"
		if redisClient == nil || redisClient.Ping(c.Request.Context()) != nil {
			redisStatus = "DOWN"
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "UP",
			"time":   time.Now().Format(time.RFC3339),
			"components": gin.H{
				"postgres": dbStatus,
				"redis":    redisStatus,
			},
		})
	})

	// Prometheus Metrics Endpoint Placeholder
	r.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, "# HELP velyxora_api_gateway_uptime Gateway uptime counter\n# TYPE velyxora_api_gateway_uptime counter\nvelyxora_api_gateway_uptime 1.0\n")
	})

	// Real-time Gateway WebSocket Core
	r.GET("/ws", wsGateway.HandleConnection)

	// V1 API Router Group
	v1 := r.Group("/api/v1")
	{
		// Register API Key management routes
		RegisterAPIKeyHandlers(v1)

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
					c.JSON(http.StatusLocked, gin.H{
						"error":        "Account is temporarily locked due to excessive login failures.",
						"locked_until": lockedUntil.Format(time.RFC3339),
					})
					return
				}

				if isLocked, lockedUntil, err := CheckBruteForceAndLock(clientIP); err == nil && isLocked {
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
						c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
						return
					}

					if !security.CheckPasswordHash(req.Password, hashedPassword) {
						_ = RecordLockoutFailure(req.Email)
						_ = RecordLockoutFailure(clientIP)
						c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
						return
					}
				} else {
					// Fallback Mock authentication for standalone verification/development runs
					userID = "usr_mock_123"
					hashedPassword, _ = security.HashPassword("StrongPass1!")
					if req.Email != "test@velyxora.com" || req.Password != "StrongPass1!" {
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

				// Synchronous compliance verification gate for order placement (P0009)
				if err := VerifyTradingCompliance(c.Request.Context(), userClaims.UserID, req.Symbol, req.Side, req.Price, req.Quantity); err != nil {
					c.JSON(http.StatusForbidden, gin.H{
						"error": fmt.Sprintf("Order Compliance Blocked: %v", err),
						"compliance_code": "COMPLIANCE_BLOCKED",
					})
					return
				}

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

				// Emit Event to Kafka
				event := types.KafkaEvent{
					Type:      types.EventOrderCreated,
					Payload:   order,
					Timestamp: time.Now(),
				}

				// Publish to order matching queue
				_ = kafkaProducer.Publish(context.Background(), "velyxora-orders", order.ID, event)

				c.JSON(http.StatusAccepted, gin.H{
					"message": "Order created and queued",
					"order":   order,
				})
			})
		}

		// Security Audit Actions (MFA foundation configurations under P003)
		mfa := v1.Group("/mfa")
		mfa.Use(UnifiedAuthMiddleware(cfg.JWTSecret))
		{
			mfa.POST("/enable", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"mfa_secret": "JBSWY3DPEHPK3PXP",
					"qr_code_url": "otpauth://totp/Velyxora:user?secret=JBSWY3DPEHPK3PXP&issuer=Velyxora",
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

				// 1. Synchronous Compliance Gate checks (KYC status, restrictions, limits, sanctions checks)
				if err := globalComplianceEngine.CheckWithdrawalAllowed(c.Request.Context(), userClaims.UserID, req.Amount, strings.ToUpper(req.Asset), req.Address); err != nil {
					c.JSON(http.StatusForbidden, gin.H{
						"error": fmt.Sprintf("Withdrawal Compliance Gate Blocked: %v", err),
						"compliance_code": "COMPLIANCE_BLOCKED",
					})
					return
				}

				// 2. Run AML rules monitoring
				alerts, amlErr := globalComplianceEngine.EvaluateAML(c.Request.Context(), userClaims.UserID, req.Amount, strings.ToUpper(req.Asset), "WITHDRAWAL", "wth_pending")
				var riskScore float64 = 0.15
				if amlErr == nil && len(alerts) > 0 {
					riskScore = 0.85
				}

				// 3. Check Travel Rule if required (for values >= 1000 USDT/USD/units)
				if req.Amount >= 1000.0 {
					origName := c.GetHeader("X-Travel-Originator-Name")
					benefName := c.GetHeader("X-Travel-Beneficiary-Name")

					origAddress := "123 Originator St"
					origAccount := userClaims.UserID
					benefAddress := req.Address
					benefAccount := "ACC_BENEF"

					trErr := globalComplianceEngine.RegisterTravelRule(c.Request.Context(), "wth_pending", origName, origAddress, origAccount, benefName, benefAddress, benefAccount)
					if trErr != nil {
						c.JSON(http.StatusUnprocessableEntity, gin.H{
							"error": fmt.Sprintf("Travel Rule compliance validation failed: %v. Please provide originator/beneficiary information via headers.", trErr),
							"compliance_code": "TRAVEL_RULE_MISSING",
						})
						return
					}
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
					RiskScore: riskScore,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				if globalDB != nil {
					_, dbErr := globalDB.Pool.Exec(c.Request.Context(),
						"INSERT INTO withdrawals (id, user_id, asset, amount, fee, address, status, risk_score, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
						withdrawal.ID, withdrawal.UserID, withdrawal.Asset, withdrawal.Amount, withdrawal.Fee, withdrawal.Address, string(withdrawal.Status), withdrawal.RiskScore, withdrawal.CreatedAt, withdrawal.UpdatedAt)
					if dbErr != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist withdrawal record", "details": dbErr.Error()})
						return
					}
				}

				// Publish to Kafka
				event := types.KafkaEvent{
					Type:      types.EventType("WITHDRAWAL_REQUESTED"),
					Payload:   withdrawal,
					Timestamp: time.Now(),
				}
				_ = globalKafkaProducer.Publish(c.Request.Context(), "velyxora-withdrawals", withdrawal.ID, event)

				c.JSON(http.StatusAccepted, gin.H{
					"message":    "Withdrawal request registered, pending risk audit",
					"withdrawal": withdrawal,
				})
			})

			// Deposit Compliance Integration (P0008 + P0009)
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

				// Determine blockchain confirmation vs compliance concept. Set block confirmation to 12.
				confirmations := 12

				// 1. Sanctions screening on sending / receiving address (Failure closed!)
				sStatus, details, scrErr := globalComplianceEngine.ScreenSanctions(c.Request.Context(), "ADDRESS_DEPOSIT", req.Address)

				status := "credited"
				evidence := "Passes all compliance verifications."

				if scrErr != nil || sStatus == security.SanctionsMatch || sStatus == security.SanctionsPotentialMatch || sStatus == security.SanctionsUnavailable {
					status = "held"
					evidence = fmt.Sprintf("Sanctions trigger matched on deposit address %s: status %s (%s)", req.Address, sStatus, details)
					_, _ = globalComplianceEngine.CreateAlert(c.Request.Context(), userClaims.UserID, req.TxHash, "sanctions_deposit_match", 0.95, evidence, security.SeverityCritical)
					_ = globalComplianceEngine.RestrictAccount(c.Request.Context(), userClaims.UserID, security.RestrictionDepositRestricted, "Deposit address on sanctions watch list", "SYSTEM")
				} else {
					// 2. Check limits compliance
					limErr := globalComplianceEngine.CheckLimits(c.Request.Context(), userClaims.UserID, req.Amount, "DEPOSIT")
					if limErr != nil {
						status = "under_review"
						evidence = fmt.Sprintf("Deposit exceeds limits policy: %v", limErr)
						_, _ = globalComplianceEngine.CreateAlert(c.Request.Context(), userClaims.UserID, req.TxHash, "aml_deposit_limit_violation", 0.60, evidence, security.SeverityHigh)
					}
				}

				depositID := "dep_" + fmt.Sprintf("%d", time.Now().UnixNano())

				// Persist deposit record to DB if active
				if globalDB != nil {
					_, dbErr := globalDB.Pool.Exec(c.Request.Context(),
						"INSERT INTO deposits (id, user_id, asset, amount, fee, address, tx_hash, confirmations, status, created_at, updated_at) VALUES ($1, $2, $3, $4, 0.0, $5, $6, $7, $8, NOW(), NOW())",
						depositID, userClaims.UserID, strings.ToUpper(req.Asset), req.Amount, req.Address, req.TxHash, confirmations, strings.ToUpper(status))
					if dbErr != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store deposit", "details": dbErr.Error()})
						return
					}

					// Credit user balance only if compliance status is clear and credited
					if status == "credited" {
						// Lock user balance and update
						tx, txErr := globalDB.Pool.Begin(c.Request.Context())
						if txErr == nil {
							defer tx.Rollback(c.Request.Context())
							var currentAvail, currentTotal float64
							errQuery := tx.QueryRow(c.Request.Context(), "SELECT available, total FROM balances WHERE user_id = $1 AND asset = $2 FOR UPDATE", userClaims.UserID, strings.ToUpper(req.Asset)).Scan(&currentAvail, &currentTotal)
							if errQuery != nil {
								// Insert default balance first
								_, _ = tx.Exec(c.Request.Context(), "INSERT INTO balances (user_id, asset, available, total, locked, pending, reserved, updated_at) VALUES ($1, $2, $3, $3, 0, 0, 0, NOW())", userClaims.UserID, strings.ToUpper(req.Asset), req.Amount)
							} else {
								_, _ = tx.Exec(c.Request.Context(), "UPDATE balances SET available = $1, total = $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4", currentAvail+req.Amount, currentTotal+req.Amount, userClaims.UserID, strings.ToUpper(req.Asset))
							}

							// Insert Ledger entry
							_, _ = tx.Exec(c.Request.Context(), "INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, 'CREDIT', $5, 'Deposit allocation credited', NOW())", "ent_dep_"+depositID, depositID, userClaims.UserID, strings.ToUpper(req.Asset), req.Amount)
							_ = tx.Commit(c.Request.Context())
						}
					}
				}

				// Publish deposit Kafka compliance event
				event := types.KafkaEvent{
					Type:      types.EventType("DEPOSIT_DETECTED"),
					Payload:   map[string]interface{}{"deposit_id": depositID, "user_id": userClaims.UserID, "status": status, "confirmations": confirmations, "evidence": evidence},
					Timestamp: time.Now(),
				}
				_ = globalKafkaProducer.Publish(c.Request.Context(), "velyxora-deposits", depositID, event)

				c.JSON(http.StatusOK, gin.H{
					"message": "Deposit processed",
					"deposit_id": depositID,
					"confirmations": confirmations,
					"compliance_status": status,
					"details": evidence,
				})
			})

			wallet.GET("/deposits/history", func(c *gin.Context) {
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				var deposits []types.Deposit
				if globalDB != nil {
					rows, err := globalDB.Pool.Query(c.Request.Context(),
						"SELECT id, user_id, asset, amount, fee, address, tx_hash, confirmations, status, created_at, updated_at FROM deposits WHERE user_id = $1", userClaims.UserID)
					if err == nil {
						defer rows.Close()
						for rows.Next() {
							var dep types.Deposit
							var statusStr string
							err := rows.Scan(&dep.ID, &dep.UserID, &dep.Asset, &dep.Amount, &dep.Fee, &dep.Address, &dep.TxHash, &dep.Confirmations, &statusStr, &dep.CreatedAt, &dep.UpdatedAt)
							if err == nil {
								dep.Status = types.DepositStatus(statusStr)
								deposits = append(deposits, dep)
							}
						}
					}
				}

				if deposits == nil {
					deposits = []types.Deposit{}
				}

				c.JSON(http.StatusOK, gin.H{"deposits": deposits})
			})
		}

		// Enterprise Compliance and Cases API Group (P0009)
		compliance := v1.Group("/compliance")
		compliance.Use(UnifiedAuthMiddleware(cfg.JWTSecret))
		{
			// User Facing KYC and Restrictions
			compliance.GET("/kyc/status", func(c *gin.Context) {
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				profile, err := globalComplianceEngine.GetKYC(c.Request.Context(), userClaims.UserID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
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
				c.JSON(http.StatusOK, gin.H{
					"message": "KYC Profile Submitted Successfully",
					"profile": profile,
				})
			})

			compliance.GET("/restrictions", func(c *gin.Context) {
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				rest, err := globalComplianceEngine.GetRestriction(c.Request.Context(), userClaims.UserID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"restriction_type": rest})
			})

			// Administrative Compliance Operations (RBAC risk:write protected)
			compliance.GET("/alerts", RBACMiddleware("risk:write"), func(c *gin.Context) {
				var alerts []security.ComplianceAlert
				if globalDB != nil {
					rows, err := globalDB.Pool.Query(c.Request.Context(),
						"SELECT id, user_id, transaction_id, rule_triggered, risk_score, evidence, severity, status, reviewer, resolution_reason, created_at, updated_at FROM compliance_alerts ORDER BY created_at DESC")
					if err == nil {
						defer rows.Close()
						for rows.Next() {
							var a security.ComplianceAlert
							var sev, stat string
							err := rows.Scan(&a.ID, &a.UserID, &a.TransactionID, &a.RuleTriggered, &a.RiskScore, &a.Evidence, &sev, &stat, &a.Reviewer, &a.ResolutionReason, &a.CreatedAt, &a.UpdatedAt)
							if err == nil {
								a.Severity = security.AlertSeverity(sev)
								a.Status = security.AlertStatus(stat)
								alerts = append(alerts, a)
							}
						}
					}
				}

				if alerts == nil {
					alerts = []security.ComplianceAlert{}
				}
				c.JSON(http.StatusOK, gin.H{"alerts": alerts})
			})

			compliance.GET("/alerts/:id", RBACMiddleware("risk:write"), func(c *gin.Context) {
				id := c.Param("id")
				if globalDB != nil {
					var a security.ComplianceAlert
					var sev, stat string
					err := globalDB.Pool.QueryRow(c.Request.Context(),
						"SELECT id, user_id, transaction_id, rule_triggered, risk_score, evidence, severity, status, reviewer, resolution_reason, created_at, updated_at FROM compliance_alerts WHERE id = $1", id).Scan(&a.ID, &a.UserID, &a.TransactionID, &a.RuleTriggered, &a.RiskScore, &a.Evidence, &sev, &stat, &a.Reviewer, &a.ResolutionReason, &a.CreatedAt, &a.UpdatedAt)
					if err == nil {
						a.Severity = security.AlertSeverity(sev)
						a.Status = security.AlertStatus(stat)
						c.JSON(http.StatusOK, a)
						return
					}
				}
				c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
			})

			compliance.GET("/cases", RBACMiddleware("risk:write"), func(c *gin.Context) {
				var cases []security.ComplianceCase
				if globalDB != nil {
					rows, err := globalDB.Pool.Query(c.Request.Context(),
						"SELECT id, user_id, status, investigator_id, resolution, created_at, updated_at FROM compliance_cases ORDER BY created_at DESC")
					if err == nil {
						defer rows.Close()
						for rows.Next() {
							var d security.ComplianceCase
							var stat string
							err := rows.Scan(&d.ID, &d.UserID, &stat, &d.InvestigatorID, &d.Resolution, &d.CreatedAt, &d.UpdatedAt)
							if err == nil {
								d.Status = security.CaseStatus(stat)
								cases = append(cases, d)
							}
						}
					}
				}

				if cases == nil {
					cases = []security.ComplianceCase{}
				}
				c.JSON(http.StatusOK, gin.H{"cases": cases})
			})

			compliance.GET("/cases/:id", RBACMiddleware("risk:write"), func(c *gin.Context) {
				id := c.Param("id")
				var d security.ComplianceCase
				var stat string
				if globalDB != nil {
					err := globalDB.Pool.QueryRow(c.Request.Context(),
						"SELECT id, user_id, status, investigator_id, resolution, created_at, updated_at FROM compliance_cases WHERE id = $1", id).Scan(&d.ID, &d.UserID, &stat, &d.InvestigatorID, &d.Resolution, &d.CreatedAt, &d.UpdatedAt)
					if err != nil {
						c.JSON(http.StatusNotFound, gin.H{"error": "Case not found"})
						return
					}
					d.Status = security.CaseStatus(stat)

					// Load case notes
					var notes []security.ComplianceCaseNote
					nRows, err := globalDB.Pool.Query(c.Request.Context(), "SELECT id, case_id, author_id, note, created_at FROM compliance_case_notes WHERE case_id = $1 ORDER BY created_at ASC", id)
					if err == nil {
						defer nRows.Close()
						for nRows.Next() {
							var n security.ComplianceCaseNote
							if errScan := nRows.Scan(&n.ID, &n.CaseID, &n.AuthorID, &n.Note, &n.CreatedAt); errScan == nil {
								notes = append(notes, n)
							}
						}
					}
					c.JSON(http.StatusOK, gin.H{"case": d, "notes": notes})
					return
				}
				c.JSON(http.StatusNotFound, gin.H{"error": "Case not found"})
			})

			compliance.POST("/cases", RBACMiddleware("risk:write"), func(c *gin.Context) {
				var req struct {
					UserID   string   `json:"user_id" binding:"required"`
					AlertIDs []string `json:"alert_ids" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				cs, err := globalComplianceEngine.CreateCase(c.Request.Context(), req.UserID, req.AlertIDs)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusCreated, cs)
			})

			compliance.PATCH("/cases/:id", RBACMiddleware("risk:write"), func(c *gin.Context) {
				id := c.Param("id")
				var req struct {
					Status     string `json:"status" binding:"required"`
					Resolution string `json:"resolution" binding:"required"`
					Note       string `json:"note"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				cs, err := globalComplianceEngine.ResolveCase(c.Request.Context(), id, security.CaseStatus(strings.ToUpper(req.Status)), req.Resolution, userClaims.UserID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				if req.Note != "" {
					_, _ = globalComplianceEngine.AddCaseNote(c.Request.Context(), id, userClaims.UserID, req.Note)
				}

				c.JSON(http.StatusOK, cs)
			})

			compliance.GET("/risk", func(c *gin.Context) {
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				targetUserID := c.Query("user_id")
				if targetUserID == "" {
					targetUserID = userClaims.UserID
				}

				// If querying another user's risk, require admin permissions
				if targetUserID != userClaims.UserID {
					role, _, rErr := GetUserRoleAndStatus(userClaims.UserID)
					allowed, pErr := HasPermission(role, "risk:write")
					if rErr != nil || pErr != nil || !allowed {
						c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: insufficient permissions to view other users' risk scoring"})
						return
					}
				}

				eval, err := globalComplianceEngine.EvaluateRiskScore(c.Request.Context(), targetUserID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, eval)
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

// VerifyTradingCompliance verifies that placing an order complies with active compliance policies and limits (P0009)
func VerifyTradingCompliance(ctx context.Context, userID string, symbol string, side string, price float64, qty float64) error {
	if globalComplianceEngine == nil {
		return nil
	}

	// 1. Account restriction checks
	rest, err := globalComplianceEngine.GetRestriction(ctx, userID)
	if err == nil {
		if rest == security.RestrictionFullyRestricted || rest == security.RestrictionTradingRestricted || rest == security.RestrictionUnderReview {
			return fmt.Errorf("trading is blocked: account restricted status %s", rest)
		}
	}

	// 2. KYC verification status and trading limits gate
	kyc, err := globalComplianceEngine.GetKYC(ctx, userID)
	if err == nil {
		if kyc.Status == security.StatusSuspended || kyc.Status == security.StatusRejected {
			return fmt.Errorf("trading is blocked: KYC status is %s", kyc.Status)
		}

		// 3. Limits validation
		limits, err := globalComplianceEngine.GetLimits(ctx, kyc.Tier)
		if err == nil {
			orderValue := price * qty
			if orderValue > limits.TradingLimitDaily {
				return fmt.Errorf("trading order value of %f exceeds daily limit of %f for tier %s", orderValue, limits.TradingLimitDaily, kyc.Tier)
			}
		}
	}

	// 4. Wash trading / self-trading patterns check
	// Simulating trading rule detection - raises high-severity alerts for abnormal order volumes
	if qty >= 500.0 {
		_, _ = globalComplianceEngine.CreateAlert(ctx, userID, "ord_abnormal", "aml_trading_abnormal_volume", 0.70, fmt.Sprintf("High volume trading order of %f %s symbol requested", qty, symbol), security.SeverityHigh)
	}

	return nil
}
