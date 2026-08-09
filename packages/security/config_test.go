package security

import (
	"os"
	"testing"
)

func TestValidateProductionEnvironment(t *testing.T) {
	// 1. Should succeed if not production environment
	os.Setenv("APP_ENV", "development")
	if err := ValidateProductionEnvironment(); err != nil {
		t.Errorf("expected no error in development environment, got %v", err)
	}

	// 2. Should fail if APP_ENV is production but secrets are missing
	os.Setenv("APP_ENV", "production")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("API_KEY_MASTER_SECRET")
	os.Unsetenv("BACKUP_ENCRYPTION_KEY")
	os.Unsetenv("DB_PASSWORD")
	if err := ValidateProductionEnvironment(); err == nil {
		t.Error("expected error for missing production configurations")
	}

	// 3. Should fail if secrets are weak defaults
	os.Setenv("JWT_SECRET", "super-secure-velyxora-key-999")
	os.Setenv("API_KEY_MASTER_SECRET", "velyxora-apikeys-master-key-32b")
	os.Setenv("BACKUP_ENCRYPTION_KEY", "velyxora-backup-default-aes-key32")
	os.Setenv("DB_PASSWORD", "postgres")
	if err := ValidateProductionEnvironment(); err == nil {
		t.Error("expected error for known default/weak secrets in production")
	}

	// 4. Should fail if keys are too short
	os.Setenv("JWT_SECRET", "short-key")
	os.Setenv("API_KEY_MASTER_SECRET", "short-key-2")
	os.Setenv("BACKUP_ENCRYPTION_KEY", "short-key-3")
	os.Setenv("DB_PASSWORD", "strongpassword123")
	if err := ValidateProductionEnvironment(); err == nil {
		t.Error("expected error for short key lengths")
	}

	// 5. Should succeed when secure production variables are supplied
	os.Setenv("JWT_SECRET", "very-strong-production-jwt-secret-key-that-exceeds-32-chars")
	os.Setenv("API_KEY_MASTER_SECRET", "very-strong-production-api-master-secret-key-32-chars")
	os.Setenv("BACKUP_ENCRYPTION_KEY", "very-strong-production-backup-secret-key-32-chars")
	os.Setenv("DB_PASSWORD", "very-strong-production-database-password-999")
	os.Setenv("LOG_LEVEL", "INFO")
	if err := ValidateProductionEnvironment(); err != nil {
		t.Errorf("expected no error for healthy production environment, got %v", err)
	}

	// Clean up env
	os.Unsetenv("APP_ENV")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("API_KEY_MASTER_SECRET")
	os.Unsetenv("BACKUP_ENCRYPTION_KEY")
	os.Unsetenv("DB_PASSWORD")
}
