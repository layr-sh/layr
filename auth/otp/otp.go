// Package otp provides secure One-Time Password generation, hashing, and verification.
package otp

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"
)

// CodeTTL is the default time-to-live for an OTP code (15 minutes).
const CodeTTL = 15 * time.Minute

// MaxAttempts is the maximum verification attempts before code invalidation.
const MaxAttempts = 5

// MaxCodeLimit represents the upper bound for 6-digit numeric OTP generation (000000 - 999999).
const MaxCodeLimit = 1000000

// Record represents an OTP token stored in database.
type Record struct {
	ID        string    `json:"id"`
	Recipient string    `json:"recipient"`
	CodeHash  string    `json:"code_hash"`
	Purpose   string    `json:"purpose"`
	Attempts  int       `json:"attempts"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// GenerateCode creates a random 6-digit numeric string (e.g. "482910").
func GenerateCode(randomReader io.Reader) (string, error) {
	log.Trace("generating random numeric OTP code")
	if randomReader == nil {
		randomReader = rand.Reader
	}

	maxLimit := big.NewInt(MaxCodeLimit)
	numericValue, err := rand.Int(randomReader, maxLimit)
	if err != nil {
		log.Debugf("failed to generate random OTP code: %v", err)
		return "", fmt.Errorf("failed to generate random OTP: %w", err)
	}

	code := fmt.Sprintf("%06d", numericValue.Int64())
	log.Debug("successfully generated 6-digit random OTP code")
	return code, nil
}

// HashCode computes a SHA-256 hex digest for storage.
func HashCode(code string) string {
	log.Trace("computing SHA-256 hash digest for OTP code")
	code = strings.TrimSpace(code)
	hash := sha256.Sum256([]byte(code))
	return fmt.Sprintf("%x", hash)
}

// VerifyCode performs constant-time comparison between raw input code and stored code hash.
func VerifyCode(inputCode, storedHash string) bool {
	log.Debug("verifying OTP code against stored hash")
	inputHash := HashCode(inputCode)
	match := subtle.ConstantTimeCompare([]byte(inputHash), []byte(storedHash)) == 1
	if !match {
		log.Debug("OTP verification failed: code hash mismatch")
	} else {
		log.Debug("OTP verification succeeded: code hash matched")
	}
	return match
}

// IsExpired checks whether an OTP code has expired.
func IsExpired(expiresAt time.Time) bool {
	expired := time.Now().UTC().After(expiresAt)
	if expired {
		log.Debugf("OTP code expired (expires_at: %s, now: %s)", expiresAt.Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	} else {
		log.Tracef("OTP code is active (expires_at: %s)", expiresAt.Format(time.RFC3339))
	}
	return expired
}
