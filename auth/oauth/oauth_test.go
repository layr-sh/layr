package oauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestOauthBuildAuthorizeURLUnit(t *testing.T) {
	// 1. Google Preset URL
	googleAuthorizeURL, err := BuildAuthorizeURL(PresetGoogle, "client-id-123", "http://localhost/callback", "state-google", "")
	if err != nil || !strings.Contains(googleAuthorizeURL, "accounts.google.com") || !strings.Contains(googleAuthorizeURL, "openid") {
		t.Fatalf("unexpected Google auth URL: %s (err: %v)", googleAuthorizeURL, err)
	}

	// 2. GitHub Preset URL
	githubAuthorizeURL, err := BuildAuthorizeURL(PresetGitHub, "github-client-id", "http://localhost/callback", "state-github", "")
	if err != nil || !strings.Contains(githubAuthorizeURL, "github.com") {
		t.Fatalf("unexpected GitHub auth URL: %s", githubAuthorizeURL)
	}

	// 3. Discord Preset URL
	discordAuthorizeURL, err := BuildAuthorizeURL(PresetDiscord, "discord-client-id", "http://localhost/callback", "state-discord", "")
	if err != nil || !strings.Contains(discordAuthorizeURL, "discord.com") {
		t.Fatalf("unexpected Discord auth URL: %s", discordAuthorizeURL)
	}

	// 4. Apple Preset URL with form_post and code+id_token
	appleAuthorizeURL, err := BuildAuthorizeURL(PresetApple, "apple-client-id", "http://localhost/callback", "state-apple", "")
	if err != nil || !strings.Contains(appleAuthorizeURL, "form_post") || !strings.Contains(appleAuthorizeURL, "code+id_token") {
		t.Fatalf("unexpected Apple auth URL: %s", appleAuthorizeURL)
	}

	// 5. Custom Provider build URL with query parameters
	customProviderConfig := ProviderConfig{
		Name:         "custom-identity-provider",
		AuthURL:      "https://idp.example.com/oauth/auth?tenant=corp",
		TokenURL:     "https://idp.example.com/oauth/token",
		ClientID:     "custom-client-id",
		Scope:        "openid profile",
		ResponseType: "code",
	}
	customURL, err := BuildAuthorizeURLWithConfig(customProviderConfig, "http://localhost/callback", "state-custom")
	if err != nil || !strings.Contains(customURL, "tenant=corp&client_id=custom-client-id") {
		t.Fatalf("unexpected custom auth URL: %s (err: %v)", customURL, err)
	}

	// 6. Build with empty ResponseType defaults to "code"
	customDefaultTypeProviderConfig := ProviderConfig{
		AuthURL:  "https://idp.example.com/oauth/auth",
		TokenURL: "https://idp.example.com/oauth/token",
		ClientID: "custom-id",
	}
	customDefaultTypeURL, err := BuildAuthorizeURLWithConfig(customDefaultTypeProviderConfig, "http://localhost/callback", "state-default")
	if err != nil || !strings.Contains(customDefaultTypeURL, "response_type=code") {
		t.Fatalf("expected response_type=code, got: %s", customDefaultTypeURL)
	}

	// 7. Validation errors
	if _, err := BuildAuthorizeURL("unsupported-preset", "id", "http://localhost/callback", "state", ""); err == nil {
		t.Fatal("expected error on unsupported provider without URLs")
	}
	if _, err := BuildAuthorizeURLWithConfig(ProviderConfig{AuthURL: "https://auth.com"}, "http://localhost/callback", "state"); err == nil {
		t.Fatal("expected error on empty client_id")
	}
	if _, err := BuildAuthorizeURLWithConfig(ProviderConfig{ClientID: "id"}, "http://localhost/callback", "state"); err == nil {
		t.Fatal("expected error on empty auth_url")
	}
}

func TestOauthResolveProviderConfigUnit(t *testing.T) {
	// 1. Explicit Preset in config
	resolvedProviderConfig, err := ResolveProviderConfig("", ProviderConfig{
		Preset:   PresetGoogle,
		ClientID: "client-preset",
	})
	if err != nil || resolvedProviderConfig.AuthURL == "" || resolvedProviderConfig.Scope != "openid email profile" {
		t.Fatalf("expected resolved preset, got: %+v (err: %v)", resolvedProviderConfig, err)
	}

	// 2. Custom provider missing token_url
	_, err = ResolveProviderConfig("custom", ProviderConfig{
		AuthURL: "https://auth.com",
	})
	if err == nil || !strings.Contains(err.Error(), "requires auth_url and token_url") {
		t.Fatalf("expected error on missing token_url, got: %v", err)
	}

	// 3. Custom provider missing auth_url
	_, err = ResolveProviderConfig("custom", ProviderConfig{
		TokenURL: "https://token.com",
	})
	if err == nil || !strings.Contains(err.Error(), "requires auth_url and token_url") {
		t.Fatalf("expected error on missing auth_url, got: %v", err)
	}
}

func TestOauthParseIDTokenUnit(t *testing.T) {
	if claims := parseIDToken("invalid"); claims != nil {
		t.Fatal("expected nil on invalid token format")
	}
	if claims := parseIDToken("a.!invalid-base64!.c"); claims != nil {
		t.Fatal("expected nil on invalid base64 payload")
	}
	if claims := parseIDToken("a." + base64.RawURLEncoding.EncodeToString([]byte(`not-json`)) + ".c"); claims != nil {
		t.Fatal("expected nil on non-json payload")
	}

	standardBase64Token := "a." + base64.StdEncoding.EncodeToString([]byte(`{"sub":"std-sub"}`)) + ".c"
	claims := parseIDToken(standardBase64Token)
	if claims == nil || claims["sub"] != "std-sub" {
		t.Fatalf("expected std base64 claims, got: %+v", claims)
	}
}

func TestOauthExtractAttributesUnit(t *testing.T) {
	// 1. Custom attributes
	customProviderConfig := ProviderConfig{
		IDAttribute:     "uid",
		EmailAttribute:  "mail",
		NameAttribute:   "display_name",
		AvatarAttribute: "photo_url",
	}
	userInfo := &UserInfo{}
	extractAttributes(userInfo, map[string]any{
		"uid":          "custom-uid-123",
		"mail":         "user@corp.com",
		"display_name": "Corp User",
		"photo_url":    "https://corp.com/photo.png",
	}, customProviderConfig)

	if userInfo.ProviderUserID != "custom-uid-123" || userInfo.Email != "user@corp.com" || userInfo.Name != "Corp User" || userInfo.AvatarURL != "https://corp.com/photo.png" {
		t.Fatalf("mismatched extracted custom attributes: %+v", userInfo)
	}

	// 2. Fallbacks to standard fields (id, username, picture)
	fallbackUserInfo := &UserInfo{}
	extractAttributes(fallbackUserInfo, map[string]any{
		"id":       float64(456789),
		"email":    "fallback@example.com",
		"username": "fallback_user",
		"picture":  "https://fallback.com/pic.png",
	}, ProviderConfig{})

	if fallbackUserInfo.ProviderUserID != "456789" || fallbackUserInfo.Email != "fallback@example.com" || fallbackUserInfo.Name != "fallback_user" || fallbackUserInfo.AvatarURL != "https://fallback.com/pic.png" {
		t.Fatalf("mismatched fallback attributes: %+v", fallbackUserInfo)
	}

	// 3. Fallback to sub field
	subUserInfo := &UserInfo{}
	extractAttributes(subUserInfo, map[string]any{
		"sub": "sub-id-999",
	}, ProviderConfig{})
	if subUserInfo.ProviderUserID != "sub-id-999" {
		t.Fatalf("expected sub-id-999, got: %s", subUserInfo.ProviderUserID)
	}
}

func TestOauthSetHTTPClientUnit(t *testing.T) {
	SetHTTPClient(nil)
	if defaultHTTPClient == nil {
		t.Fatal("expected defaultHTTPClient to be initialized")
	}
}

func TestOauthIDTokenVerificationUnit(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := json.Marshal(map[string]any{"keys": []map[string]string{{"kid": "trusted", "kty": "OKP", "crv": tls.Ed25519.String(), "alg": "EdDSA", "x": base64.RawURLEncoding.EncodeToString(publicKey)}}})
	if err != nil {
		t.Fatal(err)
	}
	SetHTTPClient(&mockHTTPClient{doFunc: func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://www.googleapis.com/oauth2/v3/certs" {
			t.Errorf("unexpected privateKey URL: %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(keys)))}, nil
	}})
	defer SetHTTPClient(nil)
	providerConfig := ProviderConfig{Preset: PresetGoogle, ClientID: "client"}
	for _, name := range []string{"valid", "wrong issuer", "wrong audience", "expired", "missing expiration", "wrong signature", "missing subject", "wrong authorized party", "unknown privateKey", "unsigned"} {
		t.Run(name, func(t *testing.T) {
			mapClaims := jwt.MapClaims{"iss": "https://accounts.google.com", "aud": "client", "exp": time.Now().Add(time.Hour).Unix(), "sub": "provider-user"}
			signingPrivateKey := privateKey
			switch name {
			case "wrong issuer":
				mapClaims["iss"] = "https://attacker.example"
			case "wrong audience":
				mapClaims["aud"] = "another-client"
			case "expired":
				mapClaims["exp"] = time.Now().Add(-time.Hour).Unix()
			case "missing expiration":
				delete(mapClaims, "exp")
			case "missing subject":
				delete(mapClaims, "sub")
			case "wrong authorized party":
				mapClaims["azp"] = "another-client"
			case "wrong signature":
				_, signingPrivateKey, err = ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
			}
			token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, mapClaims)
			token.Header["kid"] = "trusted"
			if name == "unknown privateKey" {
				token.Header["kid"] = "attacker"
			}
			raw, err := token.SignedString(signingPrivateKey)
			if err != nil {
				t.Fatal(err)
			}
			if name == "unsigned" {
				raw = makeDummyJWT(`{"iss":"https://accounts.google.com","aud":"client","sub":"user","exp":9999999999}`)
			}
			_, err = verifyIDToken(context.Background(), providerConfig, raw)
			if name == "valid" && err != nil {
				t.Fatal(err)
			}
			if name != "valid" && err == nil {
				t.Fatal("accepted invalid ID token")
			}
		})
	}
	customProviderConfig := ProviderConfig{AuthURL: "https://trusted.example/tenant/authorize", TokenURL: "https://trusted.example/tenant/token", ClientID: "client"}
	if _, err := verifyIDToken(context.Background(), customProviderConfig, makeDummyJWT(`{"iss":"https://attacker.example","sub":"user"}`)); err == nil {
		t.Fatal("accepted untrusted discovery issuer")
	}
}

func TestOauthIDTokenFailurePathsUnit(t *testing.T) {
	ctx := context.Background()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	rsaPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ecPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecBytes, err := ecPrivateKey.PublicKey.Bytes()
	require.NoError(t, err)
	encode := base64.RawURLEncoding.EncodeToString
	baseKey := map[string]string{"kid": "test", "kty": "OKP", "crv": tls.Ed25519.String(), "x": encode(publicKey)}
	providerConfig := ProviderConfig{Name: "custom", AuthURL: "https://issuer.example/authorize", TokenURL: "https://issuer.example/token", ClientID: "client"}
	sign := func(signingMethod jwt.SigningMethod, signingPrivateKey any, multipleAudiences bool) string {
		mapClaims := jwt.MapClaims{"iss": "https://issuer.example", "aud": "client", "exp": time.Now().Add(time.Hour).Unix(), "sub": "user"}
		if multipleAudiences {
			mapClaims["aud"] = []string{"client", "other"}
		}
		token := jwt.NewWithClaims(signingMethod, mapClaims)
		token.Header["kid"] = "test"
		raw, err := token.SignedString(signingPrivateKey)
		require.NoError(t, err)
		return raw
	}
	for _, name := range []string{"discovery network", "discovery invalid", "untrusted keys", "keys network", "keys invalid", "RSA", "RSA wrong algorithm", "EC", "EC wrong algorithm", "multiple audiences"} {
		t.Run(name, func(t *testing.T) {
			key := baseKey
			raw := sign(jwt.SigningMethodEdDSA, privateKey, name == "multiple audiences")
			if name == "RSA" || name == "RSA wrong algorithm" {
				key = map[string]string{"kid": "test", "kty": "RSA", "n": encode(rsaPrivateKey.N.Bytes()), "e": encode(big.NewInt(int64(rsaPrivateKey.E)).Bytes())}
				if name == "RSA" {
					raw = sign(jwt.SigningMethodRS256, rsaPrivateKey, false)
				}
			}
			if name == "EC" || name == "EC wrong algorithm" {
				key = map[string]string{"kid": "test", "kty": "EC", "crv": "P-256", "x": encode(ecBytes[1:33]), "y": encode(ecBytes[33:])}
				if name == "EC" {
					raw = sign(jwt.SigningMethodES256, ecPrivateKey, false)
				}
			}
			SetHTTPClient(&mockHTTPClient{doFunc: func(request *http.Request) (*http.Response, error) {
				if strings.Contains(request.URL.Path, "openid-configuration") {
					if name == "discovery network" {
						return nil, errors.New("discovery unavailable")
					}
					if name == "discovery invalid" {
						return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("invalid"))}, nil
					}
					keysURL := "https://issuer.example/keys"
					if name == "untrusted keys" {
						keysURL = "https://attacker.example/keys"
					}
					body, err := json.Marshal(map[string]string{"issuer": "https://issuer.example", "authorization_endpoint": providerConfig.AuthURL, "token_endpoint": providerConfig.TokenURL, "jwks_uri": keysURL})
					require.NoError(t, err)
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
				}
				if name == "keys network" {
					return nil, errors.New("keys unavailable")
				}
				if name == "keys invalid" {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("invalid"))}, nil
				}
				body, err := json.Marshal(map[string]any{"keys": []map[string]string{key}})
				require.NoError(t, err)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
			}})
			t.Cleanup(func() { SetHTTPClient(nil) })
			_, err := verifyIDToken(ctx, providerConfig, raw)
			if name == "RSA" || name == "EC" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	for _, name := range []string{"invalid ID token", "userinfo subject mismatch"} {
		t.Run(name, func(t *testing.T) {
			raw := sign(jwt.SigningMethodEdDSA, privateKey, false)
			if name == "invalid ID token" {
				raw = "invalid"
			}
			SetHTTPClient(&mockHTTPClient{doFunc: func(request *http.Request) (*http.Response, error) {
				var payload any
				switch request.URL.Path {
				case "/token":
					payload = map[string]string{"id_token": raw, "access_token": "access"}
				case "/keys":
					payload = map[string]any{"keys": []map[string]string{baseKey}}
				case "/userinfo":
					payload = map[string]string{"sub": "different-user"}
				default:
					payload = map[string]string{"issuer": "https://issuer.example", "authorization_endpoint": providerConfig.AuthURL, "token_endpoint": providerConfig.TokenURL, "jwks_uri": "https://issuer.example/keys"}
				}
				body, err := json.Marshal(payload)
				require.NoError(t, err)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
			}})
			t.Cleanup(func() { SetHTTPClient(nil) })
			providerConfig.UserInfoURL = "https://issuer.example/userinfo"
			_, err := ExchangeCodeWithConfig(ctx, providerConfig, "code", "https://app.example/callback")
			require.Error(t, err)
		})
	}
}
