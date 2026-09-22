package totp

import (
	"strings"
	"testing"
	"time"

	"layr.sh/core"
)

func TestTOTPConsoleMFAFlowE2E(t *testing.T) {
	// Full E2E Scenario: Console user MFA onboarding, storage encryption, and multi-factor login verification
	encryptionKeyHex, err := core.GenerateRandomCryptoEncryptionKeyHex()
	if err != nil {
		t.Fatalf("failed to generate master key: %v", err)
	}

	cryptoKeyManager, err := core.NewCryptoKeyManager(encryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	totpManager := NewManager("Layr Console")

	// Step 1: Onboarding - Generate MFA secret and Authenticator QR URL
	secretBase32, err := totpManager.GenerateSecret()
	if err != nil {
		t.Fatalf("failed to generate TOTP secret: %v", err)
	}
	authURL := totpManager.BuildAuthURL("console_user@layr.sh", secretBase32)
	if !strings.Contains(authURL, secretBase32) {
		t.Fatalf("auth URL does not contain secret: %s", authURL)
	}

	// Step 2: Storage - Envelope encrypt into DB storage format
	storedEnvelope, err := cryptoKeyManager.EncryptField([]byte(secretBase32))
	if err != nil {
		t.Fatalf("failed to encrypt TOTP secret for storage: %v", err)
	}

	// Step 3: Zero-Decryption UI Rule - plaintext secret is never readable from stored envelope
	if strings.Contains(storedEnvelope, secretBase32) {
		t.Fatal("zero-decryption rule violated: secret visible in ciphertext")
	}

	// Step 4: Login Attempt - Backend retrieves and decrypts stored envelope
	decryptedBytes, err := cryptoKeyManager.DecryptField(storedEnvelope)
	if err != nil {
		t.Fatalf("backend failed to decrypt stored TOTP envelope: %v", err)
	}
	retrievedSecret := string(decryptedBytes)

	// Step 5: Successful authentication at current time
	loginTime := time.Now().UTC()
	userCode, err := totpManager.GenerateCode(retrievedSecret, loginTime)
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	if !totpManager.ValidateCode(retrievedSecret, userCode, loginTime, 1) {
		t.Fatal("expected TOTP verification to succeed for valid code")
	}

	// Step 6: Edge Case - Code submitted with slight clock skew (+25 seconds)
	skewedTime := loginTime.Add(25 * time.Second)
	if !totpManager.ValidateCode(retrievedSecret, userCode, skewedTime, 1) {
		t.Fatal("expected TOTP verification to succeed within clock skew window")
	}

	// Step 7: Edge Case - Expired code submitted (+95 seconds)
	expiredTime := loginTime.Add(95 * time.Second)
	if totpManager.ValidateCode(retrievedSecret, userCode, expiredTime, 1) {
		t.Fatal("expected expired TOTP code to be rejected")
	}

	// Step 8: Edge Case - Tampered code (invalid digits)
	if totpManager.ValidateCode(retrievedSecret, "000000", loginTime, 1) && userCode != "000000" {
		t.Fatal("expected invalid code to be rejected")
	}

	// Step 9: Edge Case - Storage envelope tampering prevents login
	tamperedEnvelope := strings.Replace(storedEnvelope, "enc:v1:aes256gcm:", "enc:v1:aes256gcm:BAD", 1)
	if _, err := cryptoKeyManager.DecryptField(tamperedEnvelope); err == nil {
		t.Fatal("expected tampered envelope to fail decryption")
	}
}
