// Package core provides configuration, networking, and security foundations for Layr.
package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/acme/autocert"
)

// DBCertCache implements autocert.Cache by storing encrypted certificates in PostgreSQL core.acme_certificates.
type DBCertCache struct {
	db                *DatabasePool
	cryptoKeyManager  *CryptoKeyManager
	onCertPutCallback func(key string, certData []byte)
}

// NewDBCertCache initializes a database-backed autocert.Cache with encryption at rest.
func NewDBCertCache(db *DatabasePool, cryptoKeyManager *CryptoKeyManager) *DBCertCache {
	return &DBCertCache{
		db:               db,
		cryptoKeyManager: cryptoKeyManager,
	}
}

// AddCertPutCallback registers an additional hook triggered whenever a certificate is stored or updated.
func (certCache *DBCertCache) AddCertPutCallback(callback func(key string, certData []byte)) {
	if callback == nil {
		return
	}
	prevCallback := certCache.onCertPutCallback
	if prevCallback == nil {
		certCache.onCertPutCallback = callback
	} else {
		certCache.onCertPutCallback = func(key string, certData []byte) {
			prevCallback(key, certData)
			callback(key, certData)
		}
	}
}

// Get retrieves and decrypts the certificate data associated with the given key.
// Returns autocert.ErrCacheMiss if the key does not exist.
func (certCache *DBCertCache) Get(ctx context.Context, key string) ([]byte, error) {
	var encryptedData []byte
	const query = `SELECT data FROM core.acme_certificates WHERE key_name = $1`
	err := certCache.db.QueryRow(ctx, query, key).Scan(&encryptedData)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, autocert.ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to get cert from cache for key %q: %w", key, err)
	}

	decryptedData, decErr := certCache.cryptoKeyManager.DecryptField(string(encryptedData))
	if decErr != nil {
		return nil, fmt.Errorf("failed to decrypt cached cert for key %q: %w", key, decErr)
	}

	return decryptedData, nil
}

// Put encrypts and writes the certificate data into core.acme_certificates.
func (certCache *DBCertCache) Put(ctx context.Context, key string, data []byte) error {
	encryptedEnvelope, encErr := certCache.cryptoKeyManager.EncryptField(data)
	if encErr != nil {
		return fmt.Errorf("failed to encrypt cert for key %q: %w", key, encErr)
	}

	const upsertQuery = `
		INSERT INTO core.acme_certificates (key_name, data, updated_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (key_name) DO UPDATE SET data = EXCLUDED.data, updated_at = clock_timestamp()
	`
	if _, err := certCache.db.Exec(ctx, upsertQuery, key, []byte(encryptedEnvelope)); err != nil {
		return fmt.Errorf("failed to store cert in cache for key %q: %w", key, err)
	}

	if certCache.onCertPutCallback != nil {
		certCache.onCertPutCallback(key, data)
	}

	return nil
}

// Delete removes the certificate associated with the given key from core.acme_certificates.
func (certCache *DBCertCache) Delete(ctx context.Context, key string) error {
	const deleteQuery = `DELETE FROM core.acme_certificates WHERE key_name = $1`
	if _, err := certCache.db.Exec(ctx, deleteQuery, key); err != nil {
		return fmt.Errorf("failed to delete cert from cache for key %q: %w", key, err)
	}
	return nil
}
