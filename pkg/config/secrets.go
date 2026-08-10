package config

import (
	"context"
	"crypto/aes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	awstypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

type SecretsManager struct {
	awsClient    *secretsmanager.Client
	localSecrets map[string]string
	cacheTTL     time.Duration
	cacheTime    map[string]time.Time
	mutex        *sync.RWMutex
	enableAWS    bool
	enableVault  bool
}

type SecretConfig struct {
	AWSRegion     string
	VaultAddr     string
	VaultToken    string
	LocalFallback bool
}

func NewSecretsManager(cfg SecretConfig) (*SecretsManager, error) {
	sm := &SecretsManager{
		localSecrets: make(map[string]string),
		cacheTime:    make(map[string]time.Time),
		mutex:        &sync.RWMutex{},
		cacheTTL:     1 * time.Hour,
		enableAWS:    cfg.AWSRegion != "",
		enableVault:  cfg.VaultAddr != "",
	}

	if sm.enableAWS {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.AWSRegion))
		if err != nil {
			if cfg.LocalFallback {
				sm.enableAWS = false
			} else {
				return nil, fmt.Errorf("failed to load AWS config: %w", err)
			}
		} else {
			sm.awsClient = secretsmanager.NewFromConfig(awsCfg)
		}
	}

	return sm, nil
}

func (sm *SecretsManager) GetSecret(ctx context.Context, secretName string) (string, error) {
	sm.mutex.RLock()
	if cachedValue, exists := sm.localSecrets[secretName]; exists {
		if cachedTime, timeExists := sm.cacheTime[secretName]; timeExists {
			if time.Since(cachedTime) < sm.cacheTTL {
				sm.mutex.RUnlock()
				return cachedValue, nil
			}
		}
	}
	sm.mutex.RUnlock()

	var secret string
	var err error

	if sm.enableAWS && sm.awsClient != nil {
		secret, err = sm.getSecretFromAWS(ctx, secretName)
		if err == nil {
			sm.cacheSecret(secretName, secret)
			return secret, nil
		}
	}

	secret, err = sm.getSecretFromEnv(secretName)
	if err == nil {
		sm.cacheSecret(secretName, secret)
		return secret, nil
	}

	return "", fmt.Errorf("secret not found: %s", secretName)
}

func (sm *SecretsManager) getSecretFromAWS(ctx context.Context, secretName string) (string, error) {
	if sm.awsClient == nil {
		return "", fmt.Errorf("AWS secrets manager not initialized")
	}

	result, err := sm.awsClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get secret from AWS: %w", err)
	}

	if result.SecretString != nil {
		return *result.SecretString, nil
	}
	if result.SecretBinary != nil {
		return base64.StdEncoding.EncodeToString(result.SecretBinary), nil
	}

	return "", fmt.Errorf("secret has no string or binary value")
}

func (sm *SecretsManager) getSecretFromEnv(secretName string) (string, error) {
	value := os.Getenv(secretName)
	if value == "" {
		return "", fmt.Errorf("environment variable not found: %s", secretName)
	}
	return value, nil
}

func (sm *SecretsManager) cacheSecret(name, value string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.localSecrets[name] = value
	sm.cacheTime[name] = time.Now()
}

func (sm *SecretsManager) StoreSecret(ctx context.Context, secretName, secretValue string, tags map[string]string) error {
	if !sm.enableAWS || sm.awsClient == nil {
		sm.cacheSecret(secretName, secretValue)
		return nil
	}

	tagsList := make([]awstypes.Tag, 0, len(tags))
	for k, v := range tags {
		key := k
		value := v
		tagsList = append(tagsList, awstypes.Tag{
			Key:   &key,
			Value: &value,
		})
	}

	_, err := sm.awsClient.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(secretValue),
		Tags:         tagsList,
	})
	if err != nil {
		return fmt.Errorf("failed to store secret in AWS: %w", err)
	}

	sm.cacheSecret(secretName, secretValue)
	return nil
}

func (sm *SecretsManager) RotateSecret(ctx context.Context, secretName string, newValue string) error {
	if !sm.enableAWS || sm.awsClient == nil {
		sm.cacheSecret(secretName, newValue)
		return nil
	}

	_, err := sm.awsClient.UpdateSecret(ctx, &secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(newValue),
	})
	if err != nil {
		return fmt.Errorf("failed to rotate secret: %w", err)
	}

	sm.cacheSecret(secretName, newValue)
	return nil
}

func (sm *SecretsManager) DeleteSecret(ctx context.Context, secretName string) error {
	if sm.enableAWS && sm.awsClient != nil {
		_, err := sm.awsClient.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
			SecretId: aws.String(secretName),
		})
		if err != nil {
			return fmt.Errorf("failed to delete secret: %w", err)
		}
	}

	sm.mutex.Lock()
	delete(sm.localSecrets, secretName)
	delete(sm.cacheTime, secretName)
	sm.mutex.Unlock()

	return nil
}

func (sm *SecretsManager) GenerateSecureRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func (sm *SecretsManager) EncryptSecretValue(plaintext, encryptionKey string) (string, error) {
	key := make([]byte, 32)
	copy(key, []byte(encryptionKey))

	cipher, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := cipher.Encrypt(nil, []byte(plaintext))
	result := append(nonce, ciphertext...)

	return base64.StdEncoding.EncodeToString(result), nil
}

func (sm *SecretsManager) DecryptSecretValue(ciphertext, encryptionKey string) (string, error) {
	key := make([]byte, 32)
	copy(key, []byte(encryptionKey))

	cipher, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonce := data[:12]
	ciphertext = string(data[12:])

	plaintext := cipher.Decrypt(nil, []byte(ciphertext))
	return string(plaintext), nil
}
