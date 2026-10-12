package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/acme/autocert"
)

func TestCoreDBCertCacheLifecycleUnit(t *testing.T) {
	kernel, cleanup := SetupTestKernel(t, nil)
	defer cleanup()

	ctx := context.Background()
	cryptoKeyManager := kernel.CryptoKeyManager()
	db := kernel.DB()

	var putCallbackKey string
	var putCallbackData []byte
	dbCertCache := NewDBCertCache(db, cryptoKeyManager)
	dbCertCache.AddCertPutCallback(func(key string, data []byte) {
		putCallbackKey = key
		putCallbackData = data
	})

	testKey := "api.example.com"
	testPlaintext := []byte("-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----")

	// 1. Get non-existent key returns autocert.ErrCacheMiss
	missingData, getErr := dbCertCache.Get(ctx, "nonexistent.domain.com")
	require.Error(t, getErr)
	require.True(t, errors.Is(getErr, autocert.ErrCacheMiss))
	require.Nil(t, missingData)

	// 2. Put stores certificate and triggers callback
	putErr := dbCertCache.Put(ctx, testKey, testPlaintext)
	require.NoError(t, putErr)
	require.Equal(t, testKey, putCallbackKey)
	require.Equal(t, testPlaintext, putCallbackData)

	// 3. Verify encryption at rest in PostgreSQL
	var rawEncryptedInDB []byte
	queryErr := db.QueryRow(ctx, "SELECT data FROM core.acme_certificates WHERE key_name = $1", testKey).Scan(&rawEncryptedInDB)
	require.NoError(t, queryErr)
	require.NotEqual(t, testPlaintext, rawEncryptedInDB)

	// 4. Get returns decrypted plaintext
	retrievedData, retrieveErr := dbCertCache.Get(ctx, testKey)
	require.NoError(t, retrieveErr)
	require.Equal(t, testPlaintext, retrievedData)

	// 5. Overwrite/update existing key
	updatedPlaintext := []byte("-----BEGIN CERTIFICATE-----\nUPDATED...\n-----END CERTIFICATE-----")
	require.NoError(t, dbCertCache.Put(ctx, testKey, updatedPlaintext))
	updatedData, retrieveUpdatedErr := dbCertCache.Get(ctx, testKey)
	require.NoError(t, retrieveUpdatedErr)
	require.Equal(t, updatedPlaintext, updatedData)

	// 6. Delete key removes it from database
	require.NoError(t, dbCertCache.Delete(ctx, testKey))

	// 7. Get after delete returns autocert.ErrCacheMiss
	afterDeleteData, afterDeleteErr := dbCertCache.Get(ctx, testKey)
	require.Error(t, afterDeleteErr)
	require.True(t, errors.Is(afterDeleteErr, autocert.ErrCacheMiss))
	require.Nil(t, afterDeleteData)

	// 8. Delete non-existent key is idempotent (no error)
	require.NoError(t, dbCertCache.Delete(ctx, testKey))

	// 9. Key name exceeding 255 chars succeeds
	longKey := strings.Repeat("a", 300) + ".example.com"
	require.NoError(t, dbCertCache.Put(ctx, longKey, testPlaintext))
	retrievedLongData, longGetErr := dbCertCache.Get(ctx, longKey)
	require.NoError(t, longGetErr)
	require.Equal(t, testPlaintext, retrievedLongData)
	require.NoError(t, dbCertCache.Delete(ctx, longKey))
}

func TestCoreDBCertCacheEdgeCasesUnit(t *testing.T) {
	kernel, cleanup := SetupTestKernel(t, nil)
	defer cleanup()

	ctx := context.Background()
	cryptoKeyManager := kernel.CryptoKeyManager()
	db := kernel.DB()
	dbCertCache := NewDBCertCache(db, cryptoKeyManager)

	// 1. Corrupted ciphertext in DB returns decryption error on Get
	corruptKey := "corrupted.example.com"
	_, insertCorruptErr := db.Exec(ctx, "INSERT INTO core.acme_certificates (key_name, data, updated_at) VALUES ($1, $2, clock_timestamp())", corruptKey, []byte("invalid-corrupted-ciphertext"))
	require.NoError(t, insertCorruptErr)

	corruptedData, corruptErr := dbCertCache.Get(ctx, corruptKey)
	require.Error(t, corruptErr)
	require.False(t, errors.Is(corruptErr, autocert.ErrCacheMiss))
	require.Contains(t, corruptErr.Error(), "failed to decrypt cached cert")
	require.Nil(t, corruptedData)

	// Clean up corrupt row
	_ = dbCertCache.Delete(ctx, corruptKey)

	// 2. Broken DB pool error paths
	brokenKernel := SetupTestKernelWithBrokenDB(t, nil)
	brokenDBCertCache := NewDBCertCache(brokenKernel.DB(), cryptoKeyManager)

	_, brokenGetErr := brokenDBCertCache.Get(ctx, "any.key")
	require.Error(t, brokenGetErr)
	require.Contains(t, brokenGetErr.Error(), "failed to get cert from cache")

	brokenPutErr := brokenDBCertCache.Put(ctx, "any.key", []byte("data"))
	require.Error(t, brokenPutErr)
	require.Contains(t, brokenPutErr.Error(), "failed to store cert in cache")

	brokenDeleteErr := brokenDBCertCache.Delete(ctx, "any.key")
	require.Error(t, brokenDeleteErr)
	require.Contains(t, brokenDeleteErr.Error(), "failed to delete cert from cache")

	// 3. Failing CryptoKeyManager randomReader returns encryption error on Put
	failingCryptoKeyManager, keyErr := NewCryptoKeyManager(TestMasterEncryptionKeyHex)
	require.NoError(t, keyErr)
	failingCryptoKeyManager.randomReader = iotest.ErrReader(errors.New("entropy error"))
	failingDBCertCache := NewDBCertCache(db, failingCryptoKeyManager)
	encryptErr := failingDBCertCache.Put(ctx, "any.key", []byte("data"))
	require.Error(t, encryptErr)
	require.Contains(t, encryptErr.Error(), "failed to encrypt cert")
}

func TestCoreDBCertCacheAddCallbackUnit(t *testing.T) {
	kernel, cleanup := SetupTestKernel(t, nil)
	defer cleanup()

	ctx := context.Background()
	cryptoKeyManager := kernel.CryptoKeyManager()
	db := kernel.DB()

	var cb1Called, cb2Called bool
	dbCertCache := NewDBCertCache(db, cryptoKeyManager)
	dbCertCache.AddCertPutCallback(nil)
	dbCertCache.AddCertPutCallback(func(key string, data []byte) {
		cb1Called = true
	})
	dbCertCache.AddCertPutCallback(func(key string, data []byte) {
		cb2Called = true
	})

	require.NoError(t, dbCertCache.Put(ctx, "test.domain.com", []byte("cert-data")))
	require.True(t, cb1Called)
	require.True(t, cb2Called)
}
