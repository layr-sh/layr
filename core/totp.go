package core

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

const (
	totpSecretByteLength  = 20
	totpCounterByteLength = 8
)

// TOTPManager generates and validates RFC 6238 Time-based One-Time Passwords.
type TOTPManager struct {
	issuer        string
	periodSeconds int64
	digits        int
	randomReader  io.Reader
}

// NewTOTPManager initializes a TOTP manager with default 30-second intervals and 6 digits.
func NewTOTPManager(issuer string) *TOTPManager {
	if issuer == "" {
		issuer = "Layr"
	}
	return &TOTPManager{
		issuer:        issuer,
		periodSeconds: 30,
		digits:        6,
		randomReader:  rand.Reader,
	}
}

// SetRandomReader sets the random reader (for testing).
func (totpManager *TOTPManager) SetRandomReader(reader io.Reader) {
	if reader == nil {
		totpManager.randomReader = rand.Reader
		return
	}
	totpManager.randomReader = reader
}

// GenerateSecret creates a 20-byte random base32 encoded TOTP secret without padding.
func (totpManager *TOTPManager) GenerateSecret() (string, error) {
	log.Debugf("generating random TOTP secret")
	secretBytes := make([]byte, totpSecretByteLength)
	if _, err := io.ReadFull(totpManager.randomReader, secretBytes); err != nil {
		return "", fmt.Errorf("failed to generate random TOTP secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes), nil
}

// GenerateCode calculates the 6-digit TOTP code for a secret at a specific time.
func (totpManager *TOTPManager) GenerateCode(secretBase32 string, atTime time.Time) (string, error) {
	log.Tracef("generating TOTP code at %s", atTime.Format(time.RFC3339))
	secretKey, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secretBase32)))
	if err != nil {
		return "", fmt.Errorf("invalid base32 secret: %w", err)
	}

	counter := uint64(atTime.Unix() / totpManager.periodSeconds)
	counterBytes := make([]byte, totpCounterByteLength)
	binary.BigEndian.PutUint64(counterBytes, counter)

	//nolint:namingclarity
	mac := hmac.New(sha1.New, secretKey)
	mac.Write(counterBytes)
	hash := mac.Sum(nil)

	offset := hash[len(hash)-1] & 0x0f
	codeValue := (binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff) % 1000000

	return fmt.Sprintf("%06d", codeValue), nil
}

// ValidateCode verifies a TOTP code within a window of [-skew, +skew] intervals.
func (totpManager *TOTPManager) ValidateCode(secretBase32 string, code string, atTime time.Time, skewSteps int) bool {
	log.Tracef("validating TOTP code at %s with skew %d", atTime.Format(time.RFC3339), skewSteps)
	code = strings.TrimSpace(code)
	if len(code) != totpManager.digits {
		return false
	}
	if skewSteps < 0 {
		skewSteps = 1
	}

	currentStep := atTime.Unix() / totpManager.periodSeconds
	for step := -skewSteps; step <= skewSteps; step++ {
		targetTime := time.Unix((currentStep+int64(step))*totpManager.periodSeconds, 0)
		expectedCode, err := totpManager.GenerateCode(secretBase32, targetTime)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(code), []byte(expectedCode)) == 1 {
			return true
		}
	}
	return false
}

// BuildAuthURL constructs a standard otpauth:// URL for authenticator apps.
func (totpManager *TOTPManager) BuildAuthURL(account, secretBase32 string) string {
	label := fmt.Sprintf("%s:%s", totpManager.issuer, account)
	values := url.Values{}
	values.Set("secret", secretBase32)
	values.Set("issuer", totpManager.issuer)
	values.Set("algorithm", "SHA1")
	values.Set("digits", fmt.Sprintf("%d", totpManager.digits))
	values.Set("period", fmt.Sprintf("%d", totpManager.periodSeconds))

	return fmt.Sprintf("otpauth://totp/%s?%s", url.PathEscape(label), values.Encode())
}
