package passkey

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"layr.sh/core"
)

const (
	tenMinutesOffset = 10 * time.Minute
)

type errEntropyReader struct{}

func (errEntropyReader) Read(_ []byte) (int, error) {
	return 0, errors.New("entropy failure")
}

func TestPasskeyManagerUnit(t *testing.T) {
	// 1. Default Relying Party Initialization
	defaultManager := NewManager("", "")
	if defaultManager.relyingPartyID != "localhost" || defaultManager.relyingPartyName != core.GetConfig().Project.Name {
		t.Fatalf("unexpected defaults: %+v", defaultManager)
	}

	// Dynamic core.Config project name verification
	t.Cleanup(func() {
		core.SetLoadedConfig(nil)
	})
	configuredProjectName := "Production Passkey Portal"
	core.SetLoadedConfig(&core.Config{
		Project: core.ProjectConfig{
			Name: configuredProjectName,
		},
	})
	configuredManager := NewManager("", "")
	if configuredManager.relyingPartyName != configuredProjectName {
		t.Fatalf("expected relying party name %s from core.Config, got: %s", configuredProjectName, configuredManager.relyingPartyName)
	}

	// Empty project name fallback
	core.SetLoadedConfig(&core.Config{
		Project: core.ProjectConfig{
			Name: "",
		},
	})
	fallbackManager := NewManager("", "")
	if fallbackManager.relyingPartyName != "Layr App" {
		t.Fatalf("expected fallback relying party name 'Layr App', got: %s", fallbackManager.relyingPartyName)
	}
	core.SetLoadedConfig(nil)

	// 2. Custom Relying Party Initialization
	customPasskeyManager := NewManager("myapp.internal", "Custom App")
	if customPasskeyManager.relyingPartyID != "myapp.internal" || customPasskeyManager.relyingPartyName != "Custom App" {
		t.Fatalf("expected myapp.internal and Custom App, got: %s, %s",
			customPasskeyManager.relyingPartyID, customPasskeyManager.relyingPartyName)
	}

	// 3. BeginSignUp challenge generation
	signUpOptions, err := customPasskeyManager.BeginSignUp("user-123", "Alice")
	if err != nil {
		t.Fatalf("failed to begin sign up: %v", err)
	}
	if signUpOptions.Challenge == "" || signUpOptions.UserID != "user-123" || signUpOptions.UserName != "Alice" {
		t.Fatalf("unexpected sign up response: %+v", signUpOptions)
	}
	if signUpOptions.RelyingPartyID != "myapp.internal" || signUpOptions.RelyingPartyName != "Custom App" {
		t.Fatalf("unexpected relying party: %+v", signUpOptions)
	}

	// Verify challenge is valid 32-byte raw URL base64
	decodedChallenge, decodeErr := base64.RawURLEncoding.DecodeString(signUpOptions.Challenge)
	if decodeErr != nil || len(decodedChallenge) != challengeByteLength {
		t.Fatalf("expected 32-byte raw URL base64 challenge, got: %v (len: %d)", decodeErr, len(decodedChallenge))
	}

	// 4. BeginSignIn challenge generation
	signInOptions, err := customPasskeyManager.BeginSignIn()
	if err != nil {
		t.Fatalf("failed to begin signin: %v", err)
	}
	if signInOptions.Challenge == "" || signInOptions.RelyingPartyID != "myapp.internal" {
		t.Fatalf("expected non-empty challenge with myapp.internal, got: %+v", signInOptions)
	}

	// 5. Consume valid challenge
	consumedUserID, err := customPasskeyManager.ConsumeChallenge(signUpOptions.Challenge)
	if err != nil || consumedUserID != "user-123" {
		t.Fatalf("expected user-123, got: %s (err: %v)", consumedUserID, err)
	}

	// 6. Consume non-existent / already consumed challenge
	if _, consumeErr := customPasskeyManager.ConsumeChallenge(signUpOptions.Challenge); consumeErr == nil {
		t.Fatal("expected error on consumed challenge")
	}
	if _, consumeErr := customPasskeyManager.ConsumeChallenge("completely-unknown-challenge"); consumeErr == nil {
		t.Fatal("expected error on non-existent challenge")
	}

	// 7. Expired challenge
	expiredChallengeKey := "expired-challenge-key"
	customPasskeyManager.challenges[expiredChallengeKey] = SessionChallenge{
		UserID:    "user-expired",
		Challenge: expiredChallengeKey,
		ExpiresAt: time.Now().Add(-tenMinutesOffset),
	}
	if _, consumeErr := customPasskeyManager.ConsumeChallenge(expiredChallengeKey); consumeErr == nil {
		t.Fatal("expected error on expired challenge")
	}

	// 8. Signature verification helper
	validPublicKey := []byte("public-key-bytes")
	validClientData := []byte(`{"type":"webauthn.get","challenge":"abc"}`)
	validAuthenticatorData := []byte("authenticator-data-bytes")
	validSignature := []byte("signature-bytes")

	if !VerifySignature(validPublicKey, validClientData, validAuthenticatorData, validSignature) {
		t.Fatal("expected verify signature to pass")
	}
	if VerifySignature(nil, validClientData, validAuthenticatorData, validSignature) {
		t.Fatal("expected verify signature to fail on nil public key")
	}
	if VerifySignature(validPublicKey, validClientData, validAuthenticatorData, nil) {
		t.Fatal("expected verify signature to fail on nil signature")
	}
	if VerifySignature([]byte{}, validClientData, validAuthenticatorData, validSignature) {
		t.Fatal("expected verify signature to fail on empty public key")
	}
	if VerifySignature(validPublicKey, validClientData, validAuthenticatorData, []byte{}) {
		t.Fatal("expected verify signature to fail on empty signature")
	}

	// 9. Entropy failure injection
	customPasskeyManager.SetRandomReader(errEntropyReader{})
	if _, regErr := customPasskeyManager.BeginSignUp("user-456", "Bob"); regErr == nil {
		t.Fatal("expected error on entropy failure during sign up")
	}
	if _, signErr := customPasskeyManager.BeginSignIn(); signErr == nil {
		t.Fatal("expected error on entropy failure during signin")
	}
}
