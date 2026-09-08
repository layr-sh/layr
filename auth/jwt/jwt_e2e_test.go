package jwt

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

	"layr.sh/core"
)

func TestJWTEndToEndTokenIssuanceAndDiscoveryE2E(t *testing.T) {
	masterEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cryptoKeyManager, err := core.NewCryptoKeyManager(masterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	authSigner, err := NewSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	// 1. Setup Auth Authority HTTP server serving OIDC discovery and JWKS
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/.well-known/jwks.json", authSigner.HandleJWKS)
	serveMux.HandleFunc("/.well-known/openid-configuration", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		discoveryDocument := BuildOIDCDiscovery("http://" + request.Host)
		_ = json.NewEncoder(writer).Encode(discoveryDocument)
	})

	authServer := httptest.NewServer(serveMux)
	defer authServer.Close()

	// 2. Simulate User Login: Authority generates access token and refresh token
	applicationUserID := uuid.NewV7().String()
	userEmail := "user@example.com"
	userRole := "member"
	userCustomClaims := map[string]any{
		"subscription_tier": "pro",
		"features":          []any{"analytics", "storage"},
	}

	accessToken, tokenGenerateErr := authSigner.GenerateAccessToken(Claims{
		Subject: applicationUserID,
		Email:   userEmail,
		Phone:   "+15550001111",
		Role:    userRole,
		Claims:  userCustomClaims,
	}, 3600)
	if tokenGenerateErr != nil {
		t.Fatalf("failed to generate access token: %v", tokenGenerateErr)
	}

	refreshToken := GenerateRefreshToken()
	storedRefreshTokenHash := HashRefreshToken(refreshToken)

	// 3. Resource Server Dynamic Discovery Flow:
	// A. Query OIDC discovery to discover jwks_uri
	oidcRequest, oidcRequestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, authServer.URL+"/.well-known/openid-configuration", nil)
	if oidcRequestErr != nil {
		t.Fatalf("failed to create oidc request: %v", oidcRequestErr)
	}
	oidcDiscoveryResponse, oidcFetchErr := authServer.Client().Do(oidcRequest)
	if oidcFetchErr != nil {
		t.Fatalf("failed to fetch OIDC configuration: %v", oidcFetchErr)
	}
	defer func() { _ = oidcDiscoveryResponse.Body.Close() }()

	var oidcConfiguration OIDCConfiguration
	if oidcDecodeErr := json.NewDecoder(oidcDiscoveryResponse.Body).Decode(&oidcConfiguration); oidcDecodeErr != nil {
		t.Fatalf("failed to decode OIDC configuration: %v", oidcDecodeErr)
	}

	// B. Query discovered jwks_uri to download public key
	jwksRequest, jwksRequestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, oidcConfiguration.JwksURI, nil)
	if jwksRequestErr != nil {
		t.Fatalf("failed to create jwks request: %v", jwksRequestErr)
	}
	jwksResponse, jwksFetchErr := authServer.Client().Do(jwksRequest)
	if jwksFetchErr != nil {
		t.Fatalf("failed to fetch JWKS from %s: %v", oidcConfiguration.JwksURI, jwksFetchErr)
	}
	defer func() { _ = jwksResponse.Body.Close() }()

	var jwksData JWKSResponse
	if jwksDecodeErr := json.NewDecoder(jwksResponse.Body).Decode(&jwksData); jwksDecodeErr != nil {
		t.Fatalf("failed to decode JWKS payload: %v", jwksDecodeErr)
	}

	if len(jwksData.Keys) == 0 {
		t.Fatal("expected at least one JWK in discovered keys")
	}

	// C. Extract and construct public key from discovered JWK
	var activePublicKey ed25519.PublicKey
	for _, key := range jwksData.Keys {
		if key.KeyID == authSigner.KeyID() && key.KeyType == "OKP" && key.Curve == tls.Ed25519.String() {
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

	var verifiedClaims Claims
	if claimsUnmarshalErr := json.Unmarshal(claimsBytes, &verifiedClaims); claimsUnmarshalErr != nil {
		t.Fatalf("failed to unmarshal verified claims: %v", claimsUnmarshalErr)
	}

	// Verify claims standard assertions
	nowTimestamp := time.Now().UTC().Unix()
	if verifiedClaims.ExpiresAt < nowTimestamp {
		t.Fatal("token is unexpectedly expired")
	}
	if verifiedClaims.NotBefore > nowTimestamp {
		t.Fatal("token is not yet valid")
	}
	expectedAudience := core.GetConfig().Project.Slug() + ":user"
	if verifiedClaims.Audience != expectedAudience {
		t.Fatalf("unexpected audience: %s", verifiedClaims.Audience)
	}
	expectedIssuer := core.GetConfig().Project.Slug()
	if verifiedClaims.Issuer != expectedIssuer {
		t.Fatalf("unexpected issuer: %s", verifiedClaims.Issuer)
	}
	if verifiedClaims.Subject != applicationUserID {
		t.Fatalf("unexpected subject: %s", verifiedClaims.Subject)
	}
	if verifiedClaims.Email != userEmail || verifiedClaims.Role != userRole {
		t.Fatalf("unexpected email or role: %s, %s", verifiedClaims.Email, verifiedClaims.Role)
	}
	if verifiedClaims.Claims["subscription_tier"] != "pro" {
		t.Fatalf("unexpected claim subscription_tier: %v", verifiedClaims.Claims["subscription_tier"])
	}

	// 5. Verification against Authority Internal Signer
	authorityVerifiedClaims, authorityVerifyErr := authSigner.VerifyAccessToken(accessToken)
	if authorityVerifyErr != nil {
		t.Fatalf("auth authority internal verification failed: %v", authorityVerifyErr)
	}
	if assertErr := authorityVerifiedClaims.Assert("sub", applicationUserID); assertErr != nil {
		t.Fatalf("assert sub failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedClaims.Assert("email", userEmail); assertErr != nil {
		t.Fatalf("assert email failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedClaims.Assert("role", userRole); assertErr != nil {
		t.Fatalf("assert role failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedClaims.Assert("aud", expectedAudience); assertErr != nil {
		t.Fatalf("assert aud failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedClaims.Assert("iss", expectedIssuer); assertErr != nil {
		t.Fatalf("assert iss failed: %v", assertErr)
	}
	if assertErr := authorityVerifiedClaims.Assert("subscription_tier", "pro"); assertErr != nil {
		t.Fatalf("assert subscription_tier failed: %v", assertErr)
	}

	// 6. Security Invariants: Tampered and Forged Token Rejection
	// A. Modified payload
	tamperedPayload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"forged-user-id"}`))
	tamperedToken := tokenSegments[0] + "." + tamperedPayload + "." + tokenSegments[2]
	if _, tamperedVerifyErr := authSigner.VerifyAccessToken(tamperedToken); tamperedVerifyErr == nil {
		t.Fatal("expected failure on tampered payload token")
	}

	// B. Invalid signature
	corruptedSignature := base64.RawURLEncoding.EncodeToString([]byte("totally-corrupted-signature-bytes"))
	invalidSignatureToken := tokenSegments[0] + "." + tokenSegments[1] + "." + corruptedSignature
	if _, invalidSigVerifyErr := authSigner.VerifyAccessToken(invalidSignatureToken); invalidSigVerifyErr == nil {
		t.Fatal("expected failure on invalid signature token")
	}

	// C. Forged token signed with different key
	adversaryCryptoKeyManager, _ := core.NewCryptoKeyManager("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	adversarySigner, _ := NewSigner(adversaryCryptoKeyManager)
	adversaryToken, _ := adversarySigner.GenerateAccessToken(Claims{
		Subject: applicationUserID,
		Email:   "forged@example.com",
	}, 600)
	if _, adversaryVerifyErr := authSigner.VerifyAccessToken(adversaryToken); adversaryVerifyErr == nil {
		t.Fatal("expected failure on token signed with adversary key")
	}

	// 7. Refresh token validation and rotation
	clientProvidedRefreshToken := refreshToken
	if HashRefreshToken(clientProvidedRefreshToken) != storedRefreshTokenHash {
		t.Fatal("expected refresh token hash to match stored hash")
	}

	// Rotate refresh token
	newRefreshToken := GenerateRefreshToken()
	newRefreshTokenHash := HashRefreshToken(newRefreshToken)
	if newRefreshToken == refreshToken || newRefreshTokenHash == storedRefreshTokenHash {
		t.Fatal("expected new distinct refresh token upon rotation")
	}

	// 8. One-time verification HMAC token flow
	verificationNonce := fmt.Sprintf("nonce-%s-%d", applicationUserID, nowTimestamp)
	verificationToken := authSigner.SignHMAC(verificationNonce)
	if !authSigner.VerifyHMAC(verificationNonce, verificationToken) {
		t.Fatal("HMAC verification failed for one-time token")
	}
	if authSigner.VerifyHMAC(verificationNonce, "invalid-tampered-token") {
		t.Fatal("HMAC verification should have failed for tampered token")
	}
}
