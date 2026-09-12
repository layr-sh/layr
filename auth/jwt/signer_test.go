package jwt

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"uuid"

	"layr.sh/core"
	"layr.sh/logger"
)

func TestJWTSignerAndVerifierUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	signer, err := NewSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	if len(signer.PublicKey()) == 0 {
		t.Fatalf("expected non-empty public key")
	}

	userID := uuid.NewV7().String()
	token, err := signer.GenerateAccessToken(Claims{
		Subject: userID,
		Email:   "user@example.com",
		Phone:   "+123456789",
		Claims:  map[string]any{"plan": "pro", "level": 3, "active": true},
	}, 900)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	claims, err := signer.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("failed to verify access token: %v", err)
	}

	if claims.Subject != userID || claims.Email != "user@example.com" || claims.Role != "authenticated" || claims.IsAnonymous {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.Claims["plan"] != "pro" {
		t.Fatalf("expected claims.Claims['plan'] == 'pro', got: %v", claims.Claims["plan"])
	}

	// Test single Assert with standard aliases and custom claims
	if assertErr := claims.Assert("sub", userID); assertErr != nil {
		t.Fatalf("assert sub failed: %v", assertErr)
	}
	if assertErr := claims.Assert("subject", userID); assertErr != nil {
		t.Fatalf("assert subject alias failed: %v", assertErr)
	}
	if assertErr := claims.Assert("aud", "layr-app:user"); assertErr != nil {
		t.Fatalf("assert aud failed: %v", assertErr)
	}
	if assertErr := claims.Assert("audience", "layr-app:user"); assertErr != nil {
		t.Fatalf("assert audience alias failed: %v", assertErr)
	}
	if assertErr := claims.Assert("iss", "layr-app"); assertErr != nil {
		t.Fatalf("assert iss failed: %v", assertErr)
	}
	if assertErr := claims.Assert("issuer", "layr-app"); assertErr != nil {
		t.Fatalf("assert issuer alias failed: %v", assertErr)
	}
	if assertErr := claims.Assert("role", "authenticated"); assertErr != nil {
		t.Fatalf("assert role failed: %v", assertErr)
	}
	if assertErr := claims.Assert("email", "user@example.com"); assertErr != nil {
		t.Fatalf("assert email failed: %v", assertErr)
	}
	if assertErr := claims.Assert("phone", "+123456789"); assertErr != nil {
		t.Fatalf("assert phone failed: %v", assertErr)
	}
	if assertErr := claims.Assert("is_anonymous", false); assertErr != nil {
		t.Fatalf("assert is_anonymous failed: %v", assertErr)
	}
	if assertErr := claims.Assert("jti", claims.JWTID); assertErr != nil {
		t.Fatalf("assert jti failed: %v", assertErr)
	}
	if assertErr := claims.Assert("plan", "pro"); assertErr != nil {
		t.Fatalf("assert plan failed: %v", assertErr)
	}
	if assertErr := claims.Assert("level", 3); assertErr != nil {
		t.Fatalf("assert numeric level failed: %v", assertErr)
	}
	if assertErr := claims.Assert("level", int64(3)); assertErr != nil {
		t.Fatalf("assert int64 level failed: %v", assertErr)
	}
	if assertErr := claims.Assert("level", float64(3)); assertErr != nil {
		t.Fatalf("assert float64 level failed: %v", assertErr)
	}
	if assertErr := claims.Assert("active", true); assertErr != nil {
		t.Fatalf("assert bool active failed: %v", assertErr)
	}

	// Test Assert failure on mismatch and missing claim
	if assertErr := claims.Assert("role", "admin"); assertErr == nil {
		t.Fatal("expected assertion error on role mismatch")
	}
	if assertErr := claims.Assert("non_existent_key", "value"); assertErr == nil {
		t.Fatal("expected assertion error on missing custom claim")
	}
	if assertErr := claims.Assert("active", 123); assertErr == nil {
		t.Fatal("expected assertion error when comparing bool with number")
	}

	// Anonymous token verification
	anonymousToken, err := signer.GenerateAccessToken(Claims{
		Subject:     userID,
		IsAnonymous: true,
	}, 900)
	if err != nil {
		t.Fatalf("failed to generate anonymous token: %v", err)
	}
	anonymousClaims, err := signer.VerifyAccessToken(anonymousToken)
	if err != nil || !anonymousClaims.IsAnonymous {
		t.Fatalf("expected anonymous claims with IsAnonymous=true, got: %+v", anonymousClaims)
	}
	if assertErr := anonymousClaims.Assert("is_anonymous", true); assertErr != nil {
		t.Fatalf("assert is_anonymous failed: %v", assertErr)
	}
	// Test Assert on nil Claims map
	if assertErr := anonymousClaims.Assert("any_custom_key", "val"); assertErr == nil {
		t.Fatal("expected error on nil claims map")
	}

	// Fallback role and expiry
	fallbackToken, err := signer.GenerateAccessToken(Claims{
		Subject: userID,
	})
	if err != nil {
		t.Fatalf("failed to generate fallback token: %v", err)
	}
	fallbackClaims, err := signer.VerifyAccessToken(fallbackToken)
	if err != nil || fallbackClaims.Role != "authenticated" {
		t.Fatalf("expected authenticated role, got: %+v", fallbackClaims)
	}

	// Malformed tokens
	if _, verifySegmentErr := signer.VerifyAccessToken("not.three.segments.extra"); verifySegmentErr == nil {
		t.Fatalf("expected error on 4-part token")
	}
	if _, verifyBase64Err := signer.VerifyAccessToken("eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiIxIn0.!!invalid-signature-base64!!"); verifyBase64Err == nil {
		t.Fatalf("expected error on bad signature base64")
	}
	if _, verifySigErr := signer.VerifyAccessToken("eyJhbGciOiJFZERTQSJ9.e30.c2ln"); verifySigErr == nil {
		t.Fatalf("expected signature verification failure")
	}

	// Valid signature but invalid claims base64
	badClaimsPayload := "eyJhbGciOiJFZERTQSJ9.!bad-b64!"
	badSignature := ed25519.Sign(signer.privateKey, []byte(badClaimsPayload))
	badClaimsToken := badClaimsPayload + "." + base64.RawURLEncoding.EncodeToString(badSignature)
	if _, verifyClaimsDecodeErr := signer.VerifyAccessToken(badClaimsToken); verifyClaimsDecodeErr == nil {
		t.Fatalf("expected error on bad claims base64")
	}

	// Valid signature but non-JSON claims
	nonJSONClaimsPayload := "eyJhbGciOiJFZERTQSJ9.bm90LWpzb24"
	nonJSONSignature := ed25519.Sign(signer.privateKey, []byte(nonJSONClaimsPayload))
	nonJSONToken := nonJSONClaimsPayload + "." + base64.RawURLEncoding.EncodeToString(nonJSONSignature)
	if _, verifyJSONErr := signer.VerifyAccessToken(nonJSONToken); verifyJSONErr == nil {
		t.Fatalf("expected error on non-json claims")
	}

	// Expired token test
	expiredToken, _ := signer.GenerateAccessToken(Claims{
		Subject: userID,
		Email:   "exp@example.com",
	}, -10)
	if _, verifyExpiredErr := signer.VerifyAccessToken(expiredToken); verifyExpiredErr == nil || !strings.Contains(verifyExpiredErr.Error(), "expired") {
		t.Fatalf("expected expired error, got: %v", verifyExpiredErr)
	}

	// Future NotBefore token
	futureClaims := Claims{
		Subject:   userID,
		Issuer:    "layr-app",
		Audience:  "layr-app:user",
		IssuedAt:  time.Now().Unix() + 100,
		NotBefore: time.Now().Unix() + 100,
		ExpiresAt: time.Now().Unix() + 1000,
	}
	futureJSON, _ := json.Marshal(futureClaims)
	headerBase64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	futureClaimsBase64 := base64.RawURLEncoding.EncodeToString(futureJSON)
	futureSignature := ed25519.Sign(signer.privateKey, []byte(headerBase64+"."+futureClaimsBase64))
	futureToken := headerBase64 + "." + futureClaimsBase64 + "." + base64.RawURLEncoding.EncodeToString(futureSignature)
	if _, verifyFutureErr := signer.VerifyAccessToken(futureToken); verifyFutureErr == nil || !strings.Contains(verifyFutureErr.Error(), "not valid yet") {
		t.Fatalf("expected not valid yet error, got: %v", verifyFutureErr)
	}

	// Audience mismatch
	badAudienceClaims := futureClaims
	badAudienceClaims.NotBefore = time.Now().Unix() - 10
	badAudienceClaims.Audience = "wrong:audience"
	badAudienceJSON, _ := json.Marshal(badAudienceClaims)
	badAudienceClaimsBase64 := base64.RawURLEncoding.EncodeToString(badAudienceJSON)
	badAudienceSignature := ed25519.Sign(signer.privateKey, []byte(headerBase64+"."+badAudienceClaimsBase64))
	badAudienceToken := headerBase64 + "." + badAudienceClaimsBase64 + "." + base64.RawURLEncoding.EncodeToString(badAudienceSignature)
	verifiedBadAudienceClaims, verifyAudienceErr := signer.VerifyAccessToken(badAudienceToken)
	if verifyAudienceErr != nil {
		t.Fatalf("expected signature verification to pass, got: %v", verifyAudienceErr)
	}
	if assertErr := verifiedBadAudienceClaims.Assert("aud", "layr-app:user"); assertErr == nil || !strings.Contains(assertErr.Error(), "mismatch") {
		t.Fatalf("expected audience mismatch error, got: %v", assertErr)
	}

	// Issuer mismatch
	badIssuerClaims := futureClaims
	badIssuerClaims.NotBefore = time.Now().Unix() - 10
	badIssuerClaims.Issuer = "wrong:issuer"
	badIssuerJSON, _ := json.Marshal(badIssuerClaims)
	badIssuerClaimsBase64 := base64.RawURLEncoding.EncodeToString(badIssuerJSON)
	badIssuerSignature := ed25519.Sign(signer.privateKey, []byte(headerBase64+"."+badIssuerClaimsBase64))
	badIssuerToken := headerBase64 + "." + badIssuerClaimsBase64 + "." + base64.RawURLEncoding.EncodeToString(badIssuerSignature)
	verifiedBadIssuerClaims, verifyIssuerErr := signer.VerifyAccessToken(badIssuerToken)
	if verifyIssuerErr != nil {
		t.Fatalf("expected signature verification to pass, got: %v", verifyIssuerErr)
	}
	if assertErr := verifiedBadIssuerClaims.Assert("iss", "layr-app"); assertErr == nil || !strings.Contains(assertErr.Error(), "mismatch") {
		t.Fatalf("expected issuer mismatch error, got: %v", assertErr)
	}
}

func TestJWTRefreshTokenAndHMACUnit(t *testing.T) {
	refreshToken := GenerateRefreshToken()
	if len(refreshToken) != 64 {
		t.Fatalf("expected 64-char hex refresh token, got: %s (len: %d)", refreshToken, len(refreshToken))
	}
	refreshTokenHash := HashRefreshToken(refreshToken)
	if len(refreshTokenHash) != 64 {
		t.Fatalf("expected 64-char sha256 hex, got: %s", refreshTokenHash)
	}

	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}
	signer, err := NewSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	message := "test-hmac-payload"
	hmacSignature := signer.SignHMAC(message)
	if !signer.VerifyHMAC(message, hmacSignature) {
		t.Fatalf("expected HMAC verification to pass")
	}
	if signer.VerifyHMAC(message, "invalid-signature") {
		t.Fatalf("expected HMAC verification to fail on bad signature")
	}
}

func TestJWTGenerateIDTokenAndOIDCDiscoveryUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}
	signer, err := NewSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	userID := uuid.NewV7().String()
	idToken, tokenGenerateErr := signer.GenerateIDToken(OIDCIDTokenClaims{
		Issuer:        "https://auth.example.com",
		Audience:      "client-123",
		Subject:       userID,
		Email:         "test@example.com",
		EmailVerified: true,
		PhoneNumber:   "+1234567890",
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

	var oidcIDTokenClaims OIDCIDTokenClaims
	if unmarshalErr := json.Unmarshal(claimsBytes, &oidcIDTokenClaims); unmarshalErr != nil {
		t.Fatalf("failed to parse claims JSON: %v", unmarshalErr)
	}

	if oidcIDTokenClaims.Issuer != "https://auth.example.com" {
		t.Errorf("expected issuer https://auth.example.com, got %s", oidcIDTokenClaims.Issuer)
	}
	if oidcIDTokenClaims.Audience != "client-123" {
		t.Errorf("expected audience client-123, got %s", oidcIDTokenClaims.Audience)
	}
	if oidcIDTokenClaims.Subject != userID {
		t.Errorf("expected subject %s, got %s", userID, oidcIDTokenClaims.Subject)
	}
	if oidcIDTokenClaims.Email != "test@example.com" || !oidcIDTokenClaims.EmailVerified {
		t.Errorf("unexpected email claims: %s, %v", oidcIDTokenClaims.Email, oidcIDTokenClaims.EmailVerified)
	}
	if oidcIDTokenClaims.Nonce != "nonce-xyz" {
		t.Errorf("expected nonce nonce-xyz, got %s", oidcIDTokenClaims.Nonce)
	}

	// Test GenerateIDToken with empty role, empty issuer (fallback to core.GetConfig().Project.Slug()), and expiry 0
	defaultIDToken, defaultTokenGenerateErr := signer.GenerateIDToken(OIDCIDTokenClaims{
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
	defaultOIDCIDTokenClaimsBytes, defaultClaimsDecodeErr := base64.RawURLEncoding.DecodeString(defaultSegments[1])
	if defaultClaimsDecodeErr != nil {
		t.Fatalf("failed to decode default claims: %v", defaultClaimsDecodeErr)
	}
	var defaultOIDCIDTokenClaims OIDCIDTokenClaims
	if defaultUnmarshalErr := json.Unmarshal(defaultOIDCIDTokenClaimsBytes, &defaultOIDCIDTokenClaims); defaultUnmarshalErr != nil {
		t.Fatalf("failed to unmarshal default claims: %v", defaultUnmarshalErr)
	}
	if defaultOIDCIDTokenClaims.Issuer != "layr-app" {
		t.Fatalf("expected fallback issuer 'layr-app', got: %s", defaultOIDCIDTokenClaims.Issuer)
	}

	// Verify discovery
	oidcConfiguration := BuildOIDCDiscovery("https://auth.example.com")
	if oidcConfiguration.Issuer != "https://auth.example.com" {
		t.Errorf("expected discovery issuer https://auth.example.com, got %s", oidcConfiguration.Issuer)
	}
	if oidcConfiguration.EndSessionEndpoint != "https://auth.example.com/api/v1/auth/oauth/sign-out" {
		t.Errorf("expected sign-out end session endpoint, got %s", oidcConfiguration.EndSessionEndpoint)
	}
	if len(oidcConfiguration.GrantTypesSupported) == 0 || oidcConfiguration.GrantTypesSupported[0] != "authorization_code" {
		t.Errorf("unexpected grant types: %v", oidcConfiguration.GrantTypesSupported)
	}
}

func TestJWTProjectHandleConfigurationUnit(t *testing.T) {
	cryptoKeyManager, managerErr := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if managerErr != nil {
		t.Fatalf("failed to create KeyManager: %v", managerErr)
	}

	// 1. Verify default values derived from core default config ("layr-app")
	expectedDefaultIssuer := "layr-app"
	expectedDefaultAudience := "layr-app:user"
	expectedDefaultKeyID := "layr-app-ed25519-v1"

	defaultSigner, defaultSignerErr := NewSigner(cryptoKeyManager)
	if defaultSignerErr != nil {
		t.Fatalf("failed to create default Signer: %v", defaultSignerErr)
	}
	if defaultSigner.KeyID() != expectedDefaultKeyID {
		t.Fatalf("expected default key ID %s, got: %s", expectedDefaultKeyID, defaultSigner.KeyID())
	}

	// 2. Verify custom key ID and fallback when empty string passed
	customKeySigner, customKeySignerErr := NewSigner(cryptoKeyManager, "custom-key-v2")
	if customKeySignerErr != nil {
		t.Fatalf("failed to create Signer with custom key ID: %v", customKeySignerErr)
	}
	if customKeySigner.KeyID() != "custom-key-v2" {
		t.Fatalf("expected custom key ID custom-key-v2, got: %s", customKeySigner.KeyID())
	}

	fallbackKeySigner, fallbackKeySignerErr := NewSigner(cryptoKeyManager, "")
	if fallbackKeySignerErr != nil {
		t.Fatalf("failed to create Signer with empty key ID: %v", fallbackKeySignerErr)
	}
	if fallbackKeySigner.KeyID() != expectedDefaultKeyID {
		t.Fatalf("expected fallback key ID %s, got: %s", expectedDefaultKeyID, fallbackKeySigner.KeyID())
	}

	// 3. Verify token issuance and verification with default audience/issuer
	testUserID := uuid.NewV7().String()
	defaultToken, defaultTokenErr := defaultSigner.GenerateAccessToken(Claims{
		Subject: testUserID,
		Email:   "user@example.com",
	}, 600)
	if defaultTokenErr != nil {
		t.Fatalf("failed to generate default token: %v", defaultTokenErr)
	}
	defaultVerifiedClaims, verifyDefaultErr := defaultSigner.VerifyAccessToken(defaultToken)
	if verifyDefaultErr != nil {
		t.Fatalf("failed to verify default token: %v", verifyDefaultErr)
	}
	if assertErr := defaultVerifiedClaims.Assert("iss", expectedDefaultIssuer); assertErr != nil {
		t.Fatalf("failed default claims iss assert: %v", assertErr)
	}
	if assertErr := defaultVerifiedClaims.Assert("aud", expectedDefaultAudience); assertErr != nil {
		t.Fatalf("failed default claims aud assert: %v", assertErr)
	}

	// 4. Verify token issuance and verification with custom audience and issuer parameters
	customAudience := "custom-audience:admin"
	customIssuer := "custom-auth-service"
	customToken, customTokenErr := defaultSigner.GenerateAccessToken(Claims{
		Subject:  testUserID,
		Email:    "admin@example.com",
		Role:     "admin",
		Audience: customAudience,
		Issuer:   customIssuer,
	}, 600)
	if customTokenErr != nil {
		t.Fatalf("failed to generate custom token: %v", customTokenErr)
	}
	customVerifiedClaims, verifyCustomErr := defaultSigner.VerifyAccessToken(customToken)
	if verifyCustomErr != nil {
		t.Fatalf("failed to verify custom token: %v", verifyCustomErr)
	}
	if assertErr := customVerifiedClaims.Assert("aud", customAudience); assertErr != nil {
		t.Fatalf("failed custom aud assert: %v", assertErr)
	}
	if assertErr := customVerifiedClaims.Assert("iss", customIssuer); assertErr != nil {
		t.Fatalf("failed custom iss assert: %v", assertErr)
	}

	// Mismatched expected audience or issuer must fail assertion
	if badAudienceErr := customVerifiedClaims.Assert("aud", "wrong-audience"); badAudienceErr == nil {
		t.Fatal("expected error on mismatched audience")
	}
	if badIssuerErr := customVerifiedClaims.Assert("iss", "wrong-issuer"); badIssuerErr == nil {
		t.Fatal("expected error on mismatched issuer")
	}

	// 5. Verify dynamic core config update via SetLoadedConfig
	t.Cleanup(func() {
		core.SetLoadedConfig(nil)
	})
	configuredProject := "Production Gateway"
	core.SetLoadedConfig(&core.Config{
		Project: core.ProjectConfig{
			Name: configuredProject,
		},
	})
	expectedConfiguredIssuer := "production-gateway"

	dynamicSigner, dynamicSignerErr := NewSigner(cryptoKeyManager)
	if dynamicSignerErr != nil {
		t.Fatalf("failed to create dynamic Signer: %v", dynamicSignerErr)
	}
	if dynamicSigner.KeyID() != expectedConfiguredIssuer+"-ed25519-v1" {
		t.Fatalf("expected dynamic key ID %s-ed25519-v1, got: %s", expectedConfiguredIssuer, dynamicSigner.KeyID())
	}
	dynamicToken, dynamicTokenErr := dynamicSigner.GenerateAccessToken(Claims{
		Subject: testUserID,
		Email:   "dyn@example.com",
	}, 600)
	if dynamicTokenErr != nil {
		t.Fatalf("failed to generate dynamic token: %v", dynamicTokenErr)
	}
	dynamicClaims, verifyDynamicErr := dynamicSigner.VerifyAccessToken(dynamicToken)
	if verifyDynamicErr != nil {
		t.Fatalf("failed to verify dynamic token: %v", verifyDynamicErr)
	}
	if assertErr := dynamicClaims.Assert("iss", expectedConfiguredIssuer); assertErr != nil {
		t.Fatalf("failed dynamic claims iss assert: %v", assertErr)
	}
	if assertErr := dynamicClaims.Assert("aud", expectedConfiguredIssuer+":user"); assertErr != nil {
		t.Fatalf("failed dynamic claims aud assert: %v", assertErr)
	}

	// 6. Verify empty project name fallback to "layr-app"
	core.SetLoadedConfig(&core.Config{
		Project: core.ProjectConfig{
			Name: "",
		},
	})
	emptySigner, emptySignerErr := NewSigner(cryptoKeyManager)
	if emptySignerErr != nil {
		t.Fatalf("failed to create Signer with empty project config: %v", emptySignerErr)
	}
	if emptySigner.KeyID() != "layr-app-ed25519-v1" {
		t.Fatalf("expected fallback key ID layr-app-ed25519-v1, got: %s", emptySigner.KeyID())
	}
	emptyToken, emptyTokenErr := emptySigner.GenerateAccessToken(Claims{Subject: "u1"})
	if emptyTokenErr != nil {
		t.Fatalf("failed to generate access token with empty config: %v", emptyTokenErr)
	}
	emptyVerifiedClaims, verifyEmptyErr := emptySigner.VerifyAccessToken(emptyToken)
	if verifyEmptyErr != nil {
		t.Fatalf("failed to verify access token: %v", verifyEmptyErr)
	}
	if assertErr := emptyVerifiedClaims.Assert("iss", "layr-app"); assertErr != nil {
		t.Fatalf("expected fallback issuer 'layr-app', got err: %v", assertErr)
	}
	emptyIDToken, emptyIDErr := emptySigner.GenerateIDToken(OIDCIDTokenClaims{Subject: "u1"})
	if emptyIDErr != nil {
		t.Fatalf("failed to generate id token with empty config: %v", emptyIDErr)
	}
	if emptyIDToken == "" {
		t.Fatal("expected non-empty id token")
	}

	emptyM2MToken, emptyM2MErr := emptySigner.GenerateM2MToken("sa-empty", nil, 3600)
	if emptyM2MErr != nil {
		t.Fatalf("failed to generate m2m token with empty config: %v", emptyM2MErr)
	}
	emptyVerifiedM2MClaims, verifyEmptyM2MErr := emptySigner.VerifyM2MToken(emptyM2MToken)
	if verifyEmptyM2MErr != nil {
		t.Fatalf("failed to verify m2m token: %v", verifyEmptyM2MErr)
	}
	if emptyVerifiedM2MClaims.Issuer != "layr" {
		t.Errorf("expected fallback issuer 'layr', got: %s", emptyVerifiedM2MClaims.Issuer)
	}
	if emptyVerifiedM2MClaims.Audience != "layr:service_account" {
		t.Errorf("expected fallback audience 'layr:service_account', got: %s", emptyVerifiedM2MClaims.Audience)
	}

	core.SetLoadedConfig(nil)
}

func TestJWTSignerAutomaticLoggingScopeUnit(t *testing.T) {
	var buffer bytes.Buffer
	log.SetOutput(&buffer)
	defer log.SetOutput(nil)

	log.SetLevel(logger.LevelDebug)
	defer log.ResetLevel()

	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	signer, err := NewSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	token, err := signer.GenerateAccessToken(Claims{Subject: "user-123"}, 300)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	output := buffer.String()
	expectedLog := "[DEBUG] [auth] issued access token for subject user-123"
	if !strings.Contains(output, expectedLog) {
		t.Errorf("expected log output to contain %q, but got:\n%s", expectedLog, output)
	}

	buffer.Reset()
	_, err = signer.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("failed to verify access token: %v", err)
	}

	verifyOutput := buffer.String()
	expectedVerifyLog := "[DEBUG] [auth] verified access token for subject user-123"
	if !strings.Contains(verifyOutput, expectedVerifyLog) {
		t.Errorf("expected log output to contain %q, but got:\n%s", expectedVerifyLog, verifyOutput)
	}
}

func TestJWTM2MTokenSuccessUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	signer, err := NewSigner(cryptoKeyManager, "layr-ed25519-v1")
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	// 1. Default expiry and nil scopes
	serviceAccountID := uuid.NewV7().String()
	defaultToken, err := signer.GenerateM2MToken(serviceAccountID, nil, 0)
	if err != nil {
		t.Fatalf("failed to generate default M2M token: %v", err)
	}

	defaultM2MClaims, err := signer.VerifyM2MToken(defaultToken)
	if err != nil {
		t.Fatalf("failed to verify default M2M token: %v", err)
	}
	if defaultM2MClaims.Subject != serviceAccountID {
		t.Errorf("expected subject %s, got %s", serviceAccountID, defaultM2MClaims.Subject)
	}
	slugifier := core.NewSlugifier()
	expectedHandle := slugifier.Slugify(core.GetConfig().Project.Name)
	if expectedHandle == "" {
		expectedHandle = "layr"
	}
	if defaultM2MClaims.Issuer != expectedHandle {
		t.Errorf("expected issuer %s, got %s", expectedHandle, defaultM2MClaims.Issuer)
	}
	if defaultM2MClaims.Audience != expectedHandle+":service_account" {
		t.Errorf("expected audience %s:service_account, got %s", expectedHandle, defaultM2MClaims.Audience)
	}
	if len(defaultM2MClaims.Scopes) != 0 {
		t.Errorf("expected empty scopes, got %v", defaultM2MClaims.Scopes)
	}
	if defaultM2MClaims.JWTID == "" {
		t.Errorf("expected non-empty JWTID")
	}

	// 2. Custom expiry and custom scopes
	customScopes := []string{"data:read", "auth:admin"}
	customToken, err := signer.GenerateM2MToken(serviceAccountID, customScopes, 7200)
	if err != nil {
		t.Fatalf("failed to generate custom M2M token: %v", err)
	}

	customM2MClaims, err := signer.VerifyM2MToken(customToken)
	if err != nil {
		t.Fatalf("failed to verify custom M2M token: %v", err)
	}
	if len(customM2MClaims.Scopes) != 2 || customM2MClaims.Scopes[0] != "data:read" || customM2MClaims.Scopes[1] != "auth:admin" {
		t.Errorf("expected scopes %v, got %v", customScopes, customM2MClaims.Scopes)
	}

	// 3. Test empty signer.keyID fallback
	emptyKeyIDSigner := &Signer{
		privateKey: signer.privateKey,
		publicKey:  signer.publicKey,
		seed:       signer.seed,
		keyID:      "",
	}
	fallbackToken, err := emptyKeyIDSigner.GenerateM2MToken(serviceAccountID, customScopes, -1)
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

	// 4. Test Claims.Assert with "scopes"
	scopesClaims := Claims{
		Subject: serviceAccountID,
		Scopes:  customScopes,
	}
	if assertScopesErr := scopesClaims.Assert("scopes", customScopes); assertScopesErr != nil {
		t.Errorf("failed to assert scopes on Claims: %v", assertScopesErr)
	}
}

func TestJWTM2MTokenFailureUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	signer, err := NewSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	// 0. Empty service account ID and uninitialized private key
	if _, emptyIDErr := signer.GenerateM2MToken("", nil, 3600); emptyIDErr == nil {
		t.Errorf("expected error on empty serviceAccountID")
	}
	emptySigner := &Signer{}
	if _, uninitializedErr := emptySigner.GenerateM2MToken("sa-1", nil, 3600); uninitializedErr == nil {
		t.Errorf("expected error on uninitialized signer")
	}

	// 1. Invalid segment count
	if _, segmentErr := signer.VerifyM2MToken("invalid.token"); segmentErr == nil {
		t.Errorf("expected error on invalid segments")
	}

	// 2. Invalid signature base64
	if _, sigBase64Err := signer.VerifyM2MToken("eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiIxIn0.!!invalid!!"); sigBase64Err == nil {
		t.Errorf("expected error on invalid signature base64")
	}

	// 3. Signature verification failure
	if _, sigVerifyErr := signer.VerifyM2MToken("eyJhbGciOiJFZERTQSJ9.e30.c2lnbmF0dXJl"); sigVerifyErr == nil {
		t.Errorf("expected error on signature mismatch")
	}

	// 4. Invalid claims base64
	validToken, _ := signer.GenerateM2MToken("sa-1", nil, 3600)
	segments := strings.Split(validToken, ".")
	invalidBase64SigningInput := segments[0] + ".!!invalid!!"
	invalidBase64Signature := ed25519.Sign(signer.privateKey, []byte(invalidBase64SigningInput))
	badClaimsToken := invalidBase64SigningInput + "." + base64.RawURLEncoding.EncodeToString(invalidBase64Signature)
	if _, claimsBase64Err := signer.VerifyM2MToken(badClaimsToken); claimsBase64Err == nil {
		t.Errorf("expected error on invalid claims base64")
	}

	// 5. Invalid claims JSON
	nonJSONClaims := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	nonJSONSigningInput := segments[0] + "." + nonJSONClaims
	signature := ed25519.Sign(signer.privateKey, []byte(nonJSONSigningInput))
	tamperedNonJSONToken := nonJSONSigningInput + "." + base64.RawURLEncoding.EncodeToString(signature)
	if _, jsonUnmarshalErr := signer.VerifyM2MToken(tamperedNonJSONToken); jsonUnmarshalErr == nil {
		t.Errorf("expected error on invalid claims JSON")
	}

	// 6. Expired token
	expiredM2MClaims := M2MClaims{
		Subject:   "sa-expired",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour).Unix(),
	}
	expiredClaimsJSON, _ := json.Marshal(expiredM2MClaims)
	expiredClaimsBase64 := base64.RawURLEncoding.EncodeToString(expiredClaimsJSON)
	expiredSigningInput := segments[0] + "." + expiredClaimsBase64
	expiredSignature := ed25519.Sign(signer.privateKey, []byte(expiredSigningInput))
	expiredToken := expiredSigningInput + "." + base64.RawURLEncoding.EncodeToString(expiredSignature)
	if _, expiredErr := signer.VerifyM2MToken(expiredToken); expiredErr == nil || !strings.Contains(expiredErr.Error(), "expired") {
		t.Errorf("expected token expired error, got %v", expiredErr)
	}
}
