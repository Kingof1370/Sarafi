package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps redis.Client to provide common caching routines
type RedisClient struct {
	Client *redis.Client
}

// RedisConfig holds connection details for Redis cache layer
type RedisConfig struct {
	Addr     string // "host:port"
	Password string
	DB       int
	UseTLS   bool
}

// NewRedisClient creates and validates connection to Redis
func NewRedisClient(cfg RedisConfig) (*RedisClient, error) {
	var tlsConfig *tls.Config
	if cfg.UseTLS || os.Getenv("REDIS_USE_TLS") == "true" {
		insecure := os.Getenv("REDIS_TLS_INSECURE") == "true"
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: insecure,
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:      cfg.Addr,
		Password:  cfg.Password,
		DB:        cfg.DB,
		TLSConfig: tlsConfig,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return &RedisClient{Client: client}, nil
}

// Set stores a key-value pair in Redis cache with an optional TTL
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.Client.Set(ctx, key, value, ttl).Err()
}

// Get retrieves a value from Redis cache
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.Client.Get(ctx, key).Result()
}

// Del removes a key from Redis cache
func (r *RedisClient) Del(ctx context.Context, key string) error {
	return r.Client.Del(ctx, key).Err()
}

// Ping checks the health of the Redis client
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.Client.Ping(ctx).Err()
}
