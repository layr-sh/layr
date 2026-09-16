package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrKVStoreKeyNotFound indicates the requested key does not exist or has expired.
var ErrKVStoreKeyNotFound = errors.New("key not found")

// KVStore represents the pluggable Key-Value and caching storage interface.
type KVStore interface {
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

const defaultDatabaseSweepInterval = 60 * time.Second

// NewKVStore initializes a KVStore driver based on configuration.
func NewKVStore(ctx context.Context, db *DatabasePool) (KVStore, error) {
	kvStoreConfig := GetConfig().KVStore
	backend := kvStoreConfig.Backend
	if backend == "" || backend == "database" {
		if db == nil {
			return nil, errors.New("database required for database kv backend")
		}
		return NewDatabaseKVStore(ctx, db, defaultDatabaseSweepInterval), nil
	}

	if backend == "redis" {
		return NewRedisKVStore(ctx)
	}

	return nil, fmt.Errorf("unsupported kv backend '%s'", backend)
}
