package security

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SecretProvider defines an interface for secure external secret management systems (e.g., Vault, AWS Secrets Manager)
type SecretProvider interface {
	GetSecret(key string) (string, error)
}

// MemorySecretProvider is a mock/concrete implementation of SecretProvider for safe production injection / testing
type MemorySecretProvider struct {
	secrets map[string]string
}

func NewMemorySecretProvider(secrets map[string]string) *MemorySecretProvider {
	return &MemorySecretProvider{secrets: secrets}
}

func (m *MemorySecretProvider) GetSecret(key string) (string, error) {
	val, ok := m.secrets[key]
	if !ok {
		return "", fmt.Errorf("secret key %s not found in provider", key)
	}
	return val, nil
}

// Config holds centralized security and connection configurations
type Config struct {
	AppEnv              string
	Port                string
	JWTSecret           string
	APIKeyMasterSecret  string
	DBHost              string
	DBPort              int
	DBUser              string
	DBPassword          string
	DBName              string
	DBSSLMode           string
	RedisAddr           string
	RedisPassword       string
	KafkaBrokers        []string
	BlockchainRPCSecret string
	Debug               bool
}

var (
	DefaultDevJWTSecret      = "super-secret-velyxora-key-999"
	DefaultDevAPIKeyMaster   = "velyxora-apikeys-master-key-32b"
	DefaultDevDBPassword     = "postgres"
)

// LoadAndValidateConfig reads configuration from environments and optional SecretProvider, then validates rules
func LoadAndValidateConfig(provider SecretProvider) (*Config, error) {
	env := getEnvVar("APP_ENV", "development")
	isProd := strings.ToLower(env) == "production"

	debugStr := getEnvVar("APP_DEBUG", "false")
	debugVal, _ := strconv.ParseBool(debugStr)

	cfg := &Config{
		AppEnv:        env,
		Port:          getEnvVar("PORT", "8080"),
		DBHost:        getEnvVar("DB_HOST", "localhost"),
		DBUser:        getEnvVar("DB_USER", "postgres"),
		DBName:        getEnvVar("DB_NAME", "velyxora"),
		DBSSLMode:     getEnvVar("DB_SSLMODE", "disable"),
		RedisAddr:     getEnvVar("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnvVar("REDIS_PASSWORD", ""),
		Debug:         debugVal,
	}

	dbPortStr := getEnvVar("DB_PORT", "5432")
	dbPort, err := strconv.Atoi(dbPortStr)
	if err != nil {
		dbPort = 5432
	}
	cfg.DBPort = dbPort

	kafkaBrokersStr := getEnvVar("KAFKA_BROKERS", "localhost:9092")
	cfg.KafkaBrokers = strings.Split(kafkaBrokersStr, ",")

	// Fetch secrets from provider if available, fallback to env vars
	cfg.JWTSecret = fetchSecret("JWT_SECRET", DefaultDevJWTSecret, provider)
	cfg.APIKeyMasterSecret = fetchSecret("API_KEY_MASTER_SECRET", DefaultDevAPIKeyMaster, provider)
	cfg.DBPassword = fetchSecret("DB_PASSWORD", DefaultDevDBPassword, provider)
	cfg.BlockchainRPCSecret = fetchSecret("BLOCKCHAIN_RPC_SECRET", "", provider)

	// In production environment: Enforce extreme validation checks to fail-closed on weak defaults
	if isProd {
		// Enforce TLS / secure connection configuration checks
		if cfg.DBSSLMode == "disable" || cfg.DBSSLMode == "" {
			return nil, errors.New("security validation failed: production mode requires secure DB_SSLMODE ('require' or 'verify-full')")
		}

		if cfg.JWTSecret == "" || cfg.JWTSecret == DefaultDevJWTSecret {
			return nil, errors.New("security validation failed: production mode rejects default or empty JWT_SECRET")
		}

		if len(cfg.JWTSecret) < 32 {
			return nil, errors.New("security validation failed: production JWT_SECRET must be at least 32 bytes")
		}

		if cfg.APIKeyMasterSecret == "" || cfg.APIKeyMasterSecret == DefaultDevAPIKeyMaster {
			return nil, errors.New("security validation failed: production mode rejects default or empty API_KEY_MASTER_SECRET")
		}

		if len(cfg.APIKeyMasterSecret) < 32 {
			return nil, errors.New("security validation failed: production API_KEY_MASTER_SECRET must be at least 32 bytes")
		}

		if cfg.DBPassword == "" || cfg.DBPassword == DefaultDevDBPassword {
			return nil, errors.New("security validation failed: production mode rejects default or empty DB_PASSWORD")
		}

		if cfg.Debug {
			return nil, errors.New("security validation failed: debug mode (APP_DEBUG) must be false in production environments")
		}
	}

	return cfg, nil
}

func getEnvVar(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}

func fetchSecret(key, defaultVal string, provider SecretProvider) string {
	if provider != nil {
		val, err := provider.GetSecret(key)
		if err == nil && val != "" {
			return val
		}
	}
	return getEnvVar(key, defaultVal)
}
