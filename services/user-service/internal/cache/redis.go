package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

// RedisService handles Redis operations
type RedisService struct {
	Client *redis.Client
}

// NewRedisService creates a new Redis service
func NewRedisService() (*RedisService, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found in cache package, using system env")
	}

	// Get Redis configuration from environment
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}

	password := os.Getenv("REDIS_PASSWORD")
	db := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if parsed, err := fmt.Sscanf(dbStr, "%d", &db); err != nil || parsed != 1 {
			db = 0
		}
	}

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisService{Client: rdb}, nil
}

// Set stores a key-value pair with expiration
func (rs *RedisService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return rs.Client.Set(ctx, key, jsonValue, expiration).Err()
}

// Get retrieves a value by key
func (rs *RedisService) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := rs.Client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("key not found")
		}
		return fmt.Errorf("failed to get value: %w", err)
	}

	return json.Unmarshal([]byte(val), dest)
}

// Delete removes a key
func (rs *RedisService) Delete(ctx context.Context, key string) error {
	return rs.Client.Del(ctx, key).Err()
}

// Exists checks if a key exists
func (rs *RedisService) Exists(ctx context.Context, key string) (bool, error) {
	result, err := rs.Client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}
	return result > 0, nil
}

// SetOTP stores OTP with expiration (5 minutes)
func (rs *RedisService) SetOTP(ctx context.Context, email, otp string) error {
	key := fmt.Sprintf("otp:%s", email)
	return rs.Set(ctx, key, otp, 5*time.Minute)
}

// GetOTP retrieves OTP
func (rs *RedisService) GetOTP(ctx context.Context, email string) (string, error) {
	key := fmt.Sprintf("otp:%s", email)
	var otp string
	err := rs.Get(ctx, key, &otp)
	return otp, err
}

// DeleteOTP removes OTP
func (rs *RedisService) DeleteOTP(ctx context.Context, email string) error {
	key := fmt.Sprintf("otp:%s", email)
	return rs.Delete(ctx, key)
}

// SetUserSession stores user session data
func (rs *RedisService) SetUserSession(ctx context.Context, userID, sessionID string, data interface{}) error {
	key := fmt.Sprintf("session:%s:%s", userID, sessionID)
	return rs.Set(ctx, key, data, 24*time.Hour) // 24 hours
}

// GetUserSession retrieves user session data
func (rs *RedisService) GetUserSession(ctx context.Context, userID, sessionID string, dest interface{}) error {
	key := fmt.Sprintf("session:%s:%s", userID, sessionID)
	return rs.Get(ctx, key, dest)
}

// DeleteUserSession removes user session
func (rs *RedisService) DeleteUserSession(ctx context.Context, userID, sessionID string) error {
	key := fmt.Sprintf("session:%s:%s", userID, sessionID)
	return rs.Delete(ctx, key)
}

// SetRateLimit stores rate limit data
func (rs *RedisService) SetRateLimit(ctx context.Context, key string, count int, window time.Duration) error {
	return rs.Set(ctx, key, count, window)
}

// GetRateLimit retrieves rate limit data
func (rs *RedisService) GetRateLimit(ctx context.Context, key string) (int, error) {
	var count int
	err := rs.Get(ctx, key, &count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// IncrementRateLimit increments rate limit counter
func (rs *RedisService) IncrementRateLimit(ctx context.Context, key string, window time.Duration) (int, error) {
	pipe := rs.Client.Pipeline()
	
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to increment rate limit: %w", err)
	}
	
	return int(incr.Val()), nil
}

// Close closes the Redis connection
func (rs *RedisService) Close() error {
	return rs.Client.Close()
}
