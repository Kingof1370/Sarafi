package security

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// InsecureDefaults lists known development or insecure fallback secret values
var InsecureDefaults = map[string]bool{
	"super-secure-velyxora-key-999":                     true,
	"super-secret-velyxora-key-999":                     true,
	"velyxora-jwt-secret-key-signature":                 true,
	"velyxora-apikeys-master-key-32b":                   true,
	"velyxora-extremely-secure-aes256-backup-key-10014": true,
	"velyxora-backup-default-aes-key32":                 true,
	"postgres":                                          true,
	"super-secure-db-password-123":                     true,
	"password":                                          true,
	"123456":                                            true,
	"admin":                                             true,
}

// ValidateProductionEnvironment validates that all required secrets are loaded securely when APP_ENV is production
func ValidateProductionEnvironment() error {
	appEnv := os.Getenv("APP_ENV")
	// If APP_ENV is empty or not "production", do not fail system startup for development/standalone testing.
	if !strings.EqualFold(appEnv, "production") {
		return nil
	}

	// 1. Validate JWT_SECRET
	jwtSec := os.Getenv("JWT_SECRET")
	if jwtSec == "" {
		return errors.New("fail closed: mandatory JWT_SECRET is missing in production environment")
	}
	if len(jwtSec) < 32 {
		return fmt.Errorf("fail closed: JWT_SECRET must be at least 32 characters/bytes for secure HMAC-SHA256, got length %d", len(jwtSec))
	}
	if InsecureDefaults[jwtSec] {
		return fmt.Errorf("fail closed: known weak or default JWT_SECRET '%s' is prohibited in production", jwtSec)
	}

	// 2. Validate API_KEY_MASTER_SECRET
	apiKeySec := os.Getenv("API_KEY_MASTER_SECRET")
	if apiKeySec == "" {
		return errors.New("fail closed: mandatory API_KEY_MASTER_SECRET is missing in production environment")
	}
	if len(apiKeySec) < 32 {
		return fmt.Errorf("fail closed: API_KEY_MASTER_SECRET must be at least 32 characters/bytes for secure envelope encryption, got length %d", len(apiKeySec))
	}
	if InsecureDefaults[apiKeySec] {
		return fmt.Errorf("fail closed: known weak or default API_KEY_MASTER_SECRET is prohibited in production")
	}

	// 3. Validate BACKUP_ENCRYPTION_KEY
	backupSec := os.Getenv("BACKUP_ENCRYPTION_KEY")
	if backupSec == "" {
		return errors.New("fail closed: mandatory BACKUP_ENCRYPTION_KEY is missing in production environment")
	}
	if len(backupSec) < 32 {
		return fmt.Errorf("fail closed: BACKUP_ENCRYPTION_KEY must be at least 32 characters/bytes for secure AES-256 backup encryption, got length %d", len(backupSec))
	}
	if InsecureDefaults[backupSec] {
		return fmt.Errorf("fail closed: known weak or default BACKUP_ENCRYPTION_KEY is prohibited in production")
	}

	// 4. Validate DB_PASSWORD
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		return errors.New("fail closed: mandatory DB_PASSWORD is missing in production environment")
	}
	if InsecureDefaults[dbPass] {
		return fmt.Errorf("fail closed: known weak or default DB_PASSWORD '%s' is prohibited in production", dbPass)
	}

	// 5. Check Debug and Logging Modes
	logLevel := os.Getenv("LOG_LEVEL")
	if strings.EqualFold(logLevel, "DEBUG") {
		return errors.New("fail closed: active DEBUG log level is prohibited in production environment to prevent sensitive data leakage")
	}

	// 6. Check Database SSL/TLS Settings
	dbHost := os.Getenv("DB_HOST")
	dbSSLMode := os.Getenv("DB_SSLMODE")
	// If the host is not local, force secure TLS configuration
	if dbHost != "" && dbHost != "localhost" && dbHost != "127.0.0.1" {
		if dbSSLMode == "disable" || dbSSLMode == "" {
			return fmt.Errorf("fail closed: unencrypted database network configuration detected (DB_SSLMODE=%s). TLS is mandatory for remote databases in production", dbSSLMode)
		}
	}

	return nil
}
