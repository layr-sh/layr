package totp

import (
	"strings"
	"testing"
	"time"

	"layr.sh/core"
)

func TestTOTPEncryptionLifecycleIntegration(t *testing.T) {
	encryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cryptoKeyManager, err := core.NewCryptoKeyManager(encryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	totpManager := NewManager("Layr Console")

	// 1. Generate TOTP secret during console user setup
	totpSecret, err := totpManager.GenerateSecret()
	if err != nil {
		t.Fatalf("failed to generate TOTP secret: %v", err)
	}

	// 2. Generate authenticator onboarding URL
	authURL := totpManager.BuildAuthURL("console_user@layr.sh", totpSecret)
	if !strings.HasPrefix(authURL, "otpauth://totp/") {
		t.Fatalf("invalid onboarding URL: %s", authURL)
	}

	// 3. Envelope encrypt secret for persistence in PostgreSQL (console.users)
	encryptedEnvelope, err := cryptoKeyManager.EncryptField([]byte(totpSecret))
	if err != nil {
		t.Fatalf("failed to encrypt TOTP secret: %v", err)
	}
	if !strings.HasPrefix(encryptedEnvelope, "enc:v1:aes256gcm:") {
		t.Fatalf("expected envelope prefix, got: %s", encryptedEnvelope)
	}

	// 4. Assert zero-decryption principle: envelope must not contain plaintext base32 secret
	if strings.Contains(encryptedEnvelope, totpSecret) {
		t.Fatal("zero-decryption principle violated: raw secret leaked in ciphertext")
	}

	// 5. Backend retrieval and validation during console login
	decryptedBytes, err := cryptoKeyManager.DecryptField(encryptedEnvelope)
	if err != nil {
		t.Fatalf("failed to decrypt TOTP secret envelope: %v", err)
	}
	decryptedSecret := string(decryptedBytes)
	if decryptedSecret != totpSecret {
		t.Fatalf("decrypted secret mismatch: expected %s, got %s", totpSecret, decryptedSecret)
	}

	// 6. Validate user submitted code against decrypted secret
	now := time.Now().UTC()
	userCode, err := totpManager.GenerateCode(decryptedSecret, now)
	if err != nil {
		t.Fatalf("failed to generate code from decrypted secret: %v", err)
	}

	if !totpManager.ValidateCode(decryptedSecret, userCode, now, 1) {
		t.Fatal("validation failed for code generated from decrypted secret")
	}

	// 7. Edge case: Corrupted envelope in storage prevents TOTP verification
	corruptedEnvelope := encryptedEnvelope + "corrupt"
	if _, err := cryptoKeyManager.DecryptField(corruptedEnvelope); err == nil {
		t.Fatal("expected decryption failure on corrupted envelope")
	}
}
