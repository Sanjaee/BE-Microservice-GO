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

// CacheService handles Redis caching operations
type CacheService struct {
	client *redis.Client
	ctx    context.Context
}

// NewCacheService creates a new cache service
func NewCacheService() (*CacheService, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found in cache package, using system env")
	}

	// Get Redis configuration from environment
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	password := os.Getenv("REDIS_PASSWORD")
	if password == "" {
		password = ""
	}

	db := 0
	if os.Getenv("REDIS_DB") != "" {
		if _, err := fmt.Sscanf(os.Getenv("REDIS_DB"), "%d", &db); err != nil {
			log.Printf("⚠️ Invalid REDIS_DB value, using default: %d", db)
		}
	}

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()

	// Test connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Println("✅ Connected to Redis successfully")

	return &CacheService{
		client: rdb,
		ctx:    ctx,
	}, nil
}

// SetPayment caches payment data
func (cs *CacheService) SetPayment(paymentID string, data interface{}, expiration time.Duration) error {
	key := fmt.Sprintf("payment:%s", paymentID)
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal payment data: %w", err)
	}

	err = cs.client.Set(cs.ctx, key, jsonData, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to cache payment: %w", err)
	}

	log.Printf("💾 Cached payment: %s", paymentID)
	return nil
}

// GetPayment retrieves payment data from cache
func (cs *CacheService) GetPayment(paymentID string, dest interface{}) error {
	key := fmt.Sprintf("payment:%s", paymentID)
	
	val, err := cs.client.Get(cs.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("payment not found in cache")
		}
		return fmt.Errorf("failed to get payment from cache: %w", err)
	}

	err = json.Unmarshal([]byte(val), dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal payment data: %w", err)
	}

	return nil
}

// DeletePayment removes payment from cache
func (cs *CacheService) DeletePayment(paymentID string) error {
	key := fmt.Sprintf("payment:%s", paymentID)
	
	err := cs.client.Del(cs.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete payment from cache: %w", err)
	}

	log.Printf("🗑️ Deleted payment from cache: %s", paymentID)
	return nil
}

// SetPaymentByOrderID caches payment data by order ID
func (cs *CacheService) SetPaymentByOrderID(orderID string, data interface{}, expiration time.Duration) error {
	key := fmt.Sprintf("payment:order:%s", orderID)
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal payment data: %w", err)
	}

	err = cs.client.Set(cs.ctx, key, jsonData, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to cache payment by order ID: %w", err)
	}

	log.Printf("💾 Cached payment by order ID: %s", orderID)
	return nil
}

// GetPaymentByOrderID retrieves payment data by order ID from cache
func (cs *CacheService) GetPaymentByOrderID(orderID string, dest interface{}) error {
	key := fmt.Sprintf("payment:order:%s", orderID)
	
	val, err := cs.client.Get(cs.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("payment not found in cache")
		}
		return fmt.Errorf("failed to get payment from cache: %w", err)
	}

	err = json.Unmarshal([]byte(val), dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal payment data: %w", err)
	}

	return nil
}

// DeletePaymentByOrderID removes payment by order ID from cache
func (cs *CacheService) DeletePaymentByOrderID(orderID string) error {
	key := fmt.Sprintf("payment:order:%s", orderID)
	
	err := cs.client.Del(cs.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete payment from cache: %w", err)
	}

	log.Printf("🗑️ Deleted payment by order ID from cache: %s", orderID)
	return nil
}

// SetUserPayments caches user payments list
func (cs *CacheService) SetUserPayments(userID string, data interface{}, expiration time.Duration) error {
	key := fmt.Sprintf("user:payments:%s", userID)
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal user payments data: %w", err)
	}

	err = cs.client.Set(cs.ctx, key, jsonData, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to cache user payments: %w", err)
	}

	log.Printf("💾 Cached user payments: %s", userID)
	return nil
}

// GetUserPayments retrieves user payments from cache
func (cs *CacheService) GetUserPayments(userID string, dest interface{}) error {
	key := fmt.Sprintf("user:payments:%s", userID)
	
	val, err := cs.client.Get(cs.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("user payments not found in cache")
		}
		return fmt.Errorf("failed to get user payments from cache: %w", err)
	}

	err = json.Unmarshal([]byte(val), dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal user payments data: %w", err)
	}

	return nil
}

// DeleteUserPayments removes user payments from cache
func (cs *CacheService) DeleteUserPayments(userID string) error {
	key := fmt.Sprintf("user:payments:%s", userID)
	
	err := cs.client.Del(cs.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete user payments from cache: %w", err)
	}

	log.Printf("🗑️ Deleted user payments from cache: %s", userID)
	return nil
}

// SetMidtransTransaction caches Midtrans transaction data
func (cs *CacheService) SetMidtransTransaction(transactionID string, data interface{}, expiration time.Duration) error {
	key := fmt.Sprintf("midtrans:transaction:%s", transactionID)
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal Midtrans transaction data: %w", err)
	}

	err = cs.client.Set(cs.ctx, key, jsonData, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to cache Midtrans transaction: %w", err)
	}

	log.Printf("💾 Cached Midtrans transaction: %s", transactionID)
	return nil
}

// GetMidtransTransaction retrieves Midtrans transaction from cache
func (cs *CacheService) GetMidtransTransaction(transactionID string, dest interface{}) error {
	key := fmt.Sprintf("midtrans:transaction:%s", transactionID)
	
	val, err := cs.client.Get(cs.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("Midtrans transaction not found in cache")
		}
		return fmt.Errorf("failed to get Midtrans transaction from cache: %w", err)
	}

	err = json.Unmarshal([]byte(val), dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Midtrans transaction data: %w", err)
	}

	return nil
}

// InvalidatePaymentCache invalidates all payment-related cache entries
func (cs *CacheService) InvalidatePaymentCache(paymentID, orderID, userID string) error {
	keys := []string{
		fmt.Sprintf("payment:%s", paymentID),
		fmt.Sprintf("payment:order:%s", orderID),
		fmt.Sprintf("user:payments:%s", userID),
	}

	for _, key := range keys {
		err := cs.client.Del(cs.ctx, key).Err()
		if err != nil {
			log.Printf("⚠️ Failed to delete cache key %s: %v", key, err)
		}
	}

	log.Printf("🗑️ Invalidated payment cache for payment: %s", paymentID)
	return nil
}

// HealthCheck checks if Redis connection is healthy
func (cs *CacheService) HealthCheck() error {
	_, err := cs.client.Ping(cs.ctx).Result()
	if err != nil {
		return fmt.Errorf("Redis health check failed: %w", err)
	}
	return nil
}

// Close closes the Redis connection
func (cs *CacheService) Close() error {
	return cs.client.Close()
}
