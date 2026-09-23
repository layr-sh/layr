package password

import (
	"errors"
	"strings"
	"testing"
)

type errReader struct{}

func (errReader) Read(buffer []byte) (int, error) {
	return 0, errors.New("entropy source failed")
}

func TestPasswordArgon2idHasherUnit(t *testing.T) {
	hasher := NewHasher()

	// 1. Password hashing with standard parameters
	password := "CorrectHorseBatteryStaple123!"
	passwordHash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !strings.HasPrefix(passwordHash, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected hash format: %s", passwordHash)
	}

	// 2. Verify correct password
	isPasswordValid, err := hasher.Verify(password, passwordHash)
	if err != nil || !isPasswordValid {
		t.Fatalf("expected password verification to succeed, got valid=%v, err=%v", isPasswordValid, err)
	}

	// 3. Verify wrong password
	isWrongPasswordValid, err := hasher.Verify("WrongPassword", passwordHash)
	if err != nil || isWrongPasswordValid {
		t.Fatalf("expected password verification to fail on wrong password, got valid=%v", isWrongPasswordValid)
	}

	// 4. Verify unicode and empty password
	unicodePassword := "🔒密码SuperSecretPassword🗝️"
	unicodeHash, err := hasher.Hash(unicodePassword)
	if err != nil {
		t.Fatalf("failed to hash unicode password: %v", err)
	}
	isUnicodeValid, err := hasher.Verify(unicodePassword, unicodeHash)
	if err != nil || !isUnicodeValid {
		t.Fatalf("expected unicode password to verify successfully")
	}

	// 5. Fault injection on random reader
	hasher.SetRandomReader(errReader{})
	if _, err := hasher.Hash("any-password"); err == nil {
		t.Fatal("expected entropy failure error")
	}
	hasher.SetRandomReader(nil)

	// 6. Malformed and corrupted hashes
	malformedTestCases := []string{
		"not-enough-segments",
		"$argon2i$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",          // wrong type
		"$argon2id$v=18$m=65536,t=3,p=4$c2FsdA$aGFzaA",         // wrong version
		"$argon2id$v=19$invalid_parameters$c2FsdA$aGFzaA",      // invalid parameters count
		"$argon2id$v=19$m,t,p$c2FsdA$aGFzaA",                   // invalid param kv format
		"$argon2id$v=19$m=bad,t=3,p=4$c2FsdA$aGFzaA",           // bad m
		"$argon2id$v=19$m=65536,t=bad,p=4$c2FsdA$aGFzaA",       // bad t
		"$argon2id$v=19$m=65536,t=3,p=bad$c2FsdA$aGFzaA",       // bad p
		"$argon2id$v=19$m=65536,t=3,p=4$!invalid-salt!$aGFzaA", // bad salt base64
		"$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$!invalid-hash!", // bad hash base64
	}

	for _, malformedHash := range malformedTestCases {
		t.Run(malformedHash, func(t *testing.T) {
			if isMatch, err := hasher.Verify("test", malformedHash); err == nil && isMatch {
				t.Fatalf("expected error on malformed hash: %s", malformedHash)
			}
		})
	}
}
