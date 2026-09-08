package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// DatabaseKVStore implements KVStore over PostgreSQL UNLOGGED table core.kv_store.
type DatabaseKVStore struct {
	db            *DatabasePool
	sweepInterval time.Duration
	stopChannel   chan struct{}
	waitGroup     sync.WaitGroup
	mutex         sync.Mutex
	closed        bool
}

const (
	defaultSweepInterval = 60 * time.Second
	defaultSweepTimeout  = 10 * time.Second
	defaultKeyExpiry     = 24 * time.Hour
)

// NewDatabaseKVStore creates and starts a PostgreSQL-backed KVStore.
func NewDatabaseKVStore(ctx context.Context, db *DatabasePool, sweepInterval time.Duration) *DatabaseKVStore {
	if sweepInterval <= 0 {
		sweepInterval = defaultSweepInterval
	}

	databaseKVStore := &DatabaseKVStore{
		db:            db,
		sweepInterval: sweepInterval,
		stopChannel:   make(chan struct{}),
	}

	log.Debugf("initializing DatabaseKVStore")
	databaseKVStore.waitGroup.Add(1)
	go databaseKVStore.sweepLoop(ctx)

	return databaseKVStore
}

func (databaseKVStore *DatabaseKVStore) sweepLoop(ctx context.Context) {
	defer databaseKVStore.waitGroup.Done()
	ticker := time.NewTicker(databaseKVStore.sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-databaseKVStore.stopChannel:
			return
		case <-ticker.C:
			detachedContext := context.WithoutCancel(ctx)
			sweepContext, cancel := context.WithTimeout(detachedContext, defaultSweepTimeout)
			_, _ = databaseKVStore.Sweep(sweepContext)
			cancel()
		}
	}
}

// Sweep prunes all expired cache records from PostgreSQL.
func (databaseKVStore *DatabaseKVStore) Sweep(ctx context.Context) (int64, error) {
	log.Tracef("DatabaseKVStore.Sweep executing")
	commandTag, err := databaseKVStore.db.Exec(ctx, "DELETE FROM core.kv_store WHERE expires_at <= clock_timestamp()")
	if err != nil {
		return 0, fmt.Errorf("failed to sweep expired kv records: %w", err)
	}
	return commandTag.RowsAffected(), nil
}

// Get retrieves a value by key. Returns ErrKVStoreKeyNotFound if missing or expired.
func (databaseKVStore *DatabaseKVStore) Get(ctx context.Context, key string) (string, error) {
	log.Tracef("DatabaseKVStore.Get key %s", key)
	var value []byte
	err := databaseKVStore.db.QueryRow(ctx, `
		SELECT value FROM core.kv_store
		WHERE key = $1 AND expires_at > clock_timestamp()
	`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrKVStoreKeyNotFound
		}
		return "", fmt.Errorf("failed to get kv key '%s': %w", key, err)
	}

	return string(value), nil
}

// MGet retrieves multiple keys in a single query.
func (databaseKVStore *DatabaseKVStore) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string)
	if len(keys) == 0 {
		return result, nil
	}

	rows, err := databaseKVStore.db.Query(ctx, `
		SELECT key, value FROM core.kv_store
		WHERE key = ANY($1) AND expires_at > clock_timestamp()
	`, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to mget kv keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rowKey string
		var rowValue []byte
		_ = rows.Scan(&rowKey, &rowValue)
		result[rowKey] = string(rowValue)
	}

	return result, nil
}

// Set unconditionally stores or updates a value with the specified expiry (matching Redis SET key value EX ttl upsert semantics).
// For conditional insertion only when key is absent/expired, use SetNX.
func (databaseKVStore *DatabaseKVStore) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	log.Tracef("DatabaseKVStore.Set key %s (expiry: %v)", key, expiry)
	if expiry <= 0 {
		expiry = defaultKeyExpiry
	}

	intervalLiteral := fmt.Sprintf("%d microseconds", expiry.Microseconds())
	query := `
		INSERT INTO core.kv_store (key, value, expires_at)
		VALUES ($1, $2, clock_timestamp() + $3::interval)
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			expires_at = EXCLUDED.expires_at
	`
	_, err := databaseKVStore.db.Exec(ctx, query, key, []byte(value), intervalLiteral)
	if err != nil {
		return fmt.Errorf("failed to set kv key '%s': %w", key, err)
	}

	return nil
}

// MSet stores multiple key-value pairs with the specified expiry.
func (databaseKVStore *DatabaseKVStore) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	if len(entries) == 0 {
		return nil
	}
	if expiry <= 0 {
		expiry = defaultKeyExpiry
	}

	intervalLiteral := fmt.Sprintf("%d microseconds", expiry.Microseconds())
	keys := make([]string, 0, len(entries))
	values := make([][]byte, 0, len(entries))
	for key, value := range entries {
		keys = append(keys, key)
		values = append(values, []byte(value))
	}

	query := `
		INSERT INTO core.kv_store (key, value, expires_at)
		SELECT k, v, clock_timestamp() + $3::interval
		FROM unnest($1::text[], $2::bytea[]) AS t(k, v)
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			expires_at = EXCLUDED.expires_at
	`
	_, err := databaseKVStore.db.Exec(ctx, query, keys, values, intervalLiteral)
	if err != nil {
		return fmt.Errorf("failed to mset kv pairs: %w", err)
	}

	return nil
}

// SetNX stores a value only if the key does not exist or has expired.
func (databaseKVStore *DatabaseKVStore) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	log.Tracef("DatabaseKVStore.SetNX key %s (expiry: %v)", key, expiry)
	if expiry <= 0 {
		expiry = defaultKeyExpiry
	}

	intervalLiteral := fmt.Sprintf("%d microseconds", expiry.Microseconds())
	query := `
		INSERT INTO core.kv_store (key, value, expires_at)
		VALUES ($1, $2, clock_timestamp() + $3::interval)
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			expires_at = EXCLUDED.expires_at
		WHERE core.kv_store.expires_at <= clock_timestamp()
	`
	commandTag, err := databaseKVStore.db.Exec(ctx, query, key, []byte(value), intervalLiteral)
	if err != nil {
		return false, fmt.Errorf("failed to setnx kv key '%s': %w", key, err)
	}

	return commandTag.RowsAffected() > 0, nil
}

// Delete removes a key from cache.
func (databaseKVStore *DatabaseKVStore) Delete(ctx context.Context, key string) error {
	log.Tracef("DatabaseKVStore.Delete key %s", key)
	_, err := databaseKVStore.db.Exec(ctx, "DELETE FROM core.kv_store WHERE key = $1", key)
	if err != nil {
		return fmt.Errorf("failed to delete kv key '%s': %w", key, err)
	}
	return nil
}

// Increment atomically increments an integer counter with the specified expiry.
func (databaseKVStore *DatabaseKVStore) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	log.Tracef("DatabaseKVStore.Increment key %s (expiry: %v)", key, expiry)
	if expiry <= 0 {
		expiry = defaultKeyExpiry
	}

	intervalLiteral := fmt.Sprintf("%d microseconds", expiry.Microseconds())
	query := `
		INSERT INTO core.kv_store (key, value, expires_at)
		VALUES ($1, '1'::bytea, clock_timestamp() + $2::interval)
		ON CONFLICT (key) DO UPDATE SET
			value = (CASE
				WHEN core.kv_store.expires_at > clock_timestamp()
				THEN (convert_from(core.kv_store.value, 'UTF8')::bigint + 1)::text::bytea
				ELSE '1'::bytea
			END),
			expires_at = clock_timestamp() + $2::interval
		RETURNING convert_from(value, 'UTF8')::bigint
	`
	var count int64
	err := databaseKVStore.db.QueryRow(ctx, query, key, intervalLiteral).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to increment kv key '%s': %w", key, err)
	}

	return count, nil
}

// Expire updates the expiry on an existing key.
func (databaseKVStore *DatabaseKVStore) Expire(ctx context.Context, key string, expiry time.Duration) error {
	log.Tracef("DatabaseKVStore.Expire key %s (expiry: %v)", key, expiry)
	if expiry <= 0 {
		expiry = defaultKeyExpiry
	}

	intervalLiteral := fmt.Sprintf("%d microseconds", expiry.Microseconds())
	query := `
		UPDATE core.kv_store
		SET expires_at = clock_timestamp() + $2::interval
		WHERE key = $1 AND expires_at > clock_timestamp()
	`
	commandTag, err := databaseKVStore.db.Exec(ctx, query, key, intervalLiteral)
	if err != nil {
		return fmt.Errorf("failed to expire kv key '%s': %w", key, err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrKVStoreKeyNotFound
	}

	return nil
}

// Ping verifies database health.
func (databaseKVStore *DatabaseKVStore) Ping(ctx context.Context) error {
	log.Trace("DatabaseKVStore.Ping")
	return databaseKVStore.db.Ping(ctx)
}

// Close stops the background expiration sweeper.
func (databaseKVStore *DatabaseKVStore) Close() error {
	log.Debug("DatabaseKVStore.Close")
	databaseKVStore.mutex.Lock()
	if databaseKVStore.closed {
		databaseKVStore.mutex.Unlock()
		return nil
	}
	databaseKVStore.closed = true
	close(databaseKVStore.stopChannel)
	databaseKVStore.mutex.Unlock()

	databaseKVStore.waitGroup.Wait()
	return nil
}
