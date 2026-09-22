package core

import (
	"bytes"
	"testing"
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
