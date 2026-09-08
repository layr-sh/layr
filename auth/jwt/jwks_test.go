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
	key := jwks.Keys[0]
	if key.KeyType != "OKP" || key.Curve != tls.Ed25519.String() || key.KeyID != signer.KeyID() || key.Use != "sig" || key.Algorithm != "EdDSA" || key.X == "" {
		t.Fatalf("unexpected JWK fields: %+v", key)
	}

	// 2. HandleJWKS HTTP handler validation
	discoveryRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/jwks.json", nil)
	discoveryRecorder := httptest.NewRecorder()
	signer.HandleJWKS(discoveryRecorder, discoveryRequest)
	if discoveryRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on JWKS, got %d", discoveryRecorder.Code)
	}
	if contentType := discoveryRecorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json Content-Type, got: %s", contentType)
	}

	var decodedJWKS JWKSResponse
	if err := json.NewDecoder(discoveryRecorder.Body).Decode(&decodedJWKS); err != nil {
		t.Fatalf("failed to decode JWKS response body: %v", err)
	}
	if len(decodedJWKS.Keys) != 1 || decodedJWKS.Keys[0].KeyID != signer.KeyID() {
		t.Fatalf("mismatched decoded JWKS keys: %+v", decodedJWKS)
	}

	// 3. BuildOIDCDiscovery with custom base URL
	oidc := BuildOIDCDiscovery("https://myauth.layr.sh")
	if oidc.Issuer != "https://myauth.layr.sh" {
		t.Fatalf("expected custom issuer, got: %s", oidc.Issuer)
	}
	if oidc.AuthorizationEndpoint != "https://myauth.layr.sh/api/v1/auth/oauth/authorize" {
		t.Fatalf("unexpected authorization endpoint: %s", oidc.AuthorizationEndpoint)
	}
	if oidc.TokenEndpoint != "https://myauth.layr.sh/api/v1/auth/oauth/token" {
		t.Fatalf("unexpected token endpoint: %s", oidc.TokenEndpoint)
	}
	if oidc.UserinfoEndpoint != "https://myauth.layr.sh/api/v1/auth/oauth/userinfo" {
		t.Fatalf("unexpected userinfo endpoint: %s", oidc.UserinfoEndpoint)
	}
	if oidc.JwksURI != "https://myauth.layr.sh/.well-known/jwks.json" {
		t.Fatalf("unexpected jwks_uri: %s", oidc.JwksURI)
	}
	if len(oidc.ResponseTypesSupported) == 0 || len(oidc.ScopesSupported) == 0 || len(oidc.ClaimsSupported) == 0 {
		t.Fatalf("unexpected empty OIDC discovery arrays: %+v", oidc)
	}

	// 4. BuildOIDCDiscovery with default base URL fallback
	defaultOIDC := BuildOIDCDiscovery("")
	if defaultOIDC.Issuer != "http://localhost:8080" {
		t.Fatalf("expected default issuer, got: %s", defaultOIDC.Issuer)
	}
	if defaultOIDC.JwksURI != "http://localhost:8080/.well-known/jwks.json" {
		t.Fatalf("unexpected default jwks_uri: %s", defaultOIDC.JwksURI)
	}
}
