package core

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCoreJWTJWKSUnit(t *testing.T) {
	UnloadConfig()
	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	jwtSigner := NewJWTSigner(cryptoKeyManager)

	// 1. BuildJWKS validation
	jwks := jwtSigner.BuildJWKS()
	if len(jwks.Keys) != 1 {
		t.Fatalf("expected 1 key in JWKS, got: %d", len(jwks.Keys))
	}
	jwk := jwks.Keys[0]
	if jwk.KeyType != "OKP" || jwk.Curve != tls.Ed25519.String() || jwk.KeyID != jwtSigner.KeyID() || jwk.Use != "sig" || jwk.Algorithm != "EdDSA" || jwk.X == "" {
		t.Fatalf("unexpected JWK fields: %+v", jwk)
	}

	// 2. handleGetJWKS HTTP handler validation
	discoveryRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/jwks.json", nil)
	discoveryResponseRecorder := httptest.NewRecorder()
	jwtSigner.handleGetJWKS(discoveryResponseRecorder, discoveryRequest)
	if discoveryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on JWKS, got %d", discoveryResponseRecorder.Code)
	}
	if contentType := discoveryResponseRecorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json Content-Type, got: %s", contentType)
	}

	var decodedJWKS JWKS
	if decodeErr := json.NewDecoder(discoveryResponseRecorder.Body).Decode(&decodedJWKS); decodeErr != nil {
		t.Fatalf("failed to decode JWKS response body: %v", decodeErr)
	}
	if len(decodedJWKS.Keys) != 1 || decodedJWKS.Keys[0].KeyID != jwtSigner.KeyID() {
		t.Fatalf("mismatched decoded JWKS keys: %+v", decodedJWKS)
	}

	// 3. VerifyJWT with valid JWKS
	token := jwtSigner.GenerateAccessToken(JWTClaims{
		Subject: "usr_test_jwks",
		Role:    "authenticated",
	}, 300)

	jwtClaims, err := jwks.VerifyJWT(token)
	if err != nil {
		t.Fatalf("failed to verify token with JWKS: %v", err)
	}
	if jwtClaims.Subject != "usr_test_jwks" {
		t.Fatalf("expected subject usr_test_jwks, got %s", jwtClaims.Subject)
	}

	// 4. VerifyJWT error conditions
	emptyJWKS := &JWKS{}
	if _, err := emptyJWKS.VerifyJWT(token); err == nil {
		t.Fatal("expected error on empty JWKS")
	}

	if _, err := jwks.VerifyJWT("invalid.token"); err == nil {
		t.Fatal("expected error on invalid token format")
	}

	if _, err := jwks.VerifyJWT("!bad*base64!.payload.sig"); err == nil {
		t.Fatal("expected error on invalid header base64")
	}

	if _, err := jwks.VerifyJWT("not-base64.not-base64.not-base64"); err == nil {
		t.Fatal("expected error on bad json header")
	}

	badHeaderToken := "e30.e30.c2ln"
	if _, err := jwks.VerifyJWT(badHeaderToken); err == nil {
		t.Fatal("expected error on missing kid with mismatched keys")
	}

	mismatchedJWKS := &JWKS{
		Keys: []JWK{
			{KeyID: "different-kid", X: jwk.X},
			{KeyID: "different-kid-2", X: jwk.X},
		},
	}
	if _, err := mismatchedJWKS.VerifyJWT(token); err == nil {
		t.Fatal("expected error on mismatched kid")
	}

	// 5. Test len(jwks.Keys) == 1 && kid == ""
	noKidHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	claimsBytes, _ := json.Marshal(JWTClaims{Subject: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	noKidClaims := base64.RawURLEncoding.EncodeToString(claimsBytes)
	noKidSigningInput := noKidHeader + "." + noKidClaims
	noKidSig := ed25519.Sign(jwtSigner.privateKey, []byte(noKidSigningInput))
	noKidToken := noKidSigningInput + "." + base64.RawURLEncoding.EncodeToString(noKidSig)
	if _, err := jwks.VerifyJWT(noKidToken); err != nil {
		t.Fatalf("expected single key JWKS to verify token without kid: %v", err)
	}

	// 6. Test invalid header JSON
	badHeaderJSONToken := base64.RawURLEncoding.EncodeToString([]byte("not-json")) + "." + noKidClaims + "." + base64.RawURLEncoding.EncodeToString(noKidSig)
	if _, err := jwks.VerifyJWT(badHeaderJSONToken); err == nil {
		t.Fatal("expected error on bad header json")
	}

	// 7. Test bad JWK.X base64
	badXJWKS := &JWKS{
		Keys: []JWK{
			{KeyID: jwtSigner.KeyID(), X: "!!!not-b64!!!"},
		},
	}
	if _, err := badXJWKS.VerifyJWT(token); err == nil {
		t.Fatal("expected error on bad jwk.X base64")
	}

	// 8. Test bad signature base64
	badSigBase64Token := noKidSigningInput + ".!!!not-b64!!!"
	if _, err := jwks.VerifyJWT(badSigBase64Token); err == nil {
		t.Fatal("expected error on bad signature base64")
	}

	// 9. Test signature verification failure
	tamperedInput := noKidHeader + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"tampered"}`))
	tamperedToken := tamperedInput + "." + base64.RawURLEncoding.EncodeToString(noKidSig)
	if _, err := jwks.VerifyJWT(tamperedToken); err == nil {
		t.Fatal("expected signature failure on tampered token")
	}

	// 10. Test bad claims base64
	badClaimsBase64Input := noKidHeader + ".!!!not-b64!!!"
	badClaimsBase64Sig := ed25519.Sign(jwtSigner.privateKey, []byte(badClaimsBase64Input))
	badClaimsBase64Token := badClaimsBase64Input + "." + base64.RawURLEncoding.EncodeToString(badClaimsBase64Sig)
	if _, err := jwks.VerifyJWT(badClaimsBase64Token); err == nil {
		t.Fatal("expected error on bad claims base64")
	}

	// 11. Test bad claims JSON
	badClaimsJSONInput := noKidHeader + "." + base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	badClaimsJSONSig := ed25519.Sign(jwtSigner.privateKey, []byte(badClaimsJSONInput))
	badClaimsJSONToken := badClaimsJSONInput + "." + base64.RawURLEncoding.EncodeToString(badClaimsJSONSig)
	if _, err := jwks.VerifyJWT(badClaimsJSONToken); err == nil {
		t.Fatal("expected error on bad claims JSON")
	}

	// 12. Test expired token
	expiredClaimsBytes, _ := json.Marshal(JWTClaims{Subject: "u1", ExpiresAt: time.Now().Add(-1 * time.Hour).Unix()})
	expiredClaimsBase64 := base64.RawURLEncoding.EncodeToString(expiredClaimsBytes)
	expiredInput := noKidHeader + "." + expiredClaimsBase64
	expiredSig := ed25519.Sign(jwtSigner.privateKey, []byte(expiredInput))
	expiredToken := expiredInput + "." + base64.RawURLEncoding.EncodeToString(expiredSig)
	if _, err := jwks.VerifyJWT(expiredToken); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error, got: %v", err)
	}

	// 13. Test NotBefore in future
	futureClaimsBytes, _ := json.Marshal(JWTClaims{Subject: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix(), NotBefore: time.Now().Add(time.Hour).Unix()})
	futureClaimsBase64 := base64.RawURLEncoding.EncodeToString(futureClaimsBytes)
	futureInput := noKidHeader + "." + futureClaimsBase64
	futureSig := ed25519.Sign(jwtSigner.privateKey, []byte(futureInput))
	futureToken := futureInput + "." + base64.RawURLEncoding.EncodeToString(futureSig)
	if _, err := jwks.VerifyJWT(futureToken); err == nil || !strings.Contains(err.Error(), "not valid yet") {
		t.Fatalf("expected not valid yet error, got: %v", err)
	}
}
