package core

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// simulatedFailingReader is an io.Reader that always returns an error.
type simulatedFailingReader struct{}

func (simulatedFailingReader *simulatedFailingReader) Read(destination []byte) (int, error) {
	return 0, errors.New("simulated random failure")
}

func TestCoreCryptoKeyManagerNewUnit(t *testing.T) {
	validEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// 1. Valid 64-char hex key
	if _, err := NewCryptoKeyManager(validEncryptionKeyHex); err != nil {
		t.Fatalf("expected valid key manager, got %v", err)
	}

	// 2. Whitespace trimmed hex key
	if _, err := NewCryptoKeyManager("  \t\n" + validEncryptionKeyHex + "  \n"); err != nil {
		t.Fatalf("expected valid key manager after trimming whitespace, got %v", err)
	}

	// 3. Valid base64 key (32 bytes standard encoding)
	validBase64Key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	if _, err := NewCryptoKeyManager(validBase64Key); err != nil {
		t.Fatalf("expected valid key manager for standard base64, got %v", err)
	}

	// 4. Valid raw base64 key (32 bytes unpadded)
	validRawBase64Key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if _, err := NewCryptoKeyManager(validRawBase64Key); err != nil {
		t.Fatalf("expected valid key manager for raw base64, got %v", err)
	}

	// 5. Invalid short key
	if _, err := NewCryptoKeyManager("short"); err == nil {
		t.Fatal("expected error on short key")
	}

	// 6. Invalid hex characters
	if _, err := NewCryptoKeyManager("zzzz456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"); err == nil {
		t.Fatal("expected error on invalid hex")
	}

	// 7. Non-64 char, invalid base64
	if _, err := NewCryptoKeyManager("not-base64-!!!"); err == nil {
		t.Fatal("expected error on invalid base64")
	}

	// 8. Non-64 char, valid base64 but wrong decoded length (16 bytes instead of 32)
	if _, err := NewCryptoKeyManager("AAAAAAAAAAAAAAAAAAAAAA=="); err == nil {
		t.Fatal("expected error on base64 key with wrong byte length")
	}

	// 9. Empty string
	if _, err := NewCryptoKeyManager(""); err == nil {
		t.Fatal("expected error on empty master encryption key")
	}
}

func TestCoreCryptoKeyManagerDeriveSubkeyUnit(t *testing.T) {
	validEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cryptoKeyManager, _ := NewCryptoKeyManager(validEncryptionKeyHex)

	allContexts := []string{
		CryptoContextDBEnvelopeAES256GCM,
		CryptoContextAuthJWTSigning,
		CryptoContextConsoleUserSalt,
		CryptoContextCoreIPCHMAC,
		CryptoContextFileStorageChunkAES256GCM,
		CryptoContextPublishableKey,
		"custom:arbitrary:context:v1",
	}

	derivedMap := make(map[string][]byte)
	for _, contextName := range allContexts {
		subkey := cryptoKeyManager.DeriveSubkey(contextName)
		if len(subkey) != 32 {
			t.Fatalf("DeriveSubkey(%s) returned invalid length: %d", contextName, len(subkey))
		}
		for existingContext, existingSubkey := range derivedMap {
			if bytes.Equal(existingSubkey, subkey) {
				t.Fatalf("context isolation collision between '%s' and '%s'", existingContext, contextName)
			}
		}
		derivedMap[contextName] = subkey
	}

	// Deterministic: same context -> same subkey
	repeatedSubkey := cryptoKeyManager.DeriveSubkey("test:deterministic")
	repeatedSubkeySecond := cryptoKeyManager.DeriveSubkey("test:deterministic")
	if !bytes.Equal(repeatedSubkey, repeatedSubkeySecond) {
		t.Fatal("expected deterministic subkey derivation")
	}
}

func TestCoreCryptoKeyManagerEncryptDecryptFieldUnit(t *testing.T) {
	validEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cryptoKeyManager, _ := NewCryptoKeyManager(validEncryptionKeyHex)

	testPayloads := [][]byte{
		[]byte(""),
		[]byte("a"),
		[]byte("super-secret-oauth-client-secret-12345"),
		make([]byte, 1024),
		bytes.Repeat([]byte("payload-segment-"), 100),
	}

	for _, payload := range testPayloads {
		encrypted, err := cryptoKeyManager.EncryptField(payload)
		if err != nil {
			t.Fatalf("EncryptField failed: %v", err)
		}

		if !strings.HasPrefix(encrypted, "enc:v1:aes256gcm:") {
			t.Fatalf("expected envelope prefix, got %s", encrypted)
		}

		decrypted, err := cryptoKeyManager.DecryptField(encrypted)
		if err != nil {
			t.Fatalf("DecryptField failed: %v", err)
		}

		if !bytes.Equal(payload, decrypted) {
			t.Fatalf("decrypted mismatch: expected %d bytes, got %d bytes", len(payload), len(decrypted))
		}
	}

	// Nonce randomness: encrypting identical plaintext twice produces distinct ciphertexts
	samplePlaintext := []byte("identical-plaintext-secret")
	ciphertextFirst, err := cryptoKeyManager.EncryptField(samplePlaintext)
	if err != nil {
		t.Fatalf("first encryption failed: %v", err)
	}
	ciphertextSecond, err := cryptoKeyManager.EncryptField(samplePlaintext)
	if err != nil {
		t.Fatalf("second encryption failed: %v", err)
	}
	if ciphertextFirst == ciphertextSecond {
		t.Fatal("expected nonces to differ between independent encryptions")
	}
}

func TestCoreCryptoKeyManagerDecryptFieldErrorsUnit(t *testing.T) {
	validEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cryptoKeyManager, _ := NewCryptoKeyManager(validEncryptionKeyHex)

	// Bad prefix
	if _, err := cryptoKeyManager.DecryptField("badprefix:1:2:3:4:5"); err == nil {
		t.Fatal("expected error on bad prefix")
	}

	// Bad chunk counts
	if _, err := cryptoKeyManager.DecryptField("enc:v1:aes256gcm:too:few"); err == nil {
		t.Fatal("expected error on too few chunks")
	}
	if _, err := cryptoKeyManager.DecryptField("enc:v1:aes256gcm:one:two:three:four:five"); err == nil {
		t.Fatal("expected error on too many chunks")
	}

	// Invalid IV base64
	if _, err := cryptoKeyManager.DecryptField("enc:v1:aes256gcm:!!!:AAAA:AAAA"); err == nil {
		t.Fatal("expected error on invalid IV base64")
	}

	// Invalid CT base64
	if _, err := cryptoKeyManager.DecryptField("enc:v1:aes256gcm:AAAA:!!!:AAAA"); err == nil {
		t.Fatal("expected error on invalid CT base64")
	}

	// Invalid Tag base64
	if _, err := cryptoKeyManager.DecryptField("enc:v1:aes256gcm:AAAA:AAAA:!!!"); err == nil {
		t.Fatal("expected error on invalid Tag base64")
	}

	// Invalid IV size (3 bytes instead of 12)
	if _, err := cryptoKeyManager.DecryptField("enc:v1:aes256gcm:AAAA:AAAA:AAAA"); err == nil {
		t.Fatal("expected error on invalid IV size")
	}

	// Valid 12-byte IV but bad ciphertext/tag authentication failure
	validIV12 := "AAAAAAAAAAAAAAAA" // 12 bytes raw base64
	if _, err := cryptoKeyManager.DecryptField("enc:v1:aes256gcm:" + validIV12 + ":AAAA:AAAAAAAAAAAAAAAAAAAAAA"); err == nil {
		t.Fatal("expected error on bad cipher tag authentication")
	}
}

func TestCoreCryptoKeyManagerEncryptFieldWithFailingRandomReaderUnit(t *testing.T) {
	validEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cryptoKeyManager, _ := NewCryptoKeyManager(validEncryptionKeyHex)

	// Inject failing random reader
	cryptoKeyManager.randomReader = &simulatedFailingReader{}

	_, err := cryptoKeyManager.EncryptField([]byte("test"))
	if err == nil {
		t.Fatal("expected error when random reader fails")
	}
	if !strings.Contains(err.Error(), "simulated random failure") {
		t.Fatalf("expected simulated random failure error, got: %v", err)
	}
}

func TestCoreCryptoKeyManagerEncryptFieldWithLimitedReaderUnit(t *testing.T) {
	validEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cryptoKeyManager, _ := NewCryptoKeyManager(validEncryptionKeyHex)

	// Reader that provides only 5 bytes (insufficient for 12-byte nonce)
	cryptoKeyManager.randomReader = io.LimitReader(bytes.NewReader(make([]byte, 5)), 5)

	_, err := cryptoKeyManager.EncryptField([]byte("test"))
	if err == nil {
		t.Fatal("expected error when random reader provides insufficient bytes")
	}
}

func TestCoreCryptoGenerateRandomEncryptionKeyHexUnit(t *testing.T) {
	keyHex, err := GenerateRandomCryptoEncryptionKeyHex()
	if err != nil {
		t.Fatalf("GenerateRandomCryptoEncryptionKeyHex failed: %v", err)
	}
	if len(keyHex) != 64 {
		t.Fatalf("expected 64-char hex key, got %d chars", len(keyHex))
	}

	// Verify nil reader in GenerateRandomCryptoEncryptionKeyHexFromReader falls back to rand.Reader
	keyHexNil, err := GenerateRandomCryptoEncryptionKeyHexFromReader(nil)
	if err != nil || len(keyHexNil) != 64 {
		t.Fatalf("GenerateRandomCryptoEncryptionKeyHexFromReader(nil) failed: %v", err)
	}

	// Verify generated key initializes a valid CryptoKeyManager
	if _, err := NewCryptoKeyManager(keyHex); err != nil {
		t.Fatalf("generated key should be valid: %v", err)
	}
}

func TestCoreCryptoGenerateRandomEncryptionKeyHexFailureUnit(t *testing.T) {
	// Swap default rand reader with failing reader
	SetCryptoRandomReader(&simulatedFailingReader{})
	defer SetCryptoRandomReader(nil)

	_, err := GenerateRandomCryptoEncryptionKeyHex()
	if err == nil {
		t.Fatal("expected error when randomReader fails")
	}
	if !strings.Contains(err.Error(), "failed to read random bytes") {
		t.Fatalf("expected 'failed to read random bytes' error, got: %v", err)
	}
}

func TestCoreCryptoKeyManagerPublishableKeyDerivationAndVerificationUnit(t *testing.T) {
	validEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cryptoKeyManager, _ := NewCryptoKeyManager(validEncryptionKeyHex)

	publishableKey := cryptoKeyManager.DerivePublishableKey()
	if len(publishableKey) != 64 {
		t.Fatalf("expected 64-char hex publishable key, got %d chars", len(publishableKey))
	}

	// Infallible & deterministic check
	if cryptoKeyManager.DerivePublishableKey() != publishableKey {
		t.Fatal("expected deterministic publishable key derivation")
	}

	// Verify valid key
	if !cryptoKeyManager.VerifyPublishableKey(publishableKey) {
		t.Fatal("expected VerifyPublishableKey to succeed for valid key")
	}

	// Verify valid key with leading/trailing whitespace
	if !cryptoKeyManager.VerifyPublishableKey("  \t" + publishableKey + " \n") {
		t.Fatal("expected VerifyPublishableKey to succeed with whitespace")
	}

	// Verify invalid key
	if cryptoKeyManager.VerifyPublishableKey("0000000000000000000000000000000000000000000000000000000000000000") {
		t.Fatal("expected VerifyPublishableKey to fail for wrong key")
	}

	// Verify empty key and whitespace
	if cryptoKeyManager.VerifyPublishableKey("") {
		t.Fatal("expected VerifyPublishableKey to fail for empty string")
	}
	if cryptoKeyManager.VerifyPublishableKey("   \t\n  ") {
		t.Fatal("expected VerifyPublishableKey to fail for whitespace string")
	}
}
