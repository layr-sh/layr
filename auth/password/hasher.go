// Package password provides Argon2id password hashing and constant-time verification.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
	"layr.sh/logger"
)

var log = logger.New("auth")

const (
	expectedHashSegmentCount = 6
	expectedParameterCount   = 3
)

// Default Argon2id parameters per OWASP / Layr specification.
const (
	DefaultMemory      = 64 * 1024 // 64 MB (65536 KB)
	DefaultIterations  = 3
	DefaultParallelism = 4
	DefaultSaltLength  = 16
	DefaultKeyLength   = 32
)

// Hasher provides Argon2id password hashing and constant-time verification.
type Hasher struct {
	memory       uint32
	iterations   uint32
	parallelism  uint8
	saltLength   uint32
	keyLength    uint32
	randomReader io.Reader
}

// NewHasher initializes an Argon2id hasher with canonical parameters.
func NewHasher() *Hasher {
	log.Debugf("initializing Argon2id hasher")
	return &Hasher{
		memory:       DefaultMemory,
		iterations:   DefaultIterations,
		parallelism:  DefaultParallelism,
		saltLength:   DefaultSaltLength,
		keyLength:    DefaultKeyLength,
		randomReader: rand.Reader,
	}
}

// SetRandomReader sets the random reader (for testing).
func (hasher *Hasher) SetRandomReader(reader io.Reader) {
	if reader == nil {
		hasher.randomReader = rand.Reader
		return
	}
	hasher.randomReader = reader
}

// Hash generates an Argon2id PHC-formatted password hash.
// Output: $argon2id$v=19$m=65536,t=3,p=4$<b64salt>$<b64hash>
func (hasher *Hasher) Hash(plainPassword string) (string, error) {
	log.Debugf("hashing password with Argon2id")
	salt := make([]byte, hasher.saltLength)
	if _, err := io.ReadFull(hasher.randomReader, salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	hash := argon2.IDKey([]byte(plainPassword), salt, hasher.iterations, hasher.memory, hasher.parallelism, hasher.keyLength)

	saltBase64 := base64.RawStdEncoding.EncodeToString(salt)
	hashBase64 := base64.RawStdEncoding.EncodeToString(hash)

	log.Tracef("generated Argon2id password hash (memory=%d, iterations=%d, parallelism=%d)", hasher.memory, hasher.iterations, hasher.parallelism)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, hasher.memory, hasher.iterations, hasher.parallelism, saltBase64, hashBase64), nil
}

// Verify checks password against an Argon2id PHC-formatted hash string in constant time.
func (hasher *Hasher) Verify(plainPassword, encodedHash string) (bool, error) {
	log.Tracef("verifying password with Argon2id")
	hashSegments := strings.Split(encodedHash, "$")
	if len(hashSegments) != expectedHashSegmentCount {
		log.Debugf("password verification failed: invalid argon2id hash segment count (%d)", len(hashSegments))
		return false, errors.New("invalid argon2id hash format")
	}

	if hashSegments[1] != "argon2id" {
		log.Debugf("password verification failed: incompatible hash type %q", hashSegments[1])
		return false, fmt.Errorf("incompatible hash type: %s", hashSegments[1])
	}

	var version int
	if _, err := fmt.Sscanf(hashSegments[2], "v=%d", &version); err != nil || version != argon2.Version {
		log.Debugf("password verification failed: unsupported argon2 version")
		return false, errors.New("unsupported argon2 version")
	}

	parameterSegments := strings.Split(hashSegments[3], ",")
	if len(parameterSegments) != expectedParameterCount {
		log.Debugf("password verification failed: invalid argon2 parameters")
		return false, errors.New("invalid argon2 parameters")
	}

	var memory, iterations uint32
	var parallelism uint64

	for _, parameter := range parameterSegments {
		keyValue := strings.Split(parameter, "=")
		if len(keyValue) != 2 {
			return false, errors.New("invalid parameter format")
		}
		switch keyValue[0] {
		case "m":
			value, err := strconv.ParseUint(keyValue[1], 10, 32)
			if err != nil {
				return false, fmt.Errorf("invalid memory parameter: %w", err)
			}
			memory = uint32(value)
		case "t":
			value, err := strconv.ParseUint(keyValue[1], 10, 32)
			if err != nil {
				return false, fmt.Errorf("invalid iterations parameter: %w", err)
			}
			iterations = uint32(value)
		case "p":
			value, err := strconv.ParseUint(keyValue[1], 10, 8)
			if err != nil {
				return false, fmt.Errorf("invalid parallelism parameter: %w", err)
			}
			parallelism = value
		}
	}

	salt, err := base64.RawStdEncoding.DecodeString(hashSegments[4])
	if err != nil {
		return false, fmt.Errorf("invalid salt base64: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(hashSegments[5])
	if err != nil {
		return false, fmt.Errorf("invalid hash base64: %w", err)
	}

	computedHash := argon2.IDKey([]byte(plainPassword), salt, iterations, memory, uint8(parallelism), uint32(len(expectedHash)))

	matched := subtle.ConstantTimeCompare(computedHash, expectedHash) == 1
	log.Tracef("Argon2id password verification completed (match=%t)", matched)
	if matched {
		return true, nil
	}

	return false, nil
}
