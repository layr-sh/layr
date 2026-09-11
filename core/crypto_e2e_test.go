package core

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestCoreCryptoZeroDecryptionLifecycleE2E(t *testing.T) {
	// Scenario 1: Cryptographic Root & Publishable Key Initialization
	encryptionKeyHex, err := GenerateRandomCryptoEncryptionKeyHex()
	if err != nil {
		t.Fatalf("failed to generate random master key: %v", err)
	}

	cryptoKeyManager, err := NewCryptoKeyManager(encryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	publishableKey := cryptoKeyManager.DerivePublishableKey()
	if !cryptoKeyManager.VerifyPublishableKey(publishableKey) {
		t.Fatal("publishable key verification failed")
	}

	// Publishable key edge cases
	if !cryptoKeyManager.VerifyPublishableKey("  " + publishableKey + "\n") {
		t.Fatal("publishable key verification with whitespace failed")
	}
	if cryptoKeyManager.VerifyPublishableKey("invalid-key-attempt-000000000000000000000000000000000000000000000000") {
		t.Fatal("expected failure on invalid publishable key")
	}
	if cryptoKeyManager.VerifyPublishableKey("") {
		t.Fatal("expected failure on empty publishable key")
	}

	// Scenario 2: Secret Input Envelope Encryption & Sanitized UI Output
	testSecrets := [][]byte{
		[]byte("aws-s3-secret-access-key-xyz987654321"),
		[]byte(""),
		bytes.Repeat([]byte("secret-payload-block-"), 100),
	}

	for _, rawSecret := range testSecrets {
		encryptedSecret, err := cryptoKeyManager.EncryptField(rawSecret)
		if err != nil {
			t.Fatalf("failed to envelope encrypt secret: %v", err)
		}

		// Zero-Decryption Principle: Secret bytes must never appear unencrypted in ciphertext
		if len(rawSecret) > 0 && bytes.Contains([]byte(encryptedSecret), rawSecret) {
			t.Fatal("encrypted envelope leaks plaintext secret bytes")
		}

		// Internal backend decryption
		decryptedSecret, err := cryptoKeyManager.DecryptField(encryptedSecret)
		if err != nil {
			t.Fatalf("failed to decrypt envelope secret: %v", err)
		}
		if !bytes.Equal(decryptedSecret, rawSecret) {
			t.Fatalf("expected decrypted secret %s, got %s", string(rawSecret), string(decryptedSecret))
		}

		// Scenario 3: Tampering & Integrity Verification Edge Cases
		if len(encryptedSecret) > 20 {
			tamperedEnvelope := encryptedSecret[:len(encryptedSecret)-5] + "XXXXX"
			if _, err := cryptoKeyManager.DecryptField(tamperedEnvelope); err == nil {
				t.Fatal("expected decryption failure on tampered auth tag")
			}
		}
	}
}

func TestCoreCryptoTOTPConsoleMFAFlowE2E(t *testing.T) {
	// Full E2E Scenario: Console user MFA onboarding, storage encryption, and multi-factor login verification
	encryptionKeyHex, err := GenerateRandomCryptoEncryptionKeyHex()
	if err != nil {
		t.Fatalf("failed to generate master key: %v", err)
	}

	cryptoKeyManager, err := NewCryptoKeyManager(encryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	totpManager := NewTOTPManager("Layr Console")

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
