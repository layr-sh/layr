package jwt

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestJWTJWKSAndOIDCDiscoveryUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	signer, err := NewSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	// 1. BuildJWKS validation
	jwks := signer.BuildJWKS()
	if len(jwks.Keys) != 1 {
		t.Fatalf("expected 1 key in JWKS, got: %d", len(jwks.Keys))
	}
	jwk := jwks.Keys[0]
	if jwk.KeyType != "OKP" || jwk.Curve != tls.Ed25519.String() || jwk.KeyID != signer.KeyID() || jwk.Use != "sig" || jwk.Algorithm != "EdDSA" || jwk.X == "" {
		t.Fatalf("unexpected JWK fields: %+v", jwk)
	}

	// 2. HandleJWKS HTTP handler validation
	discoveryRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/jwks.json", nil)
	discoveryResponseRecorder := httptest.NewRecorder()
	signer.HandleJWKS(discoveryResponseRecorder, discoveryRequest)
	if discoveryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on JWKS, got %d", discoveryResponseRecorder.Code)
	}
	if contentType := discoveryResponseRecorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json Content-Type, got: %s", contentType)
	}

	var decodedJWKS JWKS
	if err := json.NewDecoder(discoveryResponseRecorder.Body).Decode(&decodedJWKS); err != nil {
		t.Fatalf("failed to decode JWKS response body: %v", err)
	}
	if len(decodedJWKS.Keys) != 1 || decodedJWKS.Keys[0].KeyID != signer.KeyID() {
		t.Fatalf("mismatched decoded JWKS keys: %+v", decodedJWKS)
	}

	// 3. BuildOIDCDiscovery with custom base URL
	oidcConfiguration := BuildOIDCDiscovery("https://myauth.layr.sh")
	if oidcConfiguration.Issuer != "https://myauth.layr.sh" {
		t.Fatalf("expected custom issuer, got: %s", oidcConfiguration.Issuer)
	}
	if oidcConfiguration.AuthorizationEndpoint != "https://myauth.layr.sh/api/v1/auth/oauth/authorize" {
		t.Fatalf("unexpected authorization endpoint: %s", oidcConfiguration.AuthorizationEndpoint)
	}
	if oidcConfiguration.TokenEndpoint != "https://myauth.layr.sh/api/v1/auth/oauth/token" {
		t.Fatalf("unexpected token endpoint: %s", oidcConfiguration.TokenEndpoint)
	}
	if oidcConfiguration.UserinfoEndpoint != "https://myauth.layr.sh/api/v1/auth/oauth/userinfo" {
		t.Fatalf("unexpected userinfo endpoint: %s", oidcConfiguration.UserinfoEndpoint)
	}
	if oidcConfiguration.JwksURI != "https://myauth.layr.sh/.well-known/jwks.json" {
		t.Fatalf("unexpected jwks_uri: %s", oidcConfiguration.JwksURI)
	}
	if len(oidcConfiguration.ResponseTypesSupported) == 0 || len(oidcConfiguration.ScopesSupported) == 0 || len(oidcConfiguration.ClaimsSupported) == 0 {
		t.Fatalf("unexpected empty OIDC discovery arrays: %+v", oidcConfiguration)
	}

	// 4. BuildOIDCDiscovery with default base URL fallback
	defaultOIDCConfiguration := BuildOIDCDiscovery("")
	if defaultOIDCConfiguration.Issuer != "http://localhost:8080" {
		t.Fatalf("expected default issuer, got: %s", defaultOIDCConfiguration.Issuer)
	}
	if defaultOIDCConfiguration.JwksURI != "http://localhost:8080/.well-known/jwks.json" {
		t.Fatalf("unexpected default jwks_uri: %s", defaultOIDCConfiguration.JwksURI)
	}
}
