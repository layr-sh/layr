package core

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

type simulatedErrorReader struct{}

func (simulatedErrorReader *simulatedErrorReader) Read(destination []byte) (int, error) {
	return 0, errors.New("reader error")
}

func TestCoreTOTPManagerInitializationUnit(t *testing.T) {
	// Custom issuer
	customTOTPManager := NewTOTPManager("Custom App")
	if customTOTPManager == nil {
		t.Fatal("expected non-nil TOTPManager")
	}
	if customTOTPManager.issuer != "Custom App" {
		t.Errorf("expected issuer 'Custom App', got '%s'", customTOTPManager.issuer)
	}
	if customTOTPManager.periodSeconds != 30 {
		t.Errorf("expected periodSeconds 30, got %d", customTOTPManager.periodSeconds)
	}
	if customTOTPManager.digits != 6 {
		t.Errorf("expected digits 6, got %d", customTOTPManager.digits)
	}

	// Empty issuer defaults to "Layr"
	defaultTOTPManager := NewTOTPManager("")
	if defaultTOTPManager.issuer != "Layr" {
		t.Errorf("expected default issuer 'Layr', got '%s'", defaultTOTPManager.issuer)
	}
}

func TestCoreTOTPManagerGenerateSecretUnit(t *testing.T) {
	totpManager := NewTOTPManager("Layr Test")

	// 1. Successful generation
	secretBase32, err := totpManager.GenerateSecret()
	if err != nil {
		t.Fatalf("failed to generate secret: %v", err)
	}
	if len(secretBase32) == 0 {
		t.Fatal("expected non-empty secret")
	}
	// 20 bytes in unpadded base32 is exactly 32 characters
	if len(secretBase32) != 32 {
		t.Fatalf("expected 32-character base32 secret for 20 random bytes, got %d chars (%s)", len(secretBase32), secretBase32)
	}

	// 2. Failure with error reader
	totpManager.SetRandomReader(&simulatedErrorReader{})
	defer totpManager.SetRandomReader(nil)

	if _, err := totpManager.GenerateSecret(); err == nil {
		t.Fatal("expected error on GenerateSecret with failing reader")
	}
}

func TestCoreTOTPManagerGenerateCodeUnit(t *testing.T) {
	totpManager := NewTOTPManager("Layr Test")
	secretBase32, _ := totpManager.GenerateSecret()

	now := time.Now().UTC()
	code, err := totpManager.GenerateCode(secretBase32, now)
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got %s", code)
	}

	// Case insensitivity: lowercase secret should generate identical code
	lowercaseSecret := strings.ToLower(secretBase32)
	lowercaseCode, err := totpManager.GenerateCode(lowercaseSecret, now)
	if err != nil {
		t.Fatalf("failed to generate code with lowercase secret: %v", err)
	}
	if lowercaseCode != code {
		t.Fatalf("expected case-insensitive code generation, got %s vs %s", code, lowercaseCode)
	}

	// Whitespace in secret should be trimmed
	paddedSecret := "  " + secretBase32 + " \n"
	paddedCode, err := totpManager.GenerateCode(paddedSecret, now)
	if err != nil {
		t.Fatalf("failed to generate code with padded secret: %v", err)
	}
	if paddedCode != code {
		t.Fatalf("expected trimmed secret to match, got %s vs %s", code, paddedCode)
	}

	// Malformed base32
	if _, err := totpManager.GenerateCode("!!!bad-base32-char!!!", now); err == nil {
		t.Fatal("expected error on GenerateCode with bad base32")
	}
}

func TestCoreTOTPManagerValidateCodeUnit(t *testing.T) {
	totpManager := NewTOTPManager("Layr Test")
	secretBase32, _ := totpManager.GenerateSecret()

	now := time.Now().UTC()
	code, _ := totpManager.GenerateCode(secretBase32, now)

	// 1. Exact match at current timestamp
	if !totpManager.ValidateCode(secretBase32, code, now, 1) {
		t.Error("expected valid TOTP code to pass validation")
	}

	// 2. Whitespace in submitted code should be tolerated
	if !totpManager.ValidateCode(secretBase32, "  "+code+" \n", now, 1) {
		t.Error("expected valid TOTP code with whitespace to pass validation")
	}

	// 3. Forward skew (+30s within skew window = 1)
	futureTimestamp := now.Add(30 * time.Second)
	if !totpManager.ValidateCode(secretBase32, code, futureTimestamp, 1) {
		t.Error("expected code to validate within +1 skew window")
	}

	// 4. Backward skew (-30s within skew window = 1)
	pastTimestamp := now.Add(-30 * time.Second)
	if !totpManager.ValidateCode(secretBase32, code, pastTimestamp, 1) {
		t.Error("expected code to validate within -1 skew window")
	}

	// 5. Negative skewSteps defaults to 1
	if !totpManager.ValidateCode(secretBase32, code, now, -1) {
		t.Error("expected code to validate with negative skew default")
	}

	// 6. Zero skew window validates only exact step
	if !totpManager.ValidateCode(secretBase32, code, now, 0) {
		t.Error("expected exact match validation with skew=0")
	}
	futureOutsideZeroSkew := now.Add(35 * time.Second)
	if totpManager.ValidateCode(secretBase32, code, futureOutsideZeroSkew, 0) {
		t.Error("expected code outside skew=0 window to fail")
	}

	// 7. Outside skew window (+90s with skew = 1)
	farFuture := now.Add(90 * time.Second)
	if totpManager.ValidateCode(secretBase32, code, farFuture, 1) {
		t.Error("expected code to fail validation outside skew window")
	}

	// 8. Invalid code lengths
	for _, invalidLengthCode := range []string{"", "123", "12345", "1234567", "abcdef"} {
		if totpManager.ValidateCode(secretBase32, invalidLengthCode, now, 1) {
			t.Errorf("expected invalid code length '%s' to fail validation", invalidLengthCode)
		}
	}

	// 9. Invalid base32 secret
	if totpManager.ValidateCode("!!!invalid-base32!!!", code, now, 1) {
		t.Error("expected invalid secret to fail in ValidateCode")
	}

	// 10. Wrong code mismatch
	wrongCode := "999999"
	if code == "999999" {
		wrongCode = "000000"
	}
	if totpManager.ValidateCode(secretBase32, wrongCode, now, 1) {
		t.Error("expected wrong code to fail validation")
	}
}

func TestCoreTOTPManagerBuildAuthURLUnit(t *testing.T) {
	totpManager := NewTOTPManager("Layr Studio")
	secretBase32 := "JBSWY3DPEHPK3PXP"

	authURL := totpManager.BuildAuthURL("console_user@layr.sh", secretBase32)

	if !strings.HasPrefix(authURL, "otpauth://totp/") {
		t.Fatalf("expected prefix 'otpauth://totp/', got %s", authURL)
	}
	if !strings.Contains(authURL, secretBase32) {
		t.Errorf("expected auth URL to contain secret: %s", authURL)
	}
	if !strings.Contains(authURL, "issuer=Layr+Studio") && !strings.Contains(authURL, "issuer=Layr%20Studio") {
		t.Errorf("expected encoded issuer, got: %s", authURL)
	}
	if !strings.Contains(authURL, "digits=6") {
		t.Errorf("expected digits=6 in URL, got: %s", authURL)
	}
	if !strings.Contains(authURL, "period=30") {
		t.Errorf("expected period=30 in URL, got: %s", authURL)
	}
}

func TestCoreTOTPManagerSetRandomReaderUnit(t *testing.T) {
	totpManager := NewTOTPManager("Layr Test")

	buffer := bytes.NewReader(make([]byte, 32))
	totpManager.SetRandomReader(buffer)
	if totpManager.randomReader != buffer {
		t.Fatal("expected custom reader to be set")
	}

	totpManager.SetRandomReader(nil)
	if totpManager.randomReader == nil {
		t.Fatal("expected nil reader to reset to rand.Reader")
	}
}
