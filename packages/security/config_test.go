package security

import (
	"os"
	"testing"
)

func TestConfigValidationDevelopment(t *testing.T) {
	// Clean environment
	os.Unsetenv("APP_ENV")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("API_KEY_MASTER_SECRET")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_SSLMODE")
	os.Unsetenv("APP_DEBUG")

	cfg, err := LoadAndValidateConfig(nil)
	if err != nil {
		t.Fatalf("Expected development config to load successfully, got: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Errorf("Expected APP_ENV=development, got %s", cfg.AppEnv)
	}

	if cfg.JWTSecret != DefaultDevJWTSecret {
		t.Errorf("Expected default JWT secret, got %s", cfg.JWTSecret)
	}
}

func TestConfigValidationProductionFailClosed(t *testing.T) {
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("APP_ENV")

	// 1. Weak SSLMode check
	os.Setenv("DB_SSLMODE", "disable")
	_, err := LoadAndValidateConfig(nil)
	if err == nil {
		t.Fatal("Expected validation to fail due to insecure SSLMode in production")
	}

	// Correct SSLMode for next checks
	os.Setenv("DB_SSLMODE", "require")
	defer os.Unsetenv("DB_SSLMODE")

	// 2. Default JWT secret check
	os.Setenv("JWT_SECRET", DefaultDevJWTSecret)
	_, err = LoadAndValidateConfig(nil)
	if err == nil {
		t.Fatal("Expected validation to fail due to default JWTSecret in production")
	}

	// Strong JWTSecret
	os.Setenv("JWT_SECRET", "super-long-secure-and-robust-jwt-key-32bytes")
	defer os.Unsetenv("JWT_SECRET")

	// 3. Default API Key Master Secret check
	os.Setenv("API_KEY_MASTER_SECRET", DefaultDevAPIKeyMaster)
	_, err = LoadAndValidateConfig(nil)
	if err == nil {
		t.Fatal("Expected validation to fail due to default APIKeyMasterSecret in production")
	}

	// Strong APIKeyMasterSecret
	os.Setenv("API_KEY_MASTER_SECRET", "super-long-secure-and-robust-api-key-master-32bytes")
	defer os.Unsetenv("API_KEY_MASTER_SECRET")

	// 4. Default DB Password check
	os.Setenv("DB_PASSWORD", DefaultDevDBPassword)
	_, err = LoadAndValidateConfig(nil)
	if err == nil {
		t.Fatal("Expected validation to fail due to default DB_PASSWORD in production")
	}

	// Strong DB Password
	os.Setenv("DB_PASSWORD", "SuperSecurePassword123!")
	defer os.Unsetenv("DB_PASSWORD")

	// 5. APP_DEBUG true in production check
	os.Setenv("APP_DEBUG", "true")
	_, err = LoadAndValidateConfig(nil)
	if err == nil {
		t.Fatal("Expected validation to fail due to APP_DEBUG=true in production")
	}

	// APP_DEBUG false should succeed
	os.Setenv("APP_DEBUG", "false")
	defer os.Unsetenv("APP_DEBUG")

	cfg, err := LoadAndValidateConfig(nil)
	if err != nil {
		t.Fatalf("Expected secure production configuration to load successfully, got %v", err)
	}

	if cfg.DBPassword != "SuperSecurePassword123!" {
		t.Errorf("Expected DBPassword to be SuperSecurePassword123!, got %s", cfg.DBPassword)
	}
}

func TestConfigSecretProvider(t *testing.T) {
	secrets := map[string]string{
		"JWT_SECRET":            "secure-provider-jwt-secret-key-32bytes",
		"API_KEY_MASTER_SECRET": "secure-provider-api-key-master-32bytes",
		"DB_PASSWORD":           "secure-provider-db-password-1234",
	}
	provider := NewMemorySecretProvider(secrets)

	os.Setenv("APP_ENV", "production")
	os.Setenv("DB_SSLMODE", "require")
	defer os.Unsetenv("APP_ENV")
	defer os.Unsetenv("DB_SSLMODE")

	cfg, err := LoadAndValidateConfig(provider)
	if err != nil {
		t.Fatalf("Expected config loading with SecretProvider to succeed, got %v", err)
	}

	if cfg.JWTSecret != secrets["JWT_SECRET"] {
		t.Errorf("Expected secret from provider, got %s", cfg.JWTSecret)
	}
}
