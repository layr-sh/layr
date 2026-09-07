package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"golang.org/x/crypto/hkdf"
)

// Standard HKDF Subkey Contexts
const (
	ContextDBSecrets      = "layr:db:envelope:aes256gcm:v1"
	ContextJWTSigning     = "layr:auth:jwt:ed25519:v1"
	ContextConsoleSalt    = "layr:console:user:salt:v1"
	ContextIPCHMAC        = "layr:core:ipc:hmacsha256:v1"
	ContextStorageEnc     = "layr:storage:chunk:aes256gcm:v1"
	ContextPublishableKey = "layr:client:publishable:v1"
)

const (
	encryptionKeyHexLength  = 64
	encryptionKeyByteLength = 32
	subkeyByteLength        = 32
	envelopeComponentCount  = 6
)

// CryptoKeyManager handles cryptographic key derivation and envelope encryption.
type CryptoKeyManager struct {
	encryptionKey []byte
	randomReader  io.Reader // injectable for testing; defaults to crypto/rand.Reader
}

// NewCryptoKeyManager parses a 32-byte hex/base64 master key.
func NewCryptoKeyManager(encryptionKeyHex string) (*CryptoKeyManager, error) {
	trimmedHex := strings.TrimSpace(encryptionKeyHex)
	var decodedKey []byte
	var err error

	if len(trimmedHex) == encryptionKeyHexLength {
		decodedKey, err = hex.DecodeString(trimmedHex)
	} else {
		decodedKey, err = base64.StdEncoding.DecodeString(trimmedHex)
		if err != nil {
			decodedKey, err = base64.RawStdEncoding.DecodeString(trimmedHex)
		}
	}

	if err != nil || len(decodedKey) != encryptionKeyByteLength {
		return nil, fmt.Errorf("master_encryption_key must be a valid 32-byte hex or base64 encoded string: %w", err)
	}

	return &CryptoKeyManager{encryptionKey: decodedKey, randomReader: rand.Reader}, nil
}

// deriveSubkey derives a deterministic 32-byte subkey using HKDF-SHA256.
// HKDF-SHA256 with a valid 32-byte key cannot fail, so this is infallible.
func (cryptoKeyManager *CryptoKeyManager) deriveSubkey(derivationContext string) []byte {
	hkdfReader := hkdf.New(sha256.New, cryptoKeyManager.encryptionKey, nil, []byte(derivationContext))
	subkey := make([]byte, subkeyByteLength)
	_, _ = io.ReadFull(hkdfReader, subkey) // HKDF with valid key always succeeds
	return subkey
}

// DeriveSubkey is the public API for subkey derivation (returns error for interface compatibility).
func (cryptoKeyManager *CryptoKeyManager) DeriveSubkey(derivationContext string) ([]byte, error) {
	return cryptoKeyManager.deriveSubkey(derivationContext), nil
}

// EncryptField encrypts plaintext using the DB secrets subkey in AES-256-GCM.
// Output format: enc:v1:aes256gcm:<base64-iv>:<base64-ciphertext>:<base64-tag>
func (cryptoKeyManager *CryptoKeyManager) EncryptField(plaintext []byte) (string, error) {
	subkey := cryptoKeyManager.deriveSubkey(ContextDBSecrets)

	// AES-256 with exactly 32-byte key and GCM with valid AES block cannot fail
	block, _ := aes.NewCipher(subkey)
	gcm, _ := cipher.NewGCM(block)

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(cryptoKeyManager.randomReader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	tagSize := gcm.Overhead()

	ciphertext := sealed[:len(sealed)-tagSize]
	authTag := sealed[len(sealed)-tagSize:]

	nonceBase64 := base64.RawURLEncoding.EncodeToString(nonce)
	ciphertextBase64 := base64.RawURLEncoding.EncodeToString(ciphertext)
	tagBase64 := base64.RawURLEncoding.EncodeToString(authTag)

	return fmt.Sprintf("enc:v1:aes256gcm:%s:%s:%s", nonceBase64, ciphertextBase64, tagBase64), nil
}

// DecryptField decrypts an envelope ciphertext.
func (cryptoKeyManager *CryptoKeyManager) DecryptField(encrypted string) ([]byte, error) {
	if !strings.HasPrefix(encrypted, "enc:v1:aes256gcm:") {
		return nil, errors.New("invalid envelope encryption prefix")
	}

	parts := strings.Split(encrypted, ":")
	if len(parts) != envelopeComponentCount {
		return nil, errors.New("invalid envelope format, expected 6 components")
	}

	nonce, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, fmt.Errorf("invalid nonce base64: %w", err)
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext base64: %w", err)
	}

	authTag, err := base64.RawURLEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, fmt.Errorf("invalid tag base64: %w", err)
	}

	subkey := cryptoKeyManager.deriveSubkey(ContextDBSecrets)
	block, _ := aes.NewCipher(subkey)
	gcm, _ := cipher.NewGCM(block)

	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}

	sealed := append(ciphertext, authTag...)
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate and decrypt ciphertext: %w", err)
	}

	return plaintext, nil
}

// DerivePublishableKey returns the deterministic client publishable key.
func (cryptoKeyManager *CryptoKeyManager) DerivePublishableKey() string {
	subkey := cryptoKeyManager.deriveSubkey(ContextPublishableKey)
	return hex.EncodeToString(subkey)
}

// VerifyPublishableKey checks if the presented key matches the derived publishable key.
func (cryptoKeyManager *CryptoKeyManager) VerifyPublishableKey(presentedKey string) bool {
	presentedKey = strings.TrimSpace(presentedKey)
	if presentedKey == "" {
		return false
	}
	expected := cryptoKeyManager.DerivePublishableKey()
	return subtle.ConstantTimeCompare([]byte(presentedKey), []byte(expected)) == 1
}

var (
	randomRWMutex sync.RWMutex
	randomReader  io.Reader = rand.Reader
)

// SetCryptoRandomReader sets the package-level random reader for testing.
func SetCryptoRandomReader(reader io.Reader) {
	randomRWMutex.Lock()
	defer randomRWMutex.Unlock()
	if reader == nil {
		randomReader = rand.Reader
		return
	}
	randomReader = reader
}

func getCryptoRandomReader() io.Reader {
	randomRWMutex.RLock()
	defer randomRWMutex.RUnlock()
	return randomReader
}

// GenerateRandomCryptoEncryptionKeyHex generates a cryptographically secure 32-byte hex key.
func GenerateRandomCryptoEncryptionKeyHex() (string, error) {
	return GenerateRandomCryptoEncryptionKeyHexFromReader(getCryptoRandomReader())
}

// GenerateRandomCryptoEncryptionKeyHexFromReader generates a 32-byte hex key using the provided reader.
func GenerateRandomCryptoEncryptionKeyHexFromReader(reader io.Reader) (string, error) {
	if reader == nil {
		reader = rand.Reader
	}
	randomBytes := make([]byte, encryptionKeyByteLength)
	if _, err := io.ReadFull(reader, randomBytes); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return hex.EncodeToString(randomBytes), nil
}
