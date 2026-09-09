package jwt

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"layr.sh/core"
)

func TestJWTJWKSHTTPIntegration(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	signer, err := NewSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/.well-known/jwks.json", signer.HandleJWKS)
	serveMux.HandleFunc("/.well-known/openid-configuration", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		oidcConfiguration := BuildOIDCDiscovery("http://" + request.Host)
		_ = json.NewEncoder(responseWriter).Encode(oidcConfiguration)
	})

	testServer := httptest.NewServer(serveMux)
	defer testServer.Close()

	// 1. Fetch JWKS from HTTP server
	jwksRequest, jwksRequestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, testServer.URL+"/.well-known/jwks.json", nil)
	if jwksRequestErr != nil {
		t.Fatalf("failed to create jwks request: %v", jwksRequestErr)
	}
	discoveryResponse, discoveryResponseErr := testServer.Client().Do(jwksRequest)
	if discoveryResponseErr != nil {
		t.Fatalf("failed to GET jwks: %v", discoveryResponseErr)
	}
	defer func() { _ = discoveryResponse.Body.Close() }()

	if discoveryResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from jwks endpoint, got: %d", discoveryResponse.StatusCode)
	}
	if contentType := discoveryResponse.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got: %s", contentType)
	}

	bodyBytes, jwksReadErr := io.ReadAll(discoveryResponse.Body)
	if jwksReadErr != nil {
		t.Fatalf("failed to read response body: %v", jwksReadErr)
	}

	var jwks JWKS
	if jwksUnmarshalErr := json.Unmarshal(bodyBytes, &jwks); jwksUnmarshalErr != nil {
		t.Fatalf("failed to unmarshal JWKS payload: %v", jwksUnmarshalErr)
	}

	if len(jwks.Keys) != 1 {
		t.Fatalf("expected exactly 1 JWK, got: %d", len(jwks.Keys))
	}

	jwk := jwks.Keys[0]
	if jwk.KeyID != signer.KeyID() || jwk.Algorithm != "EdDSA" || jwk.Curve != tls.Ed25519.String() || jwk.KeyType != "OKP" {
		t.Fatalf("unexpected JWK parameters: %+v", jwk)
	}

	// 2. Decode the public key from the JWKS response and verify token signature
	publicKeyBytes, publicKeyDecodeErr := base64.RawURLEncoding.DecodeString(jwk.X)
	if publicKeyDecodeErr != nil {
		t.Fatalf("failed to base64 decode jwk.X: %v", publicKeyDecodeErr)
	}
	discoveredPublicKey := ed25519.PublicKey(publicKeyBytes)

	userID := uuid.NewV7().String()
	token, tokenGenerateErr := signer.GenerateAccessToken(Claims{
		Subject: userID,
		Email:   "user@example.com",
		Claims:  map[string]any{"premium": true},
	}, 300)
	if tokenGenerateErr != nil {
		t.Fatalf("failed to generate access token: %v", tokenGenerateErr)
	}

	// Manually verify token using discovered public key
	tokenSegments := strings.Split(token, ".")
	if len(tokenSegments) != 3 {
		t.Fatalf("expected 3 segments in token, got: %d", len(tokenSegments))
	}
	signingInput := tokenSegments[0] + "." + tokenSegments[1]
	signatureBytes, sigDecodeErr := base64.RawURLEncoding.DecodeString(tokenSegments[2])
	if sigDecodeErr != nil {
		t.Fatalf("failed to decode signature bytes: %v", sigDecodeErr)
	}

	if !ed25519.Verify(discoveredPublicKey, []byte(signingInput), signatureBytes) {
		t.Fatal("token verification failed using discovered public key from JWKS HTTP endpoint")
	}

	// 3. Fetch OIDC Discovery from HTTP server
	oidcRequest, oidcRequestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, testServer.URL+"/.well-known/openid-configuration", nil)
	if oidcRequestErr != nil {
		t.Fatalf("failed to create oidc request: %v", oidcRequestErr)
	}
	oidcResponse, oidcFetchErr := testServer.Client().Do(oidcRequest)
	if oidcFetchErr != nil {
		t.Fatalf("failed to GET OIDC discovery: %v", oidcFetchErr)
	}
	defer func() { _ = oidcResponse.Body.Close() }()

	if oidcResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from OIDC discovery, got: %d", oidcResponse.StatusCode)
	}

	var oidcConfiguration OIDCConfiguration
	if oidcDecodeErr := json.NewDecoder(oidcResponse.Body).Decode(&oidcConfiguration); oidcDecodeErr != nil {
		t.Fatalf("failed to decode OIDC discovery JSON: %v", oidcDecodeErr)
	}

	if oidcConfiguration.Issuer != testServer.URL {
		t.Fatalf("expected issuer %s, got: %s", testServer.URL, oidcConfiguration.Issuer)
	}
	if oidcConfiguration.JwksURI != testServer.URL+"/.well-known/jwks.json" {
		t.Fatalf("expected jwks_uri %s, got: %s", testServer.URL+"/.well-known/jwks.json", oidcConfiguration.JwksURI)
	}
}
