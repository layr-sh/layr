package jwt

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"uuid"

	"layr.sh/core"
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
	token, err := signer.GenerateAccessToken(userID, "user@example.com", "+123456789", "authenticated", false, map[string]any{"plan": "pro"}, 900)
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

	// Anonymous token verification
	anonymousToken, err := signer.GenerateAccessToken(userID, "", "", "authenticated", true, nil, 900)
	if err != nil {
		t.Fatalf("failed to generate anonymous token: %v", err)
	}
	anonymousClaims, err := signer.VerifyAccessToken(anonymousToken)
	if err != nil || !anonymousClaims.IsAnonymous {
		t.Fatalf("expected anonymous claims with IsAnonymous=true, got: %+v", anonymousClaims)
	}

	// Fallback role and expiry
	fallbackToken, err := signer.GenerateAccessToken(userID, "", "", "", false, nil, 0)
	if err != nil {
		t.Fatalf("failed to generate fallback token: %v", err)
	}
	fallbackClaims, err := signer.VerifyAccessToken(fallbackToken)
	if err != nil || fallbackClaims.Role != "authenticated" {
		t.Fatalf("expected authenticated role, got: %+v", fallbackClaims)
	}

	// Malformed tokens
	if _, err := signer.VerifyAccessToken("not.three.segments.extra"); err == nil {
		t.Fatalf("expected error on 4-part token")
	}
	if _, err := signer.VerifyAccessToken("eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiIxIn0.!!invalid-signature-base64!!"); err == nil {
		t.Fatalf("expected error on bad signature base64")
	}
	if _, err := signer.VerifyAccessToken("eyJhbGciOiJFZERTQSJ9.e30.c2ln"); err == nil {
		t.Fatalf("expected signature verification failure")
	}

	// Valid signature but invalid claims base64
	badClaimsPayload := "eyJhbGciOiJFZERTQSJ9.!bad-b64!"
	badSignature := ed25519.Sign(signer.privateKey, []byte(badClaimsPayload))
	badClaimsToken := badClaimsPayload + "." + base64.RawURLEncoding.EncodeToString(badSignature)
	if _, err := signer.VerifyAccessToken(badClaimsToken); err == nil {
		t.Fatalf("expected error on bad claims base64")
	}

	// Valid signature but non-JSON claims
	nonJSONClaimsPayload := "eyJhbGciOiJFZERTQSJ9.bm90LWpzb24"
	nonJSONSignature := ed25519.Sign(signer.privateKey, []byte(nonJSONClaimsPayload))
	nonJSONToken := nonJSONClaimsPayload + "." + base64.RawURLEncoding.EncodeToString(nonJSONSignature)
	if _, err := signer.VerifyAccessToken(nonJSONToken); err == nil {
		t.Fatalf("expected error on non-json claims")
	}

	// Expired token test
	expiredToken, _ := signer.GenerateAccessToken(userID, "exp@example.com", "", "authenticated", false, nil, -10)
	if _, err := signer.VerifyAccessToken(expiredToken); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error, got: %v", err)
	}

	// Future NotBefore token
	futureClaims := AppUserClaims{
		Subject:   userID,
		Issuer:    IssuerAppUser,
		Audience:  AudienceAppUser,
		IssuedAt:  time.Now().Unix() + 100,
		NotBefore: time.Now().Unix() + 100,
		ExpiresAt: time.Now().Unix() + 1000,
	}
	futureJSON, _ := json.Marshal(futureClaims)
	headerBase64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	futureClaimsBase64 := base64.RawURLEncoding.EncodeToString(futureJSON)
	futureSignature := ed25519.Sign(signer.privateKey, []byte(headerBase64+"."+futureClaimsBase64))
	futureToken := headerBase64 + "." + futureClaimsBase64 + "." + base64.RawURLEncoding.EncodeToString(futureSignature)
	if _, err := signer.VerifyAccessToken(futureToken); err == nil || !strings.Contains(err.Error(), "not valid yet") {
		t.Fatalf("expected not valid yet error, got: %v", err)
	}

	// Audience mismatch
	badAudienceClaims := futureClaims
	badAudienceClaims.NotBefore = time.Now().Unix() - 10
	badAudienceClaims.Audience = "wrong:audience"
	badAudienceJSON, _ := json.Marshal(badAudienceClaims)
	badAudienceClaimsBase64 := base64.RawURLEncoding.EncodeToString(badAudienceJSON)
	badAudienceSignature := ed25519.Sign(signer.privateKey, []byte(headerBase64+"."+badAudienceClaimsBase64))
	badAudienceToken := headerBase64 + "." + badAudienceClaimsBase64 + "." + base64.RawURLEncoding.EncodeToString(badAudienceSignature)
	if _, err := signer.VerifyAccessToken(badAudienceToken); err == nil || !strings.Contains(err.Error(), "invalid jwt audience") {
		t.Fatalf("expected invalid audience error, got: %v", err)
	}

	// Issuer mismatch
	badIssuerClaims := futureClaims
	badIssuerClaims.NotBefore = time.Now().Unix() - 10
	badIssuerClaims.Issuer = "wrong:issuer"
	badIssuerJSON, _ := json.Marshal(badIssuerClaims)
	badIssuerClaimsBase64 := base64.RawURLEncoding.EncodeToString(badIssuerJSON)
	badIssuerSignature := ed25519.Sign(signer.privateKey, []byte(headerBase64+"."+badIssuerClaimsBase64))
	badIssuerToken := headerBase64 + "." + badIssuerClaimsBase64 + "." + base64.RawURLEncoding.EncodeToString(badIssuerSignature)
	if _, err := signer.VerifyAccessToken(badIssuerToken); err == nil || !strings.Contains(err.Error(), "invalid jwt issuer") {
		t.Fatalf("expected invalid issuer error, got: %v", err)
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
	idToken, tokenGenerateErr := signer.GenerateIDToken("https://auth.example.com", "client-123", userID, "test@example.com", "+1234567890", "user", "nonce-xyz", true, false, false, 3600)
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

	var claims OIDCIDTokenClaims
	if unmarshalErr := json.Unmarshal(claimsBytes, &claims); unmarshalErr != nil {
		t.Fatalf("failed to parse claims JSON: %v", unmarshalErr)
	}

	if claims.Issuer != "https://auth.example.com" {
		t.Errorf("expected issuer https://auth.example.com, got %s", claims.Issuer)
	}
	if claims.Audience != "client-123" {
		t.Errorf("expected audience client-123, got %s", claims.Audience)
	}
	if claims.Subject != userID {
		t.Errorf("expected subject %s, got %s", userID, claims.Subject)
	}
	if claims.Email != "test@example.com" || !claims.EmailVerified {
		t.Errorf("unexpected email claims: %s, %v", claims.Email, claims.EmailVerified)
	}
	if claims.Nonce != "nonce-xyz" {
		t.Errorf("expected nonce nonce-xyz, got %s", claims.Nonce)
	}

	// Test GenerateIDToken with empty role and expiry 0
	defaultIDToken, defaultTokenGenerateErr := signer.GenerateIDToken("https://auth.example.com", "client-default", userID, "", "", "", "", false, false, false, 0)
	if defaultTokenGenerateErr != nil {
		t.Fatalf("failed to generate default id token: %v", defaultTokenGenerateErr)
	}
	if defaultIDToken == "" {
		t.Fatal("expected non-empty default id token")
	}

	// Verify discovery
	discovery := BuildOIDCDiscovery("https://auth.example.com")
	if discovery.Issuer != "https://auth.example.com" {
		t.Errorf("expected discovery issuer https://auth.example.com, got %s", discovery.Issuer)
	}
	if discovery.EndSessionEndpoint != "https://auth.example.com/api/v1/auth/oauth/sign-out" {
		t.Errorf("expected sign-out end session endpoint, got %s", discovery.EndSessionEndpoint)
	}
	if len(discovery.GrantTypesSupported) == 0 || discovery.GrantTypesSupported[0] != "authorization_code" {
		t.Errorf("unexpected grant types: %v", discovery.GrantTypesSupported)
	}
}
