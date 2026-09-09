package otp

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	expectedCodeLength = 6
	expectedHashLength = 64
	entropyFailureSize = 32
	fiveMinutesOffset  = 5 * time.Minute
)

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("entropy error")
}

func TestOtpGenerationAndVerificationUnit(t *testing.T) {
	// 1. Standard OTP code generation
	otpCode, err := GenerateCode(nil)
	if err != nil {
		t.Fatalf("failed to generate OTP: %v", err)
	}

	if len(otpCode) != expectedCodeLength {
		t.Fatalf("expected 6-digit code, got: %s", otpCode)
	}

	for _, character := range otpCode {
		if character < '0' || character > '9' {
			t.Fatalf("expected numeric digits only, got character: %c", character)
		}
	}

	// 2. Hash code computation and whitespace trimming
	otpCodeHash := HashCode(otpCode)
	if len(otpCodeHash) != expectedHashLength {
		t.Fatalf("expected 64-char sha256 hex, got: %s", otpCodeHash)
	}

	whitespacePaddedCode := "  " + otpCode + " \t\n"
	if HashCode(whitespacePaddedCode) != otpCodeHash {
		t.Fatal("expected whitespace-trimmed hash to match raw code hash")
	}

	// 3. Constant-time code verification
	if !VerifyCode(otpCode, otpCodeHash) {
		t.Fatalf("expected OTP code verification to succeed")
	}

	if !VerifyCode(whitespacePaddedCode, otpCodeHash) {
		t.Fatalf("expected whitespace-padded OTP code verification to succeed")
	}

	if VerifyCode("999999", otpCodeHash) && otpCode != "999999" {
		t.Fatalf("expected verification to fail for mismatching code")
	}

	if VerifyCode("", otpCodeHash) {
		t.Fatal("expected verification to fail for empty code")
	}

	if VerifyCode(otpCode, "") {
		t.Fatal("expected verification to fail for empty stored hash")
	}

	// 4. Zero padding test with deterministically small random values
	zeroBufferReader := bytes.NewReader(make([]byte, entropyFailureSize))
	zeroPaddedCode, err := GenerateCode(zeroBufferReader)
	if err != nil {
		t.Fatalf("failed to generate zero-padded code: %v", err)
	}
	if len(zeroPaddedCode) != expectedCodeLength {
		t.Fatalf("expected 6 digits for zero-padded code, got: %s", zeroPaddedCode)
	}
	if zeroPaddedCode != "000000" {
		t.Fatalf("expected 000000 from zero reader, got: %s", zeroPaddedCode)
	}

	// 5. Expiration check edge cases
	now := time.Now().UTC()
	if IsExpired(now.Add(fiveMinutesOffset)) {
		t.Fatalf("future time should not be expired")
	}
	if !IsExpired(now.Add(-fiveMinutesOffset)) {
		t.Fatalf("past time should be expired")
	}
	if !IsExpired(now.Add(-1 * time.Second)) {
		t.Fatalf("1 second in past should be expired")
	}

	// 6. Entropy error handling
	if _, err := GenerateCode(errReader{}); err == nil {
		t.Fatalf("expected error on entropy error")
	}

	// 7. Constants and Record serialization assertions
	if CodeTTL != 15*time.Minute {
		t.Fatalf("expected CodeTTL to be 15m, got: %v", CodeTTL)
	}
	if MaxAttempts != 5 {
		t.Fatalf("expected MaxAttempts to be 5, got: %d", MaxAttempts)
	}

	sampleRecord := Record{
		ID:        "01918342-0000-7000-8000-000000000001",
		Recipient: "+15551234567",
		CodeHash:  otpCodeHash,
		Purpose:   "login",
		Attempts:  0,
		ExpiresAt: now.Add(CodeTTL),
		CreatedAt: now,
	}
	encodedPayload, marshalErr := json.Marshal(sampleRecord)
	if marshalErr != nil {
		t.Fatalf("failed to marshal Record: %v", marshalErr)
	}
	if !strings.Contains(string(encodedPayload), "+15551234567") {
		t.Fatalf("missing recipient in serialized Record: %s", string(encodedPayload))
	}

	var decodedRecord Record
	if unmarshalErr := json.Unmarshal(encodedPayload, &decodedRecord); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal Record: %v", unmarshalErr)
	}
	if decodedRecord.Recipient != sampleRecord.Recipient || decodedRecord.CodeHash != sampleRecord.CodeHash {
		t.Fatalf("mismatched decoded Record: %+v", decodedRecord)
	}
}
