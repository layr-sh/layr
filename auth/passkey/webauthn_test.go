package passkey

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
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

	// 8. Signature verification helper - simulation fallback and basic checks
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

	// 9. Real ECDSA P-256 verification (ASN.1 DER & Raw IEEE P1363 & Raw EC Point)
	ecdsaPrivateKey, genErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if genErr != nil {
		t.Fatalf("failed to generate ECDSA key: %v", genErr)
	}
	ecdsaPKIXBytes, _ := x509.MarshalPKIXPublicKey(&ecdsaPrivateKey.PublicKey)
	ecdsaPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: ecdsaPKIXBytes})

	clientDataHash := sha256.Sum256(validClientData)
	signedData := append(validAuthenticatorData, clientDataHash[:]...)
	signedHash := sha256.Sum256(signedData)

	ecdsaASN1Sig, signErr := ecdsa.SignASN1(rand.Reader, ecdsaPrivateKey, signedHash[:])
	if signErr != nil {
		t.Fatalf("failed to sign with ECDSA: %v", signErr)
	}

	// Valid ECDSA DER signature against PKIX PEM
	if !VerifySignature(ecdsaPEMBytes, validClientData, validAuthenticatorData, ecdsaASN1Sig) {
		t.Fatal("expected ECDSA ASN.1 signature verification to succeed")
	}

	// Raw uncompressed EC point (0x04 || X || Y)
	rawPoint, err := ecdsaPrivateKey.PublicKey.Bytes()
	if err != nil {
		t.Fatalf("failed to encode raw ECDSA public key: %v", err)
	}
	if !VerifySignature(rawPoint, validClientData, validAuthenticatorData, ecdsaASN1Sig) {
		t.Fatal("expected raw uncompressed P-256 point verification to succeed")
	}

	// Raw IEEE P1363 64-byte signature
	rBytes, sBytes, err := ecdsa.Sign(rand.Reader, ecdsaPrivateKey, signedHash[:])
	if err != nil {
		t.Fatalf("failed to generate raw ECDSA signature: %v", err)
	}
	rawP1363Sig := make([]byte, 64)
	rBytes.FillBytes(rawP1363Sig[:32])
	sBytes.FillBytes(rawP1363Sig[32:])
	if !VerifySignature(ecdsaPKIXBytes, validClientData, validAuthenticatorData, rawP1363Sig) {
		t.Fatal("expected raw IEEE P1363 signature verification to succeed")
	}

	// Invalid ECDSA signature
	corruptedSig := append([]byte(nil), ecdsaASN1Sig...)
	corruptedSig[len(corruptedSig)-1] ^= 0xFF
	if VerifySignature(ecdsaPKIXBytes, validClientData, validAuthenticatorData, corruptedSig) {
		t.Fatal("expected invalid ECDSA signature to fail")
	}

	// 10. Real Ed25519 verification
	edPublicKey, edPrivateKey, _ := ed25519.GenerateKey(rand.Reader)
	edPKIXBytes, _ := x509.MarshalPKIXPublicKey(edPublicKey)
	edSig := ed25519.Sign(edPrivateKey, signedData)
	if !VerifySignature(edPKIXBytes, validClientData, validAuthenticatorData, edSig) {
		t.Fatal("expected Ed25519 signature to succeed")
	}
	corruptedEdSig := append([]byte(nil), edSig...)
	corruptedEdSig[0] ^= 0xFF
	if VerifySignature(edPKIXBytes, validClientData, validAuthenticatorData, corruptedEdSig) {
		t.Fatal("expected corrupted Ed25519 signature to fail")
	}

	// 11. Real RSA verification
	rsaPrivateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	rsaPKIXBytes, _ := x509.MarshalPKIXPublicKey(&rsaPrivateKey.PublicKey)
	rsaSig, _ := rsa.SignPKCS1v15(rand.Reader, rsaPrivateKey, crypto.SHA256, signedHash[:])
	if !VerifySignature(rsaPKIXBytes, validClientData, validAuthenticatorData, rsaSig) {
		t.Fatal("expected RSA PKCS1v15 signature to succeed")
	}
	corruptedRSASig := append([]byte(nil), rsaSig...)
	corruptedRSASig[len(corruptedRSASig)-1] ^= 0xFF
	if VerifySignature(rsaPKIXBytes, validClientData, validAuthenticatorData, corruptedRSASig) {
		t.Fatal("expected corrupted RSA signature to fail")
	}
	rsaPSSSig, _ := rsa.SignPSS(rand.Reader, rsaPrivateKey, crypto.SHA256, signedHash[:], nil)
	if !VerifySignature(rsaPKIXBytes, validClientData, validAuthenticatorData, rsaPSSSig) {
		t.Fatal("expected RSA PSS signature to succeed")
	}

	// 12. Entropy failure injection
	customPasskeyManager.SetRandomReader(errEntropyReader{})
	if _, regErr := customPasskeyManager.BeginSignUp("user-456", "Bob"); regErr == nil {
		t.Fatal("expected error on entropy failure during sign up")
	}
	if _, signErr := customPasskeyManager.BeginSignIn(); signErr == nil {
		t.Fatal("expected error on entropy failure during signin")
	}
}
