package core

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"
)

func TestCoreJWTEndToEndTokenIssuanceAndDiscoveryE2E(t *testing.T) {
	UnloadConfig()
	masterEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cryptoKeyManager, err := NewCryptoKeyManager(masterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	authJWTSigner, err := NewJWTSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	// 1. Setup Authority HTTP server serving JWKS
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/.well-known/jwks.json", authJWTSigner.HandleJWKS)

	authServer := httptest.NewServer(serveMux)
	defer authServer.Close()

	// 2. Simulate User Login: Authority generates access token and refresh token
	applicationUserID := uuid.NewV7().String()
	sessionID := uuid.NewV7().String()
	userEmail := "user@example.com"
	userRole := "member"
	userCustomClaims := map[string]any{
		"subscription_tier": "pro",
		"features":          []any{"analytics", "storage"},
	}

	accessToken, tokenGenerateErr := authJWTSigner.GenerateAccessToken(JWTClaims{
		Subject:   applicationUserID,
		SessionID: sessionID,
		Email:     userEmail,
		Phone:     "+15550001111",
		Role:      userRole,
		Claims:    userCustomClaims,
	}, 3600)
	if tokenGenerateErr != nil {
		t.Fatalf("failed to generate access token: %v", tokenGenerateErr)
	}

	refreshToken := authJWTSigner.GenerateRefreshToken()
	storedRefreshTokenHash := authJWTSigner.HashRefreshToken(refreshToken)

	// 3. Resource Server Dynamic Discovery Flow: Query jwks.json to download public key
	jwksRequest, jwksRequestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, authServer.URL+"/.well-known/jwks.json", nil)
	if jwksRequestErr != nil {
		t.Fatalf("failed to create jwks request: %v", jwksRequestErr)
	}
	jwksResponse, jwksFetchErr := authServer.Client().Do(jwksRequest)
	if jwksFetchErr != nil {
		t.Fatalf("failed to fetch JWKS: %v", jwksFetchErr)
	}
	defer func() { _ = jwksResponse.Body.Close() }()

	var jwks JWKS
	if jwksDecodeErr := json.NewDecoder(jwksResponse.Body).Decode(&jwks); jwksDecodeErr != nil {
		t.Fatalf("failed to decode JWKS payload: %v", jwksDecodeErr)
	}

	if len(jwks.Keys) == 0 {
		t.Fatal("expected at least one JWK in discovered keys")
	}

	// Extract and construct public key from discovered JWK
	var activePublicKey ed25519.PublicKey
	for _, key := range jwks.Keys {
		if key.KeyID == authJWTSigner.KeyID() && key.KeyType == "OKP" && key.Curve == tls.Ed25519.String() {
			rawPublicKeyBytes, publicKeyDecodeErr := base64.RawURLEncoding.DecodeString(key.X)
			if publicKeyDecodeErr != nil {
				t.Fatalf("failed to base64 decode key.X: %v", publicKeyDecodeErr)
			}
			activePublicKey = ed25519.PublicKey(rawPublicKeyBytes)
			break
		}
	}

	if len(activePublicKey) == 0 {
		t.Fatal("failed to extract valid Ed25519 public key from JWKS")
	}

	// 4. Resource Server authenticates user request using discovered public key
	tokenSegments := strings.Split(accessToken, ".")
	if len(tokenSegments) != 3 {
		t.Fatalf("expected 3 token segments, got: %d", len(tokenSegments))
	}

	signedInput := tokenSegments[0] + "." + tokenSegments[1]
	signatureBytes, sigDecodeErr := base64.RawURLEncoding.DecodeString(tokenSegments[2])
	if sigDecodeErr != nil {
		t.Fatalf("failed to decode token signature: %v", sigDecodeErr)
	}

	// Verify cryptographic signature with discovered key
	if !ed25519.Verify(activePublicKey, []byte(signedInput), signatureBytes) {
		t.Fatal("cryptographic signature verification failed on resource server")
	}

	// Decode claims payload
	claimsBytes, claimsDecodeErr := base64.RawURLEncoding.DecodeString(tokenSegments[1])
	if claimsDecodeErr != nil {
		t.Fatalf("failed to decode claims payload: %v", claimsDecodeErr)
	}

	var verifiedJWTClaims JWTClaims
	if claimsUnmarshalErr := json.Unmarshal(claimsBytes, &verifiedJWTClaims); claimsUnmarshalErr != nil {
		t.Fatalf("failed to unmarshal verified claims: %v", claimsUnmarshalErr)
	}

	// Verify claims standard assertions
	nowTimestamp := time.Now().UTC().Unix()
	if verifiedJWTClaims.ExpiresAt < nowTimestamp {
		t.Fatal("token is unexpectedly expired")
	}
	if verifiedJWTClaims.NotBefore > nowTimestamp {
		t.Fatal("token is not yet valid")
	}
	expectedAudience := "layr-app:user"
	if verifiedJWTClaims.Audience != expectedAudience {
		t.Fatalf("unexpected audience: %s", verifiedJWTClaims.Audience)
	}
	expectedIssuer := "layr-app"
	if verifiedJWTClaims.Issuer != expectedIssuer {
		t.Fatalf("unexpected issuer: %s", verifiedJWTClaims.Issuer)
	}
	if verifiedJWTClaims.Subject != applicationUserID {
		t.Fatalf("unexpected subject: %s", verifiedJWTClaims.Subject)
	}
	if verifiedJWTClaims.SessionID != sessionID {
		t.Fatalf("unexpected session ID: %s", verifiedJWTClaims.SessionID)
	}
	if verifiedJWTClaims.Email != userEmail || verifiedJWTClaims.Role != userRole {
		t.Fatalf("unexpected email or role: %s, %s", verifiedJWTClaims.Email, verifiedJWTClaims.Role)
	}
	if verifiedJWTClaims.Claims["subscription_tier"] != "pro" {
		t.Fatalf("unexpected claim subscription_tier: %v", verifiedJWTClaims.Claims["subscription_tier"])
	}

	// 5. Verification against Authority Internal Signer
	authorityVerifiedJWTClaims, authorityVerifyErr := authJWTSigner.VerifyAccessToken(accessToken)
	if authorityVerifyErr != nil {
		t.Fatalf("auth authority internal verification failed: %v", authorityVerifyErr)
	}
	if assertErr := authorityVerifiedJWTClaims.Assert("sub", applicationUserID); assertErr != nil {
		t.Fatalf("assert sub failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedJWTClaims.Assert("sid", sessionID); assertErr != nil {
		t.Fatalf("assert sid failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedJWTClaims.Assert("email", userEmail); assertErr != nil {
		t.Fatalf("assert email failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedJWTClaims.Assert("role", userRole); assertErr != nil {
		t.Fatalf("assert role failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedJWTClaims.Assert("aud", expectedAudience); assertErr != nil {
		t.Fatalf("assert aud failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedJWTClaims.Assert("iss", expectedIssuer); assertErr != nil {
		t.Fatalf("assert iss failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedJWTClaims.Assert("subscription_tier", "pro"); assertErr != nil {
		t.Fatalf("assert subscription_tier failed: %v", assertErr)
	}

	// 6. Security Invariants: Tampered and Forged Token Rejection
	// A. Modified payload
	tamperedPayload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"forged-user-id"}`))
	tamperedToken := tokenSegments[0] + "." + tamperedPayload + "." + tokenSegments[2]
	if _, tamperedVerifyErr := authJWTSigner.VerifyAccessToken(tamperedToken); tamperedVerifyErr == nil {
		t.Fatal("expected failure on tampered payload token")
	}

	// B. Invalid signature
	corruptedSignature := base64.RawURLEncoding.EncodeToString([]byte("totally-corrupted-signature-bytes"))
	invalidSignatureToken := tokenSegments[0] + "." + tokenSegments[1] + "." + corruptedSignature
	if _, invalidSigVerifyErr := authJWTSigner.VerifyAccessToken(invalidSignatureToken); invalidSigVerifyErr == nil {
		t.Fatal("expected failure on invalid signature token")
	}

	// C. Forged token signed with different key
	adversaryCryptoKeyManager, _ := NewCryptoKeyManager("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	adversaryJWTSigner, _ := NewJWTSigner(adversaryCryptoKeyManager)
	adversaryToken, _ := adversaryJWTSigner.GenerateAccessToken(JWTClaims{
		Subject: applicationUserID,
		Email:   "forged@example.com",
	}, 600)
	if _, adversaryVerifyErr := authJWTSigner.VerifyAccessToken(adversaryToken); adversaryVerifyErr == nil {
		t.Fatal("expected failure on token signed with adversary key")
	}

	// 7. Refresh token validation and rotation
	clientProvidedRefreshToken := refreshToken
	if authJWTSigner.HashRefreshToken(clientProvidedRefreshToken) != storedRefreshTokenHash {
		t.Fatal("expected refresh token hash to match stored hash")
	}

	// Rotate refresh token
	newRefreshToken := authJWTSigner.GenerateRefreshToken()
	newRefreshTokenHash := authJWTSigner.HashRefreshToken(newRefreshToken)
	if newRefreshToken == refreshToken || newRefreshTokenHash == storedRefreshTokenHash {
		t.Fatal("expected new distinct refresh token upon rotation")
	}

	// 8. One-time verification HMAC token flow
	verificationNonce := fmt.Sprintf("nonce-%s-%d", applicationUserID, nowTimestamp)
	verificationToken := authJWTSigner.SignHMAC(verificationNonce)
	if !authJWTSigner.VerifyHMAC(verificationNonce, verificationToken) {
		t.Fatal("HMAC verification failed for one-time token")
	}
	if authJWTSigner.VerifyHMAC(verificationNonce, "invalid-tampered-token") {
		t.Fatal("HMAC verification should have failed for tampered token")
	}
}
