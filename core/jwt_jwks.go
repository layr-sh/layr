package core

import (
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// JWK represents a JSON Web Key for Ed25519 public key discovery (RFC 7517).
type JWK struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	KeyID     string `json:"kid"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	X         string `json:"x"`
}

// JWKS represents the RFC 7517 keys collection.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// BuildJWKS constructs the JWKS payload containing the public key.
func (signer *JWTSigner) BuildJWKS() JWKS {
	log.Debugf("building JWKS payload with keyID %s", signer.keyID)
	publicKeyBase64 := base64.RawURLEncoding.EncodeToString(signer.publicKey)
	log.Tracef("encoded public key for JWKS keyID %s", signer.keyID)
	return JWKS{
		Keys: []JWK{
			{
				KeyType:   "OKP",
				Curve:     tls.Ed25519.String(),
				KeyID:     signer.keyID,
				Use:       "sig",
				Algorithm: "EdDSA",
				X:         publicKeyBase64,
			},
		},
	}
}

// HandleJWKS serves the GET /.well-known/jwks.json HTTP endpoint.
func (signer *JWTSigner) HandleJWKS(responseWriter http.ResponseWriter, request *http.Request) {
	log.Tracef("handling JWKS HTTP request from %s", request.RemoteAddr)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(signer.BuildJWKS())
}

// VerifyJWT validates an Ed25519 token against the keys in JWKS.
func (jwks *JWKS) VerifyJWT(token string) (*JWTClaims, error) {
	if jwks == nil || len(jwks.Keys) == 0 {
		return nil, errors.New("jwks contains no keys")
	}

	tokenSegments := strings.Split(token, ".")
	if len(tokenSegments) != expectedTokenSegmentCount {
		return nil, errors.New("invalid token format: must contain 3 segments")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(tokenSegments[0])
	if err != nil {
		return nil, fmt.Errorf("invalid header base64: %w", err)
	}

	var header map[string]string
	if unmarshalErr := json.Unmarshal(headerJSON, &header); unmarshalErr != nil {
		return nil, fmt.Errorf("invalid header json: %w", unmarshalErr)
	}

	kid := header["kid"]
	var matchedJWK *JWK
	for i := range jwks.Keys {
		if jwks.Keys[i].KeyID == kid {
			matchedJWK = &jwks.Keys[i]
			break
		}
	}
	if matchedJWK == nil {
		if len(jwks.Keys) == 1 && kid == "" {
			matchedJWK = &jwks.Keys[0]
		} else {
			return nil, fmt.Errorf("no matching key found in jwks for keyID %q", kid)
		}
	}

	publicKeyBytes, err := base64.RawURLEncoding.DecodeString(matchedJWK.X)
	if err != nil {
		return nil, fmt.Errorf("failed to decode jwk public key: %w", err)
	}

	signingInput := tokenSegments[0] + "." + tokenSegments[1]
	signature, err := base64.RawURLEncoding.DecodeString(tokenSegments[2])
	if err != nil {
		return nil, errors.New("invalid signature encoding")
	}

	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), []byte(signingInput), signature) {
		return nil, errors.New("jwt signature verification failed")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(tokenSegments[1])
	if err != nil {
		return nil, fmt.Errorf("invalid claims base64: %w", err)
	}

	var jwtClaims JWTClaims
	if unmarshalClaimsErr := json.Unmarshal(claimsJSON, &jwtClaims); unmarshalClaimsErr != nil {
		return nil, fmt.Errorf("invalid claims json: %w", unmarshalClaimsErr)
	}

	now := time.Now().UTC().Unix()
	if jwtClaims.ExpiresAt < now {
		return nil, errors.New("jwt token expired")
	}
	if jwtClaims.NotBefore > now {
		return nil, errors.New("jwt token not valid yet")
	}

	return &jwtClaims, nil
}
