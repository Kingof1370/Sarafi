package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	DeviceID  string `json:"device_id"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret            []byte
	expiryDuration    time.Duration
	refreshExpiryTime time.Duration
	issuer            string
	audience          string
	mutex             *sync.RWMutex
}

func NewJWTManager(secret string, expiryHours int, issuer string) *JWTManager {
	return &JWTManager{
		secret:            []byte(secret),
		expiryDuration:    time.Duration(expiryHours) * time.Hour,
		refreshExpiryTime: time.Duration(expiryHours*7) * time.Hour,
		issuer:            issuer,
		audience:          "velyxora-exchange",
		mutex:             &sync.RWMutex{},
	}
}

func (jm *JWTManager) GenerateToken(userID, email, role, sessionID, ipAddress, userAgent, deviceID string) (string, error) {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	now := time.Now()
	expiresAt := now.Add(jm.expiryDuration)

	claims := &Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		SessionID: sessionID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		DeviceID:  deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    jm.issuer,
			Audience:  jwt.ClaimStrings{jm.audience},
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jm.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (jm *JWTManager) GenerateRefreshToken(userID, sessionID string) (string, error) {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	now := time.Now()
	expiresAt := now.Add(jm.refreshExpiryTime)

	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    jm.issuer,
		Subject:   userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jm.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return tokenString, nil
}

func (jm *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jm.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func (jm *JWTManager) RefreshToken(refreshTokenString string) (string, error) {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(refreshTokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jm.secret, nil
	})

	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid refresh token")
	}

	if claims.Subject == "" {
		return "", fmt.Errorf("invalid refresh token subject")
	}

	newClaims := &Claims{
		UserID: claims.Subject,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jm.expiryDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    jm.issuer,
			Subject:   claims.Subject,
		},
	}

	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
	newTokenString, err := newToken.SignedString(jm.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign new token: %w", err)
	}

	return newTokenString, nil
}

func (jm *JWTManager) RevokeToken(tokenString string) error {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	// TODO: Store revoked tokens in Redis with TTL
	return nil
}

func (jm *JWTManager) IsTokenRevoked(tokenString string) bool {
	// TODO: Check if token is in revocation list
	return false
}

func (jm *JWTManager) GenerateAPISignature(method, path, body, apiSecret string) string {
	message := method + "\n" + path + "\n" + body
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func (jm *JWTManager) ValidateAPISignature(method, path, body, signature, apiSecret string) bool {
	expectedSignature := jm.GenerateAPISignature(method, path, body, apiSecret)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
