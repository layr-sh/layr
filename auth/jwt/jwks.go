// Package jwt provides Ed25519 token signing, validation, JWKS, and OIDC discovery.
package jwt

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net/http"
)

// JWK represents a JSON Web Key for Ed25519 public key discovery.
type JWK struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	KeyID     string `json:"kid"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	X         string `json:"x"`
}

// JWKSResponse represents the RFC 7517 keys collection.
type JWKSResponse struct {
	Keys []JWK `json:"keys"`
}

// OIDCConfiguration represents the OpenID Connect discovery document (RFC 8414).
type OIDCConfiguration struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
	EndSessionEndpoint                string   `json:"end_session_endpoint"`
	JwksURI                           string   `json:"jwks_uri"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
}

// BuildJWKS constructs the JWKS payload containing the public key.
func (signer *Signer) BuildJWKS() JWKSResponse {
	publicKeyBase64 := base64.RawURLEncoding.EncodeToString(signer.publicKey)
	return JWKSResponse{
		Keys: []JWK{
			{
				KeyType:   "OKP",
				Curve:     tls.Ed25519.String(),
				KeyID:     KeyIDEd25519,
				Use:       "sig",
				Algorithm: "EdDSA",
				X:         publicKeyBase64,
			},
		},
	}
}

// BuildOIDCDiscovery generates the OpenID Connect discovery document for a given base URL.
func BuildOIDCDiscovery(baseURL string) OIDCConfiguration {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return OIDCConfiguration{
		Issuer:                            baseURL,
		AuthorizationEndpoint:             baseURL + "/api/v1/auth/oauth/authorize",
		TokenEndpoint:                     baseURL + "/api/v1/auth/oauth/token",
		UserinfoEndpoint:                  baseURL + "/api/v1/auth/oauth/userinfo",
		EndSessionEndpoint:                baseURL + "/api/v1/auth/oauth/sign-out",
		JwksURI:                           baseURL + "/.well-known/jwks.json",
		ResponseTypesSupported:            []string{"code", "token", "id_token"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgValuesSupported:  []string{"EdDSA"},
		ScopesSupported:                   []string{"openid", "profile", "email", "phone"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic", "none"},
		ClaimsSupported:                   []string{"sub", "iss", "aud", "exp", "iat", "email", "phone", "role"},
	}
}

// HandleJWKS serves the GET /.well-known/jwks.json endpoint.
func (signer *Signer) HandleJWKS(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_ = writeJSON(writer, signer.BuildJWKS())
}

func writeJSON(writer http.ResponseWriter, payload any) error {
	writer.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(writer).Encode(payload)
}
