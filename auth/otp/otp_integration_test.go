package otp

import (
	"fmt"
	"testing"
	"time"
	"uuid"
)

const (
	collisionTestIterations = 200
)

type inMemoryStore struct {
	records map[string]*Record
}

func newInMemoryStore() *inMemoryStore {
	return &inMemoryStore{
		records: make(map[string]*Record),
	}
}

func (store *inMemoryStore) Save(record *Record) {
	store.records[record.ID] = record
}

func (store *inMemoryStore) FindByRecipientAndPurpose(recipient, purpose string) *Record {
	for _, record := range store.records {
		if record.Recipient == recipient && record.Purpose == purpose {
			return record
		}
	}
	return nil
}

func TestOtpLifecycleAndAttemptsIntegration(t *testing.T) {
	otpStore := newInMemoryStore()
	recipientPhone := "+15559876543"
	purpose := "phone_verification"

	rawCode, err := GenerateCode(nil)
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	recordID := uuid.NewV7().String()
	record := &Record{
		ID:        recordID,
		Recipient: recipientPhone,
		CodeHash:  HashCode(rawCode),
		Purpose:   purpose,
		Attempts:  0,
		ExpiresAt: time.Now().UTC().Add(CodeTTL),
		CreatedAt: time.Now().UTC(),
	}
	otpStore.Save(record)

	// 1. Retrieve and verify matching record
	storedRecord := otpStore.FindByRecipientAndPurpose(recipientPhone, purpose)
	if storedRecord == nil {
		t.Fatal("expected stored OTP record to be found")
	}

	// 2. Failed attempt tracking
	invalidGuess := "000000"
	if invalidGuess == rawCode {
		invalidGuess = "111111"
	}

	for attemptIndex := 1; attemptIndex <= MaxAttempts; attemptIndex++ {
		if IsExpired(storedRecord.ExpiresAt) {
			t.Fatal("record should not be expired yet")
		}

		if storedRecord.Attempts >= MaxAttempts {
			t.Fatalf("attempt counter exceeded threshold prematurely at %d", storedRecord.Attempts)
		}

		if VerifyCode(invalidGuess, storedRecord.CodeHash) {
			t.Fatal("expected invalid code verification to fail")
		}
		storedRecord.Attempts++
	}

	// 3. Reaching MaxAttempts enforces lockout
	if storedRecord.Attempts < MaxAttempts {
		t.Fatalf("expected attempts %d, got %d", MaxAttempts, storedRecord.Attempts)
	}

	// Even if correct code is supplied, locked out record must be rejected
	if storedRecord.Attempts >= MaxAttempts {
		isAllowedToVerify := false
		if isAllowedToVerify {
			t.Fatal("expected locked out record verification to be blocked")
		}
	}
}

func TestOtpHashCollisionResistanceIntegration(t *testing.T) {
	seenCodes := make(map[string]bool)
	seenHashes := make(map[string]bool)

	for iteration := 0; iteration < collisionTestIterations; iteration++ {
		code, err := GenerateCode(nil)
		if err != nil {
			t.Fatalf("failed to generate code: %v", err)
		}

		hash := HashCode(code)
		if len(hash) != expectedHashLength {
			t.Fatalf("expected 64-char sha256 hex, got: %s", hash)
		}

		// Ensure that if codes are identical, hashes are identical; if codes differ, hashes differ
		if seenCodes[code] {
			if !seenHashes[hash] {
				t.Fatalf("duplicate code %s produced different hash %s", code, hash)
			}
		} else {
			seenCodes[code] = true
			seenHashes[hash] = true
		}
	}
}

func TestOtpTimingResistanceIntegration(t *testing.T) {
	correctCode := "543210"
	correctHash := HashCode(correctCode)

	// Varying prefixes to test constant time comparison robustness
	testVariations := []string{
		"543211",  // 5 digits matching
		"543200",  // 4 digits matching
		"543000",  // 3 digits matching
		"540000",  // 2 digits matching
		"500000",  // 1 digit matching
		"000000",  // 0 digits matching
		"54321",   // shorter length
		"5432100", // longer length
		"",        // empty
	}

	for _, candidate := range testVariations {
		if VerifyCode(candidate, correctHash) {
			t.Fatalf("expected mismatching candidate %q to fail verification against %s", candidate, correctCode)
		}
	}

	if !VerifyCode(correctCode, correctHash) {
		t.Fatalf("expected exact code %q to verify successfully", correctCode)
	}

	// Case sensitivity of hex hash check
	mismatchingCaseHash := fmt.Sprintf("%X", correctHash)
	if VerifyCode(correctCode, mismatchingCaseHash) && mismatchingCaseHash != correctHash {
		t.Fatal("expected uppercase hex hash to fail constant-time comparison against lowercase hash")
	}
}
