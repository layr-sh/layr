package core

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"uuid"

	"layr.sh/logger"
)

func TestCoreJWTSignerAndVerifierUnit(t *testing.T) {
	UnloadConfig()
	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	jwtSigner, err := NewJWTSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	if len(jwtSigner.PublicKey()) == 0 {
		t.Fatalf("expected non-empty public key")
	}

	userID := uuid.NewV7().String()
	sessionID := uuid.NewV7().String()
	token, err := jwtSigner.GenerateAccessToken(JWTClaims{
		Subject:   userID,
		SessionID: sessionID,
		Email:     "user@example.com",
		Phone:     "+123456789",
		Claims:    map[string]any{"plan": "pro", "level": 3, "active": true},
	}, 900)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	jwtClaims, err := jwtSigner.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("failed to verify access token: %v", err)
	}

	if jwtClaims.Subject != userID || jwtClaims.Email != "user@example.com" || jwtClaims.Role != "authenticated" || jwtClaims.IsAnonymous || jwtClaims.SessionID != sessionID {
		t.Fatalf("unexpected claims: %+v", jwtClaims)
	}
	if jwtClaims.Claims["plan"] != "pro" {
		t.Fatalf("expected claims.Claims['plan'] == 'pro', got: %v", jwtClaims.Claims["plan"])
	}

	// Test single Assert with standard aliases and custom claims
	if assertErr := jwtClaims.Assert("sub", userID); assertErr != nil {
		t.Fatalf("assert sub failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("subject", userID); assertErr != nil {
		t.Fatalf("assert subject alias failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("sid", sessionID); assertErr != nil {
		t.Fatalf("assert sid failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("session_id", sessionID); assertErr != nil {
		t.Fatalf("assert session_id alias failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("aud", "layr-app:user"); assertErr != nil {
		t.Fatalf("assert aud failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("audience", "layr-app:user"); assertErr != nil {
		t.Fatalf("assert audience alias failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("iss", "layr-app"); assertErr != nil {
		t.Fatalf("assert iss failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("issuer", "layr-app"); assertErr != nil {
		t.Fatalf("assert issuer alias failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("role", "authenticated"); assertErr != nil {
		t.Fatalf("assert role failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("email", "user@example.com"); assertErr != nil {
		t.Fatalf("assert email failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("phone", "+123456789"); assertErr != nil {
		t.Fatalf("assert phone failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("is_anonymous", false); assertErr != nil {
		t.Fatalf("assert is_anonymous failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("jti", jwtClaims.JWTID); assertErr != nil {
		t.Fatalf("assert jti failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("plan", "pro"); assertErr != nil {
		t.Fatalf("assert plan failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("level", 3); assertErr != nil {
		t.Fatalf("assert numeric level failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("level", int64(3)); assertErr != nil {
		t.Fatalf("assert int64 level failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("level", float64(3)); assertErr != nil {
		t.Fatalf("assert float64 level failed: %v", assertErr)
	}
	if assertErr := jwtClaims.Assert("active", true); assertErr != nil {
		t.Fatalf("assert bool active failed: %v", assertErr)
	}

	// Test Assert failure on mismatch and missing claim
	if assertErr := jwtClaims.Assert("role", "admin"); assertErr == nil {
		t.Fatal("expected assertion error on role mismatch")
	}
	if assertErr := jwtClaims.Assert("non_existent_key", "value"); assertErr == nil {
		t.Fatal("expected assertion error on missing custom claim")
	}
	if assertErr := jwtClaims.Assert("active", 123); assertErr == nil {
		t.Fatal("expected assertion error when comparing bool with number")
	}

	// Anonymous token verification
	anonymousToken, err := jwtSigner.GenerateAccessToken(JWTClaims{
		Subject:     userID,
		IsAnonymous: true,
	}, 900)
	if err != nil {
		t.Fatalf("failed to generate anonymous token: %v", err)
	}
	anonymousJWTClaims, err := jwtSigner.VerifyAccessToken(anonymousToken)
	if err != nil || !anonymousJWTClaims.IsAnonymous {
		t.Fatalf("expected anonymous claims with IsAnonymous=true, got: %+v", anonymousJWTClaims)
	}
	if assertErr := anonymousJWTClaims.Assert("is_anonymous", true); assertErr != nil {
		t.Fatalf("assert is_anonymous failed: %v", assertErr)
	}
	// Test Assert on nil Claims map
	if assertErr := anonymousJWTClaims.Assert("any_custom_key", "val"); assertErr == nil {
		t.Fatal("expected error on nil claims map")
	}

	// Fallback role and expiry
	fallbackToken, err := jwtSigner.GenerateAccessToken(JWTClaims{
		Subject: userID,
	})
	if err != nil {
		t.Fatalf("failed to generate fallback token: %v", err)
	}
	fallbackJWTClaims, err := jwtSigner.VerifyAccessToken(fallbackToken)
	if err != nil || fallbackJWTClaims.Role != "authenticated" {
		t.Fatalf("expected authenticated role, got: %+v", fallbackJWTClaims)
	}

	// Malformed tokens
	if _, verifySegmentErr := jwtSigner.VerifyAccessToken("not.three.segments.extra"); verifySegmentErr == nil {
		t.Fatalf("expected error on 4-part token")
	}
	if _, verifyBase64Err := jwtSigner.VerifyAccessToken("eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiIxIn0.!!invalid-signature-base64!!"); verifyBase64Err == nil {
		t.Fatalf("expected error on bad signature base64")
	}
	if _, verifySigErr := jwtSigner.VerifyAccessToken("eyJhbGciOiJFZERTQSJ9.e30.c2ln"); verifySigErr == nil {
		t.Fatalf("expected signature verification failure")
	}

	// Valid signature but invalid claims base64
	badClaimsPayload := "eyJhbGciOiJFZERTQSJ9.!bad-b64!"
	badSignature := ed25519.Sign(jwtSigner.privateKey, []byte(badClaimsPayload))
	badClaimsToken := badClaimsPayload + "." + base64.RawURLEncoding.EncodeToString(badSignature)
	if _, verifyClaimsDecodeErr := jwtSigner.VerifyAccessToken(badClaimsToken); verifyClaimsDecodeErr == nil {
		t.Fatalf("expected error on bad claims base64")
	}

	// Valid signature but non-JSON claims
	nonJSONClaimsPayload := "eyJhbGciOiJFZERTQSJ9.bm90LWpzb24"
	nonJSONSignature := ed25519.Sign(jwtSigner.privateKey, []byte(nonJSONClaimsPayload))
	nonJSONToken := nonJSONClaimsPayload + "." + base64.RawURLEncoding.EncodeToString(nonJSONSignature)
	if _, verifyJSONErr := jwtSigner.VerifyAccessToken(nonJSONToken); verifyJSONErr == nil {
		t.Fatalf("expected error on non-json claims")
	}

	// Expired token test
	expiredToken, _ := jwtSigner.GenerateAccessToken(JWTClaims{
		Subject:   userID,
		Email:     "exp@example.com",
		ExpiresAt: time.Now().Unix() - 10,
	})
	if _, verifyExpiredErr := jwtSigner.VerifyAccessToken(expiredToken); verifyExpiredErr == nil || !strings.Contains(verifyExpiredErr.Error(), "expired") {
		t.Fatalf("expected expired error, got: %v", verifyExpiredErr)
	}

	// Empty keyID fallback test
	emptyKeyIDJWTSigner := &JWTSigner{
		privateKey: jwtSigner.privateKey,
		publicKey:  jwtSigner.publicKey,
	}
	emptyKeyIDToken, emptyKeyIDErr := emptyKeyIDJWTSigner.GenerateAccessToken(JWTClaims{Subject: userID})
	if emptyKeyIDErr != nil {
		t.Fatalf("failed to generate token with empty keyID: %v", emptyKeyIDErr)
	}
	if _, emptyKeyIDVerifyErr := emptyKeyIDJWTSigner.VerifyAccessToken(emptyKeyIDToken); emptyKeyIDVerifyErr != nil {
		t.Fatalf("failed to verify token from empty keyID signer: %v", emptyKeyIDVerifyErr)
	}

	// Future NotBefore token
	futureJWTClaims := JWTClaims{
		Subject:   userID,
		Issuer:    "layr-app",
		Audience:  "layr-app:user",
		IssuedAt:  time.Now().Unix() + 100,
		NotBefore: time.Now().Unix() + 100,
		ExpiresAt: time.Now().Unix() + 1000,
	}
	futureJSON, _ := json.Marshal(futureJWTClaims)
	headerBase64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	futureClaimsBase64 := base64.RawURLEncoding.EncodeToString(futureJSON)
	futureSignature := ed25519.Sign(jwtSigner.privateKey, []byte(headerBase64+"."+futureClaimsBase64))
	futureToken := headerBase64 + "." + futureClaimsBase64 + "." + base64.RawURLEncoding.EncodeToString(futureSignature)
	if _, verifyFutureErr := jwtSigner.VerifyAccessToken(futureToken); verifyFutureErr == nil || !strings.Contains(verifyFutureErr.Error(), "not valid yet") {
		t.Fatalf("expected not valid yet error, got: %v", verifyFutureErr)
	}

	// Audience mismatch
	badAudienceJWTClaims := futureJWTClaims
	badAudienceJWTClaims.NotBefore = time.Now().Unix() - 10
	badAudienceJWTClaims.Audience = "wrong:audience"
	badAudienceJSON, _ := json.Marshal(badAudienceJWTClaims)
	badAudienceClaimsBase64 := base64.RawURLEncoding.EncodeToString(badAudienceJSON)
	badAudienceSignature := ed25519.Sign(jwtSigner.privateKey, []byte(headerBase64+"."+badAudienceClaimsBase64))
	badAudienceToken := headerBase64 + "." + badAudienceClaimsBase64 + "." + base64.RawURLEncoding.EncodeToString(badAudienceSignature)
	verifiedBadAudienceJWTClaims, verifyAudienceErr := jwtSigner.VerifyAccessToken(badAudienceToken)
	if verifyAudienceErr != nil {
		t.Fatalf("expected signature verification to pass, got: %v", verifyAudienceErr)
	}
	if assertErr := verifiedBadAudienceJWTClaims.Assert("aud", "layr-app:user"); assertErr == nil || !strings.Contains(assertErr.Error(), "mismatch") {
		t.Fatalf("expected audience mismatch error, got: %v", assertErr)
	}

	// Issuer mismatch
	badIssuerJWTClaims := futureJWTClaims
	badIssuerJWTClaims.NotBefore = time.Now().Unix() - 10
	badIssuerJWTClaims.Issuer = "wrong:issuer"
	badIssuerJSON, _ := json.Marshal(badIssuerJWTClaims)
	badIssuerClaimsBase64 := base64.RawURLEncoding.EncodeToString(badIssuerJSON)
	badIssuerSignature := ed25519.Sign(jwtSigner.privateKey, []byte(headerBase64+"."+badIssuerClaimsBase64))
	badIssuerToken := headerBase64 + "." + badIssuerClaimsBase64 + "." + base64.RawURLEncoding.EncodeToString(badIssuerSignature)
	verifiedBadIssuerJWTClaims, verifyIssuerErr := jwtSigner.VerifyAccessToken(badIssuerToken)
	if verifyIssuerErr != nil {
		t.Fatalf("expected signature verification to pass, got: %v", verifyIssuerErr)
	}
	if assertErr := verifiedBadIssuerJWTClaims.Assert("iss", "layr-app"); assertErr == nil || !strings.Contains(assertErr.Error(), "mismatch") {
		t.Fatalf("expected issuer mismatch error, got: %v", assertErr)
	}
}

func TestCoreJWTRefreshTokenAndHMACUnit(t *testing.T) {
	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}
	jwtSigner, err := NewJWTSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	refreshToken := jwtSigner.GenerateRefreshToken()
	if len(refreshToken) != 64 {
		t.Fatalf("expected 64-char hex refresh token, got: %s (len: %d)", refreshToken, len(refreshToken))
	}
	refreshTokenHash := jwtSigner.HashRefreshToken(refreshToken)
	if len(refreshTokenHash) != 64 {
		t.Fatalf("expected 64-char sha256 hex, got: %s", refreshTokenHash)
	}

	message := "test-hmac-payload"
	hmacSignature := jwtSigner.SignHMAC(message)
	if !jwtSigner.VerifyHMAC(message, hmacSignature) {
		t.Fatalf("expected HMAC verification to pass")
	}
	if jwtSigner.VerifyHMAC(message, "invalid-signature") {
		t.Fatalf("expected HMAC verification to fail on bad signature")
	}
}

func TestCoreJWTGenerateIDTokenUnit(t *testing.T) {
	UnloadConfig()
	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}
	jwtSigner, err := NewJWTSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	userID := uuid.NewV7().String()
	idToken, tokenGenerateErr := jwtSigner.GenerateIDToken(JWTClaims{
		Issuer:        "https://auth.example.com",
		Audience:      "client-123",
		Subject:       userID,
		Email:         "test@example.com",
		EmailVerified: true,
		Phone:         "+1234567890",
		Role:          "user",
		Nonce:         "nonce-xyz",
	}, 3600)
	if tokenGenerateErr != nil {
		t.Fatalf("failed to generate ID token: %v", tokenGenerateErr)
	}

	segments := strings.Split(idToken, ".")
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segments))
	}

	claimsBytes, claimsDecodeErr := base64.RawURLEncoding.DecodeString(segments[1])
	if claimsDecodeErr != nil {
		t.Fatalf("failed to decode claims base64: %v", claimsDecodeErr)
	}

	var idTokenJWTClaims JWTClaims
	if unmarshalErr := json.Unmarshal(claimsBytes, &idTokenJWTClaims); unmarshalErr != nil {
		t.Fatalf("failed to parse claims JSON: %v", unmarshalErr)
	}

	if idTokenJWTClaims.Issuer != "https://auth.example.com" {
		t.Errorf("expected issuer https://auth.example.com, got %s", idTokenJWTClaims.Issuer)
	}
	if idTokenJWTClaims.Audience != "client-123" {
		t.Errorf("expected audience client-123, got %s", idTokenJWTClaims.Audience)
	}
	if idTokenJWTClaims.Subject != userID {
		t.Errorf("expected subject %s, got %s", userID, idTokenJWTClaims.Subject)
	}
	if idTokenJWTClaims.Email != "test@example.com" || !idTokenJWTClaims.EmailVerified {
		t.Errorf("unexpected email claims: %s, %v", idTokenJWTClaims.Email, idTokenJWTClaims.EmailVerified)
	}
	if idTokenJWTClaims.Nonce != "nonce-xyz" {
		t.Errorf("expected nonce nonce-xyz, got %s", idTokenJWTClaims.Nonce)
	}

	// Test GenerateIDToken with empty role, empty issuer (fallback to GetConfig().Project.Slug()), and expiry 0
	defaultIDToken, defaultTokenGenerateErr := jwtSigner.GenerateIDToken(JWTClaims{
		Audience: "client-default",
		Subject:  userID,
	})
	if defaultTokenGenerateErr != nil {
		t.Fatalf("failed to generate default id token: %v", defaultTokenGenerateErr)
	}
	if defaultIDToken == "" {
		t.Fatal("expected non-empty default id token")
	}

	defaultSegments := strings.Split(defaultIDToken, ".")
	defaultIDTokenClaimsBytes, defaultClaimsDecodeErr := base64.RawURLEncoding.DecodeString(defaultSegments[1])
	if defaultClaimsDecodeErr != nil {
		t.Fatalf("failed to decode default claims: %v", defaultClaimsDecodeErr)
	}
	var defaultIDTokenJWTClaims JWTClaims
	if defaultUnmarshalErr := json.Unmarshal(defaultIDTokenClaimsBytes, &defaultIDTokenJWTClaims); defaultUnmarshalErr != nil {
		t.Fatalf("failed to unmarshal default claims: %v", defaultUnmarshalErr)
	}
	if defaultIDTokenJWTClaims.Issuer != "layr-app" {
		t.Fatalf("expected fallback issuer 'layr-app', got: %s", defaultIDTokenJWTClaims.Issuer)
	}
}

func TestCoreJWTProjectHandleConfigurationUnit(t *testing.T) {
	UnloadConfig()
	cryptoKeyManager, managerErr := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if managerErr != nil {
		t.Fatalf("failed to create KeyManager: %v", managerErr)
	}

	// 1. Verify default values derived from core default config ("layr-app")
	expectedDefaultIssuer := "layr-app"
	expectedDefaultAudience := "layr-app:user"
	expectedDefaultKeyID := "layr-app-ed25519-v1"

	defaultJWTSigner, defaultSignerErr := NewJWTSigner(cryptoKeyManager)
	if defaultSignerErr != nil {
		t.Fatalf("failed to create default Signer: %v", defaultSignerErr)
	}
	if defaultJWTSigner.KeyID() != expectedDefaultKeyID {
		t.Fatalf("expected default key ID %s, got: %s", expectedDefaultKeyID, defaultJWTSigner.KeyID())
	}

	// 2. Verify custom key ID and fallback when empty string passed
	customKeyJWTSigner, customKeySignerErr := NewJWTSigner(cryptoKeyManager, "custom-key-v2")
	if customKeySignerErr != nil {
		t.Fatalf("failed to create Signer with custom key ID: %v", customKeySignerErr)
	}
	if customKeyJWTSigner.KeyID() != "custom-key-v2" {
		t.Fatalf("expected custom key ID custom-key-v2, got: %s", customKeyJWTSigner.KeyID())
	}

	fallbackKeyJWTSigner, fallbackKeySignerErr := NewJWTSigner(cryptoKeyManager, "")
	if fallbackKeySignerErr != nil {
		t.Fatalf("failed to create Signer with empty key ID: %v", fallbackKeySignerErr)
	}
	if fallbackKeyJWTSigner.KeyID() != expectedDefaultKeyID {
		t.Fatalf("expected fallback key ID %s, got: %s", expectedDefaultKeyID, fallbackKeyJWTSigner.KeyID())
	}

	// 3. Verify token issuance and verification with default audience/issuer
	testUserID := uuid.NewV7().String()
	defaultToken, defaultTokenErr := defaultJWTSigner.GenerateAccessToken(JWTClaims{
		Subject: testUserID,
		Email:   "user@example.com",
	}, 600)
	if defaultTokenErr != nil {
		t.Fatalf("failed to generate default token: %v", defaultTokenErr)
	}
	defaultVerifiedJWTClaims, verifyDefaultErr := defaultJWTSigner.VerifyAccessToken(defaultToken)
	if verifyDefaultErr != nil {
		t.Fatalf("failed to verify default token: %v", verifyDefaultErr)
	}
	if assertErr := defaultVerifiedJWTClaims.Assert("iss", expectedDefaultIssuer); assertErr != nil {
		t.Fatalf("failed default claims iss assert: %v", assertErr)
	}
	if assertErr := defaultVerifiedJWTClaims.Assert("aud", expectedDefaultAudience); assertErr != nil {
		t.Fatalf("failed default claims aud assert: %v", assertErr)
	}

	// 4. Verify token issuance and verification with custom audience and issuer parameters
	customAudience := "custom-audience:admin"
	customIssuer := "custom-auth-service"
	customToken, customTokenErr := defaultJWTSigner.GenerateAccessToken(JWTClaims{
		Subject:  testUserID,
		Email:    "admin@example.com",
		Role:     "admin",
		Audience: customAudience,
		Issuer:   customIssuer,
	}, 600)
	if customTokenErr != nil {
		t.Fatalf("failed to generate custom token: %v", customTokenErr)
	}
	customVerifiedJWTClaims, verifyCustomErr := defaultJWTSigner.VerifyAccessToken(customToken)
	if verifyCustomErr != nil {
		t.Fatalf("failed to verify custom token: %v", verifyCustomErr)
	}
	if assertErr := customVerifiedJWTClaims.Assert("aud", customAudience); assertErr != nil {
		t.Fatalf("failed custom aud assert: %v", assertErr)
	}
	if assertErr := customVerifiedJWTClaims.Assert("iss", customIssuer); assertErr != nil {
		t.Fatalf("failed custom iss assert: %v", assertErr)
	}

	// Mismatched expected audience or issuer must fail assertion
	if badAudienceErr := customVerifiedJWTClaims.Assert("aud", "wrong-audience"); badAudienceErr == nil {
		t.Fatal("expected error on mismatched audience")
	}
	if badIssuerErr := customVerifiedJWTClaims.Assert("iss", "wrong-issuer"); badIssuerErr == nil {
		t.Fatal("expected error on mismatched issuer")
	}

	// 5. Verify dynamic core config update via SetLoadedConfig
	t.Cleanup(func() {
		SetLoadedConfig(nil)
	})
	configuredProject := "Production Gateway"
	SetLoadedConfig(&Config{
		Project: ProjectConfig{
			Name: configuredProject,
		},
	})
	expectedConfiguredIssuer := "production-gateway"

	dynamicJWTSigner, dynamicSignerErr := NewJWTSigner(cryptoKeyManager)
	if dynamicSignerErr != nil {
		t.Fatalf("failed to create dynamic Signer: %v", dynamicSignerErr)
	}
	if dynamicJWTSigner.KeyID() != expectedConfiguredIssuer+"-ed25519-v1" {
		t.Fatalf("expected dynamic key ID %s-ed25519-v1, got: %s", expectedConfiguredIssuer, dynamicJWTSigner.KeyID())
	}
	dynamicToken, dynamicTokenErr := dynamicJWTSigner.GenerateAccessToken(JWTClaims{
		Subject: testUserID,
		Email:   "dyn@example.com",
	}, 600)
	if dynamicTokenErr != nil {
		t.Fatalf("failed to generate dynamic token: %v", dynamicTokenErr)
	}
	dynamicJWTClaims, verifyDynamicErr := dynamicJWTSigner.VerifyAccessToken(dynamicToken)
	if verifyDynamicErr != nil {
		t.Fatalf("failed to verify dynamic token: %v", verifyDynamicErr)
	}
	if assertErr := dynamicJWTClaims.Assert("iss", expectedConfiguredIssuer); assertErr != nil {
		t.Fatalf("failed dynamic claims iss assert: %v", assertErr)
	}
	if assertErr := dynamicJWTClaims.Assert("aud", expectedConfiguredIssuer+":user"); assertErr != nil {
		t.Fatalf("failed dynamic claims aud assert: %v", assertErr)
	}

	// 6. Verify empty project name fallback to "layr-app"
	SetLoadedConfig(&Config{
		Project: ProjectConfig{
			Name: "",
		},
	})
	emptyJWTSigner, emptySignerErr := NewJWTSigner(cryptoKeyManager)
	if emptySignerErr != nil {
		t.Fatalf("failed to create Signer with empty project config: %v", emptySignerErr)
	}
	if emptyJWTSigner.KeyID() != "layr-app-ed25519-v1" {
		t.Fatalf("expected fallback key ID layr-app-ed25519-v1, got: %s", emptyJWTSigner.KeyID())
	}
	emptyToken, emptyTokenErr := emptyJWTSigner.GenerateAccessToken(JWTClaims{Subject: "u1"})
	if emptyTokenErr != nil {
		t.Fatalf("failed to generate access token with empty config: %v", emptyTokenErr)
	}
	emptyVerifiedJWTClaims, verifyEmptyErr := emptyJWTSigner.VerifyAccessToken(emptyToken)
	if verifyEmptyErr != nil {
		t.Fatalf("failed to verify access token: %v", verifyEmptyErr)
	}
	if assertErr := emptyVerifiedJWTClaims.Assert("iss", "layr-app"); assertErr != nil {
		t.Fatalf("expected fallback issuer 'layr-app', got err: %v", assertErr)
	}
	emptyIDToken, emptyIDErr := emptyJWTSigner.GenerateIDToken(JWTClaims{Subject: "u1"})
	if emptyIDErr != nil {
		t.Fatalf("failed to generate id token with empty config: %v", emptyIDErr)
	}
	if emptyIDToken == "" {
		t.Fatal("expected non-empty id token")
	}

	emptyM2MToken, emptyM2MErr := emptyJWTSigner.GenerateM2MToken("sa-empty", nil, 3600, "layr:service_account")
	if emptyM2MErr != nil {
		t.Fatalf("failed to generate m2m token with empty config: %v", emptyM2MErr)
	}
	emptyVerifiedM2MJWTClaims, verifyEmptyM2MErr := emptyJWTSigner.VerifyM2MToken(emptyM2MToken)
	if verifyEmptyM2MErr != nil {
		t.Fatalf("failed to verify m2m token: %v", verifyEmptyM2MErr)
	}
	if emptyVerifiedM2MJWTClaims.Issuer != "layr" {
		t.Errorf("expected fallback issuer 'layr', got: %s", emptyVerifiedM2MJWTClaims.Issuer)
	}
	if emptyVerifiedM2MJWTClaims.Audience != "layr:service_account" {
		t.Errorf("expected fallback audience 'layr:service_account', got: %s", emptyVerifiedM2MJWTClaims.Audience)
	}

	SetLoadedConfig(nil)
}

func TestCoreJWTSignerAutomaticLoggingScopeUnit(t *testing.T) {
	var buffer bytes.Buffer
	log.SetOutput(&buffer)
	defer log.SetOutput(nil)

	log.SetLevel(logger.LevelDebug)
	defer log.ResetLevel()

	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	jwtSigner, err := NewJWTSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	token, err := jwtSigner.GenerateAccessToken(JWTClaims{Subject: "user-123"}, 300)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	output := buffer.String()
	expectedLog := "issued access token for subject user-123"
	if !strings.Contains(output, expectedLog) {
		t.Errorf("expected log output to contain %q, but got:\n%s", expectedLog, output)
	}

	buffer.Reset()
	_, err = jwtSigner.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("failed to verify access token: %v", err)
	}

	verifyOutput := buffer.String()
	expectedVerifyLog := "verified access token for subject user-123"
	if !strings.Contains(verifyOutput, expectedVerifyLog) {
		t.Errorf("expected log output to contain %q, but got:\n%s", expectedVerifyLog, verifyOutput)
	}
}

func TestCoreJWTM2MTokenSuccessUnit(t *testing.T) {
	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	jwtSigner, err := NewJWTSigner(cryptoKeyManager, "layr-ed25519-v1")
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	// 1. Default expiry and nil scopes
	serviceAccountID := uuid.NewV7().String()
	slugifier := NewSlugifier()
	expectedHandle := slugifier.Slugify(GetConfig().Project.Name)
	if expectedHandle == "" {
		expectedHandle = "layr"
	}
	expectedAudience := expectedHandle + ":service_account"
	defaultToken, err := jwtSigner.GenerateM2MToken(serviceAccountID, nil, 0, expectedAudience)
	if err != nil {
		t.Fatalf("failed to generate default M2M token: %v", err)
	}

	defaultM2MJWTClaims, err := jwtSigner.VerifyM2MToken(defaultToken)
	if err != nil {
		t.Fatalf("failed to verify default M2M token: %v", err)
	}
	if defaultM2MJWTClaims.Subject != serviceAccountID {
		t.Errorf("expected subject %s, got %s", serviceAccountID, defaultM2MJWTClaims.Subject)
	}
	if defaultM2MJWTClaims.Issuer != expectedHandle {
		t.Errorf("expected issuer %s, got %s", expectedHandle, defaultM2MJWTClaims.Issuer)
	}
	if defaultM2MJWTClaims.Audience != expectedAudience {
		t.Errorf("expected audience %s, got %s", expectedAudience, defaultM2MJWTClaims.Audience)
	}
	if len(defaultM2MJWTClaims.Scopes()) != 0 {
		t.Errorf("expected empty scopes, got %v", defaultM2MJWTClaims.Scopes())
	}
	if defaultM2MJWTClaims.JWTID == "" {
		t.Errorf("expected non-empty JWTID")
	}

	// 2. Custom expiry and custom scopes
	customScopes := []string{"data:read", "auth:admin"}
	customToken, err := jwtSigner.GenerateM2MToken(serviceAccountID, customScopes, 7200, "https://api.example.com")
	if err != nil {
		t.Fatalf("failed to generate custom M2M token: %v", err)
	}

	customM2MJWTClaims, err := jwtSigner.VerifyM2MToken(customToken)
	if err != nil {
		t.Fatalf("failed to verify custom M2M token: %v", err)
	}
	if customM2MJWTClaims.Audience != "https://api.example.com" {
		t.Errorf("expected audience https://api.example.com, got %s", customM2MJWTClaims.Audience)
	}
	if len(customM2MJWTClaims.Scopes()) != 2 || customM2MJWTClaims.Scopes()[0] != "data:read" || customM2MJWTClaims.Scopes()[1] != "auth:admin" {
		t.Errorf("expected scopes %v, got %v", customScopes, customM2MJWTClaims.Scopes())
	}
	if customM2MJWTClaims.Scope != "data:read auth:admin" {
		t.Errorf("expected scope 'data:read auth:admin', got %s", customM2MJWTClaims.Scope)
	}

	// 3. Test empty signer.keyID fallback
	emptyKeyIDJWTSigner := &JWTSigner{
		privateKey: jwtSigner.privateKey,
		publicKey:  jwtSigner.publicKey,
		seed:       jwtSigner.seed,
		keyID:      "",
	}
	fallbackToken, err := emptyKeyIDJWTSigner.GenerateM2MToken(serviceAccountID, customScopes, -1, "https://api.example.com")
	if err != nil {
		t.Fatalf("failed to generate fallback M2M token: %v", err)
	}
	tokenSegments := strings.Split(fallbackToken, ".")
	headerBytes, _ := base64.RawURLEncoding.DecodeString(tokenSegments[0])
	var headerMap map[string]string
	_ = json.Unmarshal(headerBytes, &headerMap)
	if headerMap["kid"] != "layr-ed25519-v1" {
		t.Errorf("expected fallback kid layr-ed25519-v1, got %s", headerMap["kid"])
	}

	// 4. Test Claims.Assert with "scope" and "scopes"
	scopesJWTClaims := JWTClaims{
		Subject: serviceAccountID,
		Scope:   strings.Join(customScopes, " "),
	}
	if assertScopesErr := scopesJWTClaims.Assert("scopes", customScopes); assertScopesErr != nil {
		t.Errorf("failed to assert scopes on Claims: %v", assertScopesErr)
	}
	if assertScopeErr := scopesJWTClaims.Assert("scope", "data:read auth:admin"); assertScopeErr != nil {
		t.Errorf("failed to assert scope on Claims: %v", assertScopeErr)
	}
}

func TestCoreJWTM2MTokenFailureUnit(t *testing.T) {
	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	jwtSigner, err := NewJWTSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	// 0. Empty service account ID and uninitialized private key
	if _, emptyIDErr := jwtSigner.GenerateM2MToken("", nil, 3600, "layr:service_account"); emptyIDErr == nil {
		t.Errorf("expected error on empty serviceAccountID")
	}
	emptyJWTSigner := &JWTSigner{}
	if _, uninitializedErr := emptyJWTSigner.GenerateM2MToken("sa-1", nil, 3600, "layr:service_account"); uninitializedErr == nil {
		t.Errorf("expected error on uninitialized signer")
	}

	// 1. Invalid segment count
	if _, segmentErr := jwtSigner.VerifyM2MToken("invalid.token"); segmentErr == nil {
		t.Errorf("expected error on invalid segments")
	}

	// 2. Invalid signature base64
	if _, sigBase64Err := jwtSigner.VerifyM2MToken("eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiIxIn0.!!invalid!!"); sigBase64Err == nil {
		t.Errorf("expected error on invalid signature base64")
	}

	// 3. Signature verification failure
	if _, sigVerifyErr := jwtSigner.VerifyM2MToken("eyJhbGciOiJFZERTQSJ9.e30.c2lnbmF0dXJl"); sigVerifyErr == nil {
		t.Errorf("expected error on signature mismatch")
	}

	// 4. Invalid claims base64
	validToken, _ := jwtSigner.GenerateM2MToken("sa-1", nil, 3600, "layr:service_account")
	segments := strings.Split(validToken, ".")
	invalidBase64SigningInput := segments[0] + ".!!invalid!!"
	invalidBase64Signature := ed25519.Sign(jwtSigner.privateKey, []byte(invalidBase64SigningInput))
	badClaimsToken := invalidBase64SigningInput + "." + base64.RawURLEncoding.EncodeToString(invalidBase64Signature)
	if _, claimsBase64Err := jwtSigner.VerifyM2MToken(badClaimsToken); claimsBase64Err == nil {
		t.Errorf("expected error on invalid claims base64")
	}

	// 5. Invalid claims JSON
	nonJSONClaims := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	nonJSONSigningInput := segments[0] + "." + nonJSONClaims
	signature := ed25519.Sign(jwtSigner.privateKey, []byte(nonJSONSigningInput))
	tamperedNonJSONToken := nonJSONSigningInput + "." + base64.RawURLEncoding.EncodeToString(signature)
	if _, jsonUnmarshalErr := jwtSigner.VerifyM2MToken(tamperedNonJSONToken); jsonUnmarshalErr == nil {
		t.Errorf("expected error on invalid claims JSON")
	}

	// 6. Expired token
	expiredM2MJWTClaims := JWTClaims{
		Subject:   "sa-expired",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour).Unix(),
	}
	expiredClaimsJSON, _ := json.Marshal(expiredM2MJWTClaims)
	expiredClaimsBase64 := base64.RawURLEncoding.EncodeToString(expiredClaimsJSON)
	expiredSigningInput := segments[0] + "." + expiredClaimsBase64
	expiredSignature := ed25519.Sign(jwtSigner.privateKey, []byte(expiredSigningInput))
	expiredToken := expiredSigningInput + "." + base64.RawURLEncoding.EncodeToString(expiredSignature)
	if _, expiredErr := jwtSigner.VerifyM2MToken(expiredToken); expiredErr == nil || !strings.Contains(expiredErr.Error(), "expired") {
		t.Errorf("expected token expired error, got %v", expiredErr)
	}
}

func TestCoreJWTSignerSignOutTokenUnit(t *testing.T) {
	UnloadConfig()
	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	jwtSigner, err := NewJWTSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	userID := uuid.NewV7().String()
	sessionID := uuid.NewV7().String()
	clientID := "client-test-123"

	// 1. Success: Generate and verify sign-out token with subject and sessionID
	signOutToken, generateErr := jwtSigner.GenerateSignOutToken(userID, sessionID, clientID, 60)
	if generateErr != nil {
		t.Fatalf("failed to generate sign-out token: %v", generateErr)
	}

	jwtClaims, verifyErr := jwtSigner.VerifySignOutToken(signOutToken)
	if verifyErr != nil {
		t.Fatalf("failed to verify sign-out token: %v", verifyErr)
	}

	if jwtClaims.Subject != userID || jwtClaims.SessionID != sessionID || jwtClaims.Audience != clientID {
		t.Fatalf("claims mismatch: %+v", jwtClaims)
	}
	if assertEventsErr := jwtClaims.Assert("events", SignOutTokenEventURI); assertEventsErr != nil {
		t.Fatalf("assert events failed: %v", assertEventsErr)
	}

	// 2. Success: Generate with only subject (no session ID)
	signOutTokenNoSession, generateNoSessionErr := jwtSigner.GenerateSignOutToken(userID, "", clientID)
	if generateNoSessionErr != nil {
		t.Fatalf("failed to generate sign-out token without session: %v", generateNoSessionErr)
	}
	noSessionJWTClaims, verifyNoSessionErr := jwtSigner.VerifySignOutToken(signOutTokenNoSession)
	if verifyNoSessionErr != nil {
		t.Fatalf("failed to verify sign-out token without session: %v", verifyNoSessionErr)
	}
	if noSessionJWTClaims.Subject != userID || noSessionJWTClaims.SessionID != "" {
		t.Fatalf("unexpected claims without session: %+v", noSessionJWTClaims)
	}

	// 3. Success: Generate with only session ID (no subject)
	signOutTokenNoSubject, generateNoSubjectErr := jwtSigner.GenerateSignOutToken("", sessionID, clientID)
	if generateNoSubjectErr != nil {
		t.Fatalf("failed to generate sign-out token without subject: %v", generateNoSubjectErr)
	}
	noSubjectJWTClaims, verifyNoSubjectErr := jwtSigner.VerifySignOutToken(signOutTokenNoSubject)
	if verifyNoSubjectErr != nil {
		t.Fatalf("failed to verify sign-out token without subject: %v", verifyNoSubjectErr)
	}
	if noSubjectJWTClaims.SessionID != sessionID || noSubjectJWTClaims.Subject != "" {
		t.Fatalf("unexpected claims without subject: %+v", noSubjectJWTClaims)
	}

	// 4. Failure: missing both subject and sessionID
	if _, missingSubjectAndSessionErr := jwtSigner.GenerateSignOutToken("", "", clientID); missingSubjectAndSessionErr == nil {
		t.Errorf("expected error when both subject and sessionID are empty")
	}

	// 5. Failure: missing clientID
	if _, missingClientErr := jwtSigner.GenerateSignOutToken(userID, sessionID, ""); missingClientErr == nil {
		t.Errorf("expected error when clientID is empty")
	}

	// 6. Verification: Invalid segment count
	if _, segmentErr := jwtSigner.VerifySignOutToken("invalid.token"); segmentErr == nil {
		t.Errorf("expected error on invalid segment count")
	}

	// 7. Verification: Invalid header base64
	if _, badHeaderBase64Err := jwtSigner.VerifySignOutToken("!!bad!!.eyJzdWIiOiIxIn0.c2ln"); badHeaderBase64Err == nil {
		t.Errorf("expected error on invalid header base64")
	}

	// 8. Verification: Invalid header JSON
	badHeaderJSON := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	if _, badHeaderJSONErr := jwtSigner.VerifySignOutToken(badHeaderJSON + ".eyJzdWIiOiIxIn0.c2ln"); badHeaderJSONErr == nil {
		t.Errorf("expected error on invalid header JSON")
	}

	// 9. Verification: Unsupported algorithm
	unsupportedAlgHeader, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "logout+jwt"})
	unsupportedAlgHeaderBase64 := base64.RawURLEncoding.EncodeToString(unsupportedAlgHeader)
	if _, unsupportedAlgErr := jwtSigner.VerifySignOutToken(unsupportedAlgHeaderBase64 + ".eyJzdWIiOiIxIn0.c2ln"); unsupportedAlgErr == nil {
		t.Errorf("expected error on unsupported algorithm")
	}

	// 10. Verification: Invalid claims base64
	segments := strings.Split(signOutToken, ".")
	if _, badClaimsBase64Err := jwtSigner.VerifySignOutToken(segments[0] + ".!!bad!!." + segments[2]); badClaimsBase64Err == nil {
		t.Errorf("expected error on invalid claims base64")
	}

	// 11. Verification: Invalid claims JSON
	badClaimsJSON := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	badClaimsSigningInput := segments[0] + "." + badClaimsJSON
	badClaimsSignature := ed25519.Sign(jwtSigner.privateKey, []byte(badClaimsSigningInput))
	badClaimsToken := badClaimsSigningInput + "." + base64.RawURLEncoding.EncodeToString(badClaimsSignature)
	if _, badClaimsJSONErr := jwtSigner.VerifySignOutToken(badClaimsToken); badClaimsJSONErr == nil {
		t.Errorf("expected error on invalid claims JSON")
	}

	// 12. Verification: Invalid signature base64
	if _, badSigBase64Err := jwtSigner.VerifySignOutToken(segments[0] + "." + segments[1] + ".!!bad!!"); badSigBase64Err == nil {
		t.Errorf("expected error on invalid signature base64")
	}

	// 13. Verification: Signature mismatch
	wrongSignature := base64.RawURLEncoding.EncodeToString([]byte("wrong-signature-data-of-length-64-bytes-needed-for-ed25519-validity!!"))
	if _, sigMismatchErr := jwtSigner.VerifySignOutToken(segments[0] + "." + segments[1] + "." + wrongSignature); sigMismatchErr == nil {
		t.Errorf("expected error on signature mismatch")
	}

	// 14. Verification: Nonce claim present (prohibited in logout/sign-out tokens)
	nonceJWTClaims := JWTClaims{
		Subject:  userID,
		Audience: clientID,
		Nonce:    "injected-nonce",
		Events:   map[string]any{SignOutTokenEventURI: map[string]any{}},
	}
	nonceClaimsJSON, _ := json.Marshal(nonceJWTClaims)
	nonceClaimsBase64 := base64.RawURLEncoding.EncodeToString(nonceClaimsJSON)
	nonceSigningInput := segments[0] + "." + nonceClaimsBase64
	nonceSignature := ed25519.Sign(jwtSigner.privateKey, []byte(nonceSigningInput))
	nonceToken := nonceSigningInput + "." + base64.RawURLEncoding.EncodeToString(nonceSignature)
	if _, nonceErr := jwtSigner.VerifySignOutToken(nonceToken); nonceErr == nil || !strings.Contains(nonceErr.Error(), "nonce") {
		t.Errorf("expected error rejecting nonce claim, got %v", nonceErr)
	}

	// 15. Verification: Missing both subject and sessionID in claims
	emptySubjectJWTClaims := JWTClaims{
		Audience: clientID,
		Events:   map[string]any{SignOutTokenEventURI: map[string]any{}},
	}
	emptySubjectJSON, _ := json.Marshal(emptySubjectJWTClaims)
	emptySubjectBase64 := base64.RawURLEncoding.EncodeToString(emptySubjectJSON)
	emptySubjectSigningInput := segments[0] + "." + emptySubjectBase64
	emptySubjectSignature := ed25519.Sign(jwtSigner.privateKey, []byte(emptySubjectSigningInput))
	emptySubjectToken := emptySubjectSigningInput + "." + base64.RawURLEncoding.EncodeToString(emptySubjectSignature)
	if _, emptySubErr := jwtSigner.VerifySignOutToken(emptySubjectToken); emptySubErr == nil || !strings.Contains(emptySubErr.Error(), "sub or sid") {
		t.Errorf("expected error on missing sub and sid, got %v", emptySubErr)
	}

	// 16. Verification: Missing events claim
	noEventsJWTClaims := JWTClaims{
		Subject:  userID,
		Audience: clientID,
	}
	noEventsJSON, _ := json.Marshal(noEventsJWTClaims)
	noEventsBase64 := base64.RawURLEncoding.EncodeToString(noEventsJSON)
	noEventsSigningInput := segments[0] + "." + noEventsBase64
	noEventsSignature := ed25519.Sign(jwtSigner.privateKey, []byte(noEventsSigningInput))
	noEventsToken := noEventsSigningInput + "." + base64.RawURLEncoding.EncodeToString(noEventsSignature)
	if _, noEventsErr := jwtSigner.VerifySignOutToken(noEventsToken); noEventsErr == nil || !strings.Contains(noEventsErr.Error(), "missing events") {
		t.Errorf("expected error on missing events claim, got %v", noEventsErr)
	}

	// 17. Verification: Events claim missing backchannel sign-out URI
	wrongEventJWTClaims := JWTClaims{
		Subject:  userID,
		Audience: clientID,
		Events:   map[string]any{"http://schemas.openid.net/event/other": map[string]any{}},
	}
	wrongEventJSON, _ := json.Marshal(wrongEventJWTClaims)
	wrongEventBase64 := base64.RawURLEncoding.EncodeToString(wrongEventJSON)
	wrongEventSigningInput := segments[0] + "." + wrongEventBase64
	wrongEventSignature := ed25519.Sign(jwtSigner.privateKey, []byte(wrongEventSigningInput))
	wrongEventToken := wrongEventSigningInput + "." + base64.RawURLEncoding.EncodeToString(wrongEventSignature)
	if _, wrongEventErr := jwtSigner.VerifySignOutToken(wrongEventToken); wrongEventErr == nil || !strings.Contains(wrongEventErr.Error(), "missing backchannel") {
		t.Errorf("expected error on wrong event URI, got %v", wrongEventErr)
	}

	// 18. Verification: Expired sign-out token
	expiredJWTClaims := JWTClaims{
		Subject:   userID,
		Audience:  clientID,
		ExpiresAt: time.Now().UTC().Add(-1 * time.Minute).Unix(),
		Events:    map[string]any{SignOutTokenEventURI: map[string]any{}},
	}
	expiredJSON, _ := json.Marshal(expiredJWTClaims)
	expiredBase64 := base64.RawURLEncoding.EncodeToString(expiredJSON)
	expiredSigningInput := segments[0] + "." + expiredBase64
	expiredSignature := ed25519.Sign(jwtSigner.privateKey, []byte(expiredSigningInput))
	expiredSignOutToken := expiredSigningInput + "." + base64.RawURLEncoding.EncodeToString(expiredSignature)
	if _, expiredErr := jwtSigner.VerifySignOutToken(expiredSignOutToken); expiredErr == nil || !strings.Contains(expiredErr.Error(), "expired") {
		t.Errorf("expected error on expired sign-out token, got %v", expiredErr)
	}
}
