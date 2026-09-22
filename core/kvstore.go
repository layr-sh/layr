package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrKVStoreKeyNotFound indicates the requested key does not exist or has expired.
var ErrKVStoreKeyNotFound = errors.New("key not found")

// KVDriver represents the pluggable Key-Value and caching storage driver interface.
type KVDriver interface {
	Get(ctx context.Context, key string) (string, error)
	MGet(ctx context.Context, keys []string) (map[string]string, error)
	Set(ctx context.Context, key string, value string, expiry time.Duration) error
	MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error
	SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error)
	Delete(ctx context.Context, key string) error
	Increment(ctx context.Context, key string, expiry time.Duration) (int64, error)
	IncrementBy(ctx context.Context, key string, delta int64, expiry time.Duration) (int64, error)
	Expire(ctx context.Context, key string, expiry time.Duration) error
	Ping(ctx context.Context) error
	Close() error
}

// Sweeper is an optional interface implemented by drivers that support periodic key cleanup.
type Sweeper interface {
	Sweep(ctx context.Context) (int64, error)
}

// KVStore represents the Key-Value storage manager struct.
type KVStore struct {
	kvDriver KVDriver
}

const defaultDatabaseSweepInterval = 60 * time.Second

// NewKVStore initializes a KVStore driver based on configuration and wraps it in a *KVStore struct.
func NewKVStore(ctx context.Context, db *DatabasePool) (*KVStore, error) {
	kvStoreConfig := GetConfig().KVStore
	backend := kvStoreConfig.Backend
	if backend == "" || backend == "database" {
		return NewDatabaseKVStore(ctx, db, defaultDatabaseSweepInterval), nil
	}

	if backend == "redis" {
		redisKVStore, err := NewRedisKVStore(ctx)
		if err != nil {
			return nil, err
		}
		return &KVStore{kvDriver: redisKVStore}, nil
	}

	return nil, fmt.Errorf("unsupported kv backend '%s'", backend)
}

// NewKVStoreFromDriver wraps an existing KVDriver into a *KVStore struct.
func NewKVStoreFromDriver(kvDriver KVDriver) *KVStore {
	return &KVStore{kvDriver: kvDriver}
}

// Driver returns the underlying KVDriver.
func (store *KVStore) Driver() KVDriver {
	return store.kvDriver
}

// KVStore returns the KVStore itself to allow seamless usage when a *KVStore is referenced.
func (store *KVStore) KVStore() *KVStore {
	return store
}

// Get retrieves a string value by key.
func (store *KVStore) Get(ctx context.Context, key string) (string, error) {
	retrievedValue, err := store.kvDriver.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return retrievedValue, nil
}

// MGet retrieves multiple string values by keys in batch.
func (store *KVStore) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	entries, err := store.kvDriver.MGet(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return entries, nil
}

// Set stores a key-value pair with an expiration duration.
func (store *KVStore) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	if err := store.kvDriver.Set(ctx, key, value, expiry); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// MSet stores multiple key-value pairs in batch with an expiration duration.
func (store *KVStore) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	if err := store.kvDriver.MSet(ctx, entries, expiry); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// SetNX stores a key-value pair only if the key does not already exist.
func (store *KVStore) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	wasCreated, err := store.kvDriver.SetNX(ctx, key, value, expiry)
	if err != nil {
		return false, fmt.Errorf("%w", err)
	}
	return wasCreated, nil
}

// Delete removes a key and its value from the store.
func (store *KVStore) Delete(ctx context.Context, key string) error {
	if err := store.kvDriver.Delete(ctx, key); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// Increment atomicity increments integer value by 1.
func (store *KVStore) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	incrementResult, err := store.kvDriver.Increment(ctx, key, expiry)
	if err != nil {
		return 0, fmt.Errorf("%w", err)
	}
	return incrementResult, nil
}

// IncrementBy atomicity increments integer value by delta.
func (store *KVStore) IncrementBy(ctx context.Context, key string, delta int64, expiry time.Duration) (int64, error) {
	incrementResult, err := store.kvDriver.IncrementBy(ctx, key, delta, expiry)
	if err != nil {
		return 0, fmt.Errorf("%w", err)
	}
	return incrementResult, nil
}

// Expire sets or updates TTL on an existing key.
func (store *KVStore) Expire(ctx context.Context, key string, expiry time.Duration) error {
	if err := store.kvDriver.Expire(ctx, key, expiry); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// Ping checks if the storage backend is reachable and responsive.
func (store *KVStore) Ping(ctx context.Context) error {
	if err := store.kvDriver.Ping(ctx); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// Close gracefully closes the storage driver connections and worker loops.
func (store *KVStore) Close() error {
	if err := store.kvDriver.Close(); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// Sweep triggers driver expiration cleanup if the driver implements Sweeper.
func (store *KVStore) Sweep(ctx context.Context) (int64, error) {
	if sweeper, ok := store.kvDriver.(Sweeper); ok {
		sweptCount, err := sweeper.Sweep(ctx)
		if err != nil {
			return 0, fmt.Errorf("%w", err)
		}
		return sweptCount, nil
	}
	return 0, nil
}
