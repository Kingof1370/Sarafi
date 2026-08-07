package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plain text password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash compares a hashed password with a plain text one
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CompareBcrypt performs a standard bcrypt hash comparison, returning the error
func CompareBcrypt(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// Claims represents JWT payload structure
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateJWT creates both Access and Refresh JWT tokens
func GenerateJWT(userID, email, secret string, accessDuration, refreshDuration time.Duration) (string, string, error) {
	// Access Token
	accessClaims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "velyxora-exchange",
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}

	// Refresh Token
	refreshClaims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(refreshDuration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Issuer:    "velyxora-exchange",
		Subject:   userID,
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}

	return accessStr, refreshStr, nil
}

// ValidateJWT parses and validates a JWT token string
func ValidateJWT(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid claims")
	}

	return claims, nil
}

// VerifyAPIKeySignature verifies a payload's signature using the api secret key.
// Typical secure API validation for programmatic exchange operations.
func VerifyAPIKeySignature(payload, signature, apiSecret string) bool {
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(payload))
	expectedMAC := mac.Sum(nil)
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(sigBytes, expectedMAC)
}
