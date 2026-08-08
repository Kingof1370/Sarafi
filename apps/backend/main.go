package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"velyxora/packages/common"
	"velyxora/packages/custody"
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
var globalCustodyEngine *custody.CustodyEngine

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
	globalCustodyEngine = custody.NewCustodyEngine(globalDB)

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

	// 6.5 Setup WebSocket Gateway (P005 real-time core)
	wsGateway := NewWSGateway(cfg.JWTSecret)
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

			// Request deposit wallet address validation/allocation with unique persistent database checks
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

				// Check if address already exists for user and asset to prevent duplicate allocation
				var address string
				if globalDB != nil {
					err = globalDB.Pool.QueryRow(c.Request.Context(),
						"SELECT address FROM wallet_addresses WHERE user_id = $1 AND asset = $2 AND is_active = TRUE",
						userClaims.UserID, strings.ToUpper(req.Asset)).Scan(&address)
				}

				if address == "" {
					// Dynamically generate a uniquely assigned, cryptographically valid address for the user/asset
					salt := fmt.Sprintf("%s_%s_%d", userClaims.UserID, req.Asset, time.Now().UnixNano())
					sum := sha256.Sum256([]byte(salt))

					if strings.ToUpper(req.Asset) == "BTC" {
						// Cryptographically valid Mainnet P2PKH Bitcoin Address (Base58Check with version 0x00)
						address = common.EncodeBase58Check(0x00, sum[:20])
					} else if strings.ToUpper(req.Asset) == "SOL" {
						// Cryptographically valid Solana Address (Base58 encoded Ed25519 public key-like 32-byte representation)
						address = common.EncodeBase58(sum[:32])
					} else {
						// Cryptographically valid Ethereum format address string
						hash := hex.EncodeToString(sum[:])
						address = "0x" + hash[:40]
					}

					// Verify cryptographic validity against the specific adapter
					if !adapter.ValidateAddress(address) {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to formulate valid destination key address"})
						return
					}

					// Persist newly allocated address to database
					if globalDB != nil {
						// Ensure no cross-user ownership of address (unique constraint violation guard)
						var ownerCheck string
						_ = globalDB.Pool.QueryRow(c.Request.Context(),
							"SELECT user_id FROM wallet_addresses WHERE address = $1", address).Scan(&ownerCheck)

						if ownerCheck != "" {
							c.JSON(http.StatusConflict, gin.H{"error": "Cryptographic conflict: Generated address has duplicate allocation"})
							return
						}

						_, err = globalDB.Pool.Exec(c.Request.Context(),
							"INSERT INTO wallet_addresses (user_id, asset, address, memo, is_active, created_at) VALUES ($1, $2, $3, $4, TRUE, NOW())",
							userClaims.UserID, strings.ToUpper(req.Asset), address, "Simulated key-derived address")
						if err != nil {
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist wallet address to database"})
							return
						}
					}
				}

				c.JSON(http.StatusOK, types.WalletAddress{
					UserID:    userClaims.UserID,
					Asset:     strings.ToUpper(req.Asset),
					Address:   address,
					IsActive:  true,
					CreatedAt: time.Now(),
				})
			})

			// Process withdrawal requests with RBAC and MFA hardening
			wallet.POST("/withdraw", RBACMiddleware("wallet:write"), func(c *gin.Context) {
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

				if !adapter.ValidateAddress(req.Address) {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid destination blockchain address format"})
					return
				}

				// Strict MFA verification if enabled & exceeds basic limit
				if globalDB != nil {
					var isMFAEnabled bool
					var mfaSecret string
					err = globalDB.Pool.QueryRow(c.Request.Context(),
						"SELECT is_mfa_enabled, mfa_secret FROM users WHERE id = $1", userClaims.UserID).
						Scan(&isMFAEnabled, &mfaSecret)

					if err == nil && isMFAEnabled {
						// Require TOTP for any withdrawal over 0.1 BTC, 1.0 ETH or 10.0 SOL/USDT
						threshold := 1.0
						if req.Asset == "BTC" {
							threshold = 0.1
						} else if req.Asset == "SOL" {
							threshold = 10.0
						}

						if req.Amount >= threshold {
							mfaCode := c.GetHeader("X-MFA-Code")
							if mfaCode == "" {
								c.JSON(http.StatusForbidden, gin.H{"error": "Multi-Factor Authentication (MFA) token is required for this transaction", "code": "MFA_REQUIRED"})
								return
							}

							if !security.ValidateTOTP(mfaSecret, mfaCode) {
								c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Multi-Factor Authentication (MFA) token code"})
								return
							}
						}
					}
				}

				fee := 0.0005
				if strings.ToUpper(req.Asset) == "USDT" || strings.ToUpper(req.Asset) == "USDC" {
					fee = 1.0
				}

				totalAmount := common.RoundToPrecision(req.Amount+fee, 8)

				// Determine if dual approval is required by custody policy
				var startStatus types.WithdrawalStatus = types.WithdrawalApproved
				if globalCustodyEngine != nil {
					needsDualApproval, err := globalCustodyEngine.ValidateWithdrawal(req.Asset, req.Amount)
					if err != nil {
						c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
						return
					}
					if needsDualApproval {
						startStatus = types.WithdrawalPendingApproval
					}
				}

				wthID := "wth_" + fmt.Sprintf("%d", time.Now().UnixNano())

				// Atomic Available Balance Check & Reservation check (Move to reserved segment)
				if globalDB != nil {
					tx, err := globalDB.Pool.Begin(c.Request.Context())
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
						return
					}
					defer tx.Rollback(c.Request.Context())

					var available, reserved, total float64
					err = tx.QueryRow(c.Request.Context(),
						"SELECT available, reserved, total FROM balances WHERE user_id = $1 AND asset = $2 FOR UPDATE",
						userClaims.UserID, req.Asset).Scan(&available, &reserved, &total)

					if err != nil {
						c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient wallet funds: balance row does not exist"})
						return
					}

					if available < totalAmount {
						c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Insufficient available balance: has %f, needs %f (amount: %f + fee: %f)", available, totalAmount, req.Amount, fee)})
						return
					}

					// Deduct available, increase reserved segment
					newAvailable := common.RoundToPrecision(available-totalAmount, 8)
					newReserved := common.RoundToPrecision(reserved+totalAmount, 8)

					_, err = tx.Exec(c.Request.Context(),
						"UPDATE balances SET available = $1, reserved = $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4",
						newAvailable, newReserved, userClaims.UserID, req.Asset)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update balances"})
						return
					}

					// Insert withdrawal record
					_, err = tx.Exec(c.Request.Context(),
						"INSERT INTO withdrawals (id, user_id, asset, amount, fee, address, status, risk_score, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())",
						wthID, userClaims.UserID, req.Asset, req.Amount, fee, req.Address, string(startStatus), 0.15)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register withdrawal record"})
						return
					}

					// If dual approval is required, register a custody approval request row
					if startStatus == types.WithdrawalPendingApproval && globalCustodyEngine != nil {
						_, err = globalCustodyEngine.CreateApprovalRequest(c.Request.Context(), wthID, req.Asset, req.Amount)
						if err != nil {
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate custody governance approval"})
							return
						}
					}

					err = tx.Commit(c.Request.Context())
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
						return
					}
				}

				// Publish withdrawal requested event
				if globalKafkaProducer != nil {
					event := types.KafkaEvent{
						Type: "withdrawal.requested",
						Payload: map[string]interface{}{
							"withdrawal_id": wthID,
							"user_id":       userClaims.UserID,
							"asset":         req.Asset,
							"amount":        req.Amount,
							"fee":           fee,
							"address":       req.Address,
							"status":        string(startStatus),
						},
						Timestamp: time.Now(),
					}
					_ = globalKafkaProducer.Publish(c.Request.Context(), "velyxora-wallet-updates", "withdrawal.requested", event)
				}

				c.JSON(http.StatusAccepted, gin.H{
					"message": "Withdrawal request submitted successfully",
					"withdrawal": gin.H{
						"id":      wthID,
						"asset":   req.Asset,
						"amount":  req.Amount,
						"fee":     fee,
						"address": req.Address,
						"status":  string(startStatus),
					},
				})
			})
		}

		// Reconciliation and Custody Management Administrative endpoints
		recon := v1.Group("/reconciliation")
		recon.Use(UnifiedAuthMiddleware(cfg.JWTSecret))
		{
			recon.GET("/status", func(c *gin.Context) {
				runs := []gin.H{}
				if globalDB != nil {
					rows, err := globalDB.Pool.Query(c.Request.Context(),
						"SELECT id, status, started_at, completed_at FROM reconciliation_runs ORDER BY started_at DESC LIMIT 50")
					if err == nil {
						defer rows.Close()
						for rows.Next() {
							var id, status string
							var startedAt time.Time
							var completedAt *time.Time
							if rows.Scan(&id, &status, &startedAt, &completedAt) == nil {
								runs = append(runs, gin.H{
									"id":           id,
									"status":       status,
									"started_at":   startedAt,
									"completed_at": completedAt,
								})
							}
						}
					}
				}
				c.JSON(http.StatusOK, runs)
			})

			recon.GET("/issues", func(c *gin.Context) {
				issues := []gin.H{}
				if globalDB != nil {
					rows, err := globalDB.Pool.Query(c.Request.Context(),
						"SELECT id, run_id, layer, severity, asset, details, timestamp FROM reconciliation_issues ORDER BY timestamp DESC LIMIT 100")
					if err == nil {
						defer rows.Close()
						for rows.Next() {
							var id, runID, layer, severity, asset, details string
							var timestamp time.Time
							if rows.Scan(&id, &runID, &layer, &severity, &asset, &details, &timestamp) == nil {
								issues = append(issues, gin.H{
									"id":        id,
									"run_id":    runID,
									"layer":     layer,
									"severity":  severity,
									"asset":     asset,
									"details":   details,
									"timestamp": timestamp,
								})
							}
						}
					}
				}
				c.JSON(http.StatusOK, issues)
			})
		}

		custodyGroup := v1.Group("/custody")
		custodyGroup.Use(UnifiedAuthMiddleware(cfg.JWTSecret))
		{
			custodyGroup.POST("/approve/:id", RBACMiddleware("risk:write"), func(c *gin.Context) {
				approvalID := c.Param("id")
				claims, _ := c.Get("claims")
				userClaims := claims.(*security.Claims)

				if globalCustodyEngine == nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Custody governance engine not configured"})
					return
				}

				app, err := globalCustodyEngine.SubmitApproval(c.Request.Context(), approvalID, userClaims.Email)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				// If APPROVED, we update the withdrawal record to APPROVED in the DB
				if app.Status == "APPROVED" && globalDB != nil {
					_, err = globalDB.Pool.Exec(c.Request.Context(),
						"UPDATE withdrawals SET status = 'APPROVED', updated_at = NOW() WHERE id = $1",
						app.WithdrawalID)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update withdrawal queue status"})
						return
					}

					if globalKafkaProducer != nil {
						_ = globalKafkaProducer.Publish(c.Request.Context(), "velyxora-wallet-updates", "withdrawal.approved", map[string]interface{}{
							"withdrawal_id": app.WithdrawalID,
							"status":        "APPROVED",
						})
					}
				}

				c.JSON(http.StatusOK, gin.H{
					"message":  "Custody governance signature registered",
					"approval": app,
				})
			})

			custodyGroup.POST("/freeze", RBACMiddleware("risk:write"), func(c *gin.Context) {
				if globalCustodyEngine == nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Custody governance engine not configured"})
					return
				}

				globalCustodyEngine.SetGlobalFreeze(true)
				c.JSON(http.StatusOK, gin.H{"message": "Global emergency freeze is now ACTIVE. All transfers blocked."})
			})

			custodyGroup.POST("/unfreeze", RBACMiddleware("risk:write"), func(c *gin.Context) {
				if globalCustodyEngine == nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Custody governance engine not configured"})
					return
				}

				globalCustodyEngine.SetGlobalFreeze(false)
				c.JSON(http.StatusOK, gin.H{"message": "Global emergency freeze has been disabled."})
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
