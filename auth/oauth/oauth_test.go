package oauth

import (
	"encoding/base64"
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
