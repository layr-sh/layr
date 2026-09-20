package filestorage

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"layr.sh/core"
)

const testHexMasterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestFilestorageS3ConstructorUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testHexMasterKey)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	s3Engine := NewS3Engine(cryptoKeyManager)
	if s3Engine == nil {
		t.Fatal("expected non-nil s3 engine")
	}
}

func TestFilestorageS3MissingBackendConfigUnit(t *testing.T) {
	s3Engine := NewS3Engine(nil)
	ctx := context.Background()
	testBucket := Bucket{
		ID:            uuid.NewV7(),
		Name:          "s3-bucket",
		Backend:       "s3",
		BackendConfig: nil,
	}

	_, uploadErr := s3Engine.Upload(ctx, testBucket, "file.txt", strings.NewReader("data"), 4, "text/plain")
	if uploadErr == nil || !strings.Contains(uploadErr.Error(), "missing backend_config") {
		t.Fatalf("expected missing backend_config error, got: %v", uploadErr)
	}

	_, headErr := s3Engine.Head(ctx, testBucket, "file.txt")
	if headErr == nil || !strings.Contains(headErr.Error(), "missing backend_config") {
		t.Fatalf("expected missing backend_config error, got: %v", headErr)
	}

	deleteErr := s3Engine.Delete(ctx, testBucket, "file.txt")
	if deleteErr == nil || !strings.Contains(deleteErr.Error(), "missing backend_config") {
		t.Fatalf("expected missing backend_config error, got: %v", deleteErr)
	}

	_, _, downloadErr := s3Engine.Download(ctx, testBucket, "file.txt", nil)
	if downloadErr == nil || !strings.Contains(downloadErr.Error(), "missing backend_config") {
		t.Fatalf("expected missing backend_config error, got: %v", downloadErr)
	}
}

func TestFilestorageS3EncryptedSecretHandlingUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testHexMasterKey)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	plainSecret := "my-aws-secret-access-key-12345"
	encryptedSecret, err := cryptoKeyManager.EncryptField([]byte(plainSecret))
	if err != nil {
		t.Fatalf("failed to encrypt secret: %v", err)
	}

	cryptoS3Engine := NewS3Engine(cryptoKeyManager)
	testBucket := Bucket{
		ID:      uuid.NewV7(),
		Name:    "s3-bucket",
		Backend: "s3",
		BackendConfig: map[string]any{
			"endpoint":          "http://localhost:4566",
			"bucket":            "test-bucket",
			"region":            "us-east-1",
			"access_key_id":     "TESTKEY",
			"secret_access_key": encryptedSecret,
		},
	}

	s3UpstreamConfiguration, parseErr := cryptoS3Engine.parseUpstreamConfiguration(testBucket)
	if parseErr != nil {
		t.Fatalf("failed to parse upstream config: %v", parseErr)
	}
	if s3UpstreamConfiguration.SecretAccessKey != plainSecret {
		t.Fatalf("expected decrypted secret %q, got %q", plainSecret, s3UpstreamConfiguration.SecretAccessKey)
	}

	// Engine without crypto key manager fails on encrypted secret
	plainS3Engine := NewS3Engine(nil)
	_, parseFailErr := plainS3Engine.parseUpstreamConfiguration(testBucket)
	if parseFailErr == nil || !strings.Contains(parseFailErr.Error(), "crypto key manager unavailable") {
		t.Fatalf("expected crypto key manager unavailable error, got: %v", parseFailErr)
	}

	// Invalid encrypted secret fails to decrypt
	corruptBucket := Bucket{
		ID:      uuid.NewV7(),
		Name:    "corrupt-bucket",
		Backend: "s3",
		BackendConfig: map[string]any{
			"secret_access_key": "enc:v1:aes256gcm:not-valid-hex-or-b64",
		},
	}
	_, decryptErr := cryptoS3Engine.parseUpstreamConfiguration(corruptBucket)
	if decryptErr == nil || !strings.Contains(decryptErr.Error(), "failed to decrypt s3 secret_access_key") {
		t.Fatalf("expected failed to decrypt error, got: %v", decryptErr)
	}

	// Defaults fallback for bucket name and region
	defaultsBucket := Bucket{
		ID:      uuid.NewV7(),
		Name:    "fallback-bucket",
		Backend: "s3",
		BackendConfig: map[string]any{
			"access_key_id":     "TESTKEY",
			"secret_access_key": "raw-secret",
		},
	}
	defaultsS3UpstreamConfiguration, defaultsErr := cryptoS3Engine.parseUpstreamConfiguration(defaultsBucket)
	if defaultsErr != nil {
		t.Fatalf("failed to parse fallback bucket config: %v", defaultsErr)
	}
	if defaultsS3UpstreamConfiguration.BucketName != "fallback-bucket" {
		t.Fatalf("expected fallback bucket name 'fallback-bucket', got %q", defaultsS3UpstreamConfiguration.BucketName)
	}
	if defaultsS3UpstreamConfiguration.Region != "us-east-1" {
		t.Fatalf("expected fallback region 'us-east-1', got %q", defaultsS3UpstreamConfiguration.Region)
	}
}
