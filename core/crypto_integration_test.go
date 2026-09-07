package core

import (
	"bytes"
	"strings"
	"testing"
)

func TestCoreCryptoEncryptionKeyRotationIntegration(t *testing.T) {
	oldEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	newEncryptionKeyHex := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	unrelatedEncryptionKeyHex := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	oldCryptoKeyManager, err := NewCryptoKeyManager(oldEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create old key manager: %v", err)
	}

	newCryptoKeyManager, err := NewCryptoKeyManager(newEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create new key manager: %v", err)
	}

	unrelatedCryptoKeyManager, err := NewCryptoKeyManager(unrelatedEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create unrelated key manager: %v", err)
	}

	testPayloads := [][]byte{
		[]byte("smtp-password-to-rotate-12345"),
		[]byte(""),
		bytes.Repeat([]byte("large-payload-block-for-rotation-"), 250),
	}

	for _, secretPayload := range testPayloads {
		// 1. Encrypt with old key
		ciphertextOld, encryptErr := oldCryptoKeyManager.EncryptField(secretPayload)
		if encryptErr != nil {
			t.Fatalf("failed to encrypt with old key: %v", encryptErr)
		}

		// 2. Decrypt with old key
		decryptedPayload, decryptErr := oldCryptoKeyManager.DecryptField(ciphertextOld)
		if decryptErr != nil {
			t.Fatalf("failed to decrypt with old key: %v", decryptErr)
		}
		if !bytes.Equal(decryptedPayload, secretPayload) {
			t.Fatal("decrypted payload mismatch")
		}

		// 3. Rotate: re-encrypt with new key
		ciphertextNew, reencryptErr := newCryptoKeyManager.EncryptField(decryptedPayload)
		if reencryptErr != nil {
			t.Fatalf("failed to encrypt with new key: %v", reencryptErr)
		}

		// 4. Assert new ciphertext decrypts with new key
		decryptedNew, newDecryptErr := newCryptoKeyManager.DecryptField(ciphertextNew)
		if newDecryptErr != nil {
			t.Fatalf("failed to decrypt with new key: %v", newDecryptErr)
		}
		if !bytes.Equal(decryptedNew, secretPayload) {
			t.Fatal("decrypted payload from new key mismatch")
		}

		// 5. Assert old key cannot decrypt new ciphertext
		if _, decryptFailErr := oldCryptoKeyManager.DecryptField(ciphertextNew); decryptFailErr == nil {
			t.Fatal("expected decryption failure when using old key on new ciphertext")
		}

		// 6. Assert unrelated key cannot decrypt old or new ciphertext
		if _, unrelatedOldErr := unrelatedCryptoKeyManager.DecryptField(ciphertextOld); unrelatedOldErr == nil {
			t.Fatal("expected decryption failure when using unrelated key on old ciphertext")
		}
		if _, unrelatedNewErr := unrelatedCryptoKeyManager.DecryptField(ciphertextNew); unrelatedNewErr == nil {
			t.Fatal("expected decryption failure when using unrelated key on new ciphertext")
		}
	}

	// 7. Edge Case: Tampering during key rotation pipeline
	validCiphertext, err := oldCryptoKeyManager.EncryptField([]byte("tamper-proof-secret"))
	if err != nil {
		t.Fatalf("failed to encrypt secret: %v", err)
	}
	tamperedCiphertext := strings.Replace(validCiphertext, "enc:v1:aes256gcm:", "enc:v1:aes256gcm:TAMPERED", 1)
	if _, err := oldCryptoKeyManager.DecryptField(tamperedCiphertext); err == nil {
		t.Fatal("expected decryption error on tampered ciphertext during rotation")
	}
}
