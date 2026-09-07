package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisKVStore implements KVStore over standalone or cluster Redis instances.
type RedisKVStore struct {
	client redis.UniversalClient
}

// NewRedisKVStore initializes a Redis KVStore driver.
func NewRedisKVStore(ctx context.Context) (*RedisKVStore, error) {
	kvConfig := GetConfig().KVStore
	var client redis.UniversalClient

	if len(kvConfig.ClusterURLs) > 0 {
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: kvConfig.ClusterURLs,
		})
	} else if kvConfig.URL != "" {
		redisOptions, err := redis.ParseURL(kvConfig.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redis url: %w", err)
		}
		client = redis.NewClient(redisOptions)
	} else {
		return nil, errors.New("redis url or cluster_urls is required")
	}

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisKVStore{client: client}, nil
}

// Get retrieves a string value by key. Returns ErrKVStoreKeyNotFound if missing.
func (redisKVStore *RedisKVStore) Get(ctx context.Context, key string) (string, error) {
	value, err := redisKVStore.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrKVStoreKeyNotFound
		}
		return "", fmt.Errorf("failed to get redis key '%s': %w", key, err)
	}
	return value, nil
}

// MGet retrieves multiple keys in a single operation.
func (redisKVStore *RedisKVStore) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string)
	if len(keys) == 0 {
		return result, nil
	}

	values, err := redisKVStore.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to mget redis keys: %w", err)
	}

	for i, value := range values {
		if stringValue, ok := value.(string); ok {
			result[keys[i]] = stringValue
		}
	}

	return result, nil
}

// Set stores a string value with expiry.
func (redisKVStore *RedisKVStore) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	err := redisKVStore.client.Set(ctx, key, value, expiry).Err()
	if err != nil {
		return fmt.Errorf("failed to set redis key '%s': %w", key, err)
	}
	return nil
}

// MSet stores multiple key-value pairs with the specified expiry.
func (redisKVStore *RedisKVStore) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	if len(entries) == 0 {
		return nil
	}

	pipe := redisKVStore.client.Pipeline()
	for key, value := range entries {
		pipe.Set(ctx, key, value, expiry)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mset redis pairs: %w", err)
	}
	return nil
}

// SetNX stores a value only if the key does not already exist in Redis.
func (redisKVStore *RedisKVStore) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	ok, err := redisKVStore.client.SetNX(ctx, key, value, expiry).Result()
	if err != nil {
		return false, fmt.Errorf("failed to setnx redis key '%s': %w", key, err)
	}
	return ok, nil
}

// Delete removes a key from Redis.
func (redisKVStore *RedisKVStore) Delete(ctx context.Context, key string) error {
	err := redisKVStore.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete redis key '%s': %w", key, err)
	}
	return nil
}

// Increment atomically increments an integer counter with the specified expiry.
func (redisKVStore *RedisKVStore) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	pipe := redisKVStore.client.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	if expiry > 0 {
		pipe.Expire(ctx, key, expiry)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to increment redis key '%s': %w", key, err)
	}

	return incrCmd.Val(), nil
}

// Expire updates the expiry on an existing Redis key.
func (redisKVStore *RedisKVStore) Expire(ctx context.Context, key string, expiry time.Duration) error {
	ok, err := redisKVStore.client.Expire(ctx, key, expiry).Result()
	if err != nil {
		return fmt.Errorf("failed to expire redis key '%s': %w", key, err)
	}
	if !ok {
		return ErrKVStoreKeyNotFound
	}
	return nil
}

// Ping checks Redis connectivity.
func (redisKVStore *RedisKVStore) Ping(ctx context.Context) error {
	if err := redisKVStore.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	return nil
}

// Close terminates Redis connections.
func (redisKVStore *RedisKVStore) Close() error {
	if err := redisKVStore.client.Close(); err != nil {
		return fmt.Errorf("failed to close redis client: %w", err)
	}
	return nil
}
