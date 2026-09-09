// Package oauth provides OAuth 2.0 and OpenID Connect authentication provider integrations.
package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Base64 decoding padding quantum
const base64PaddingModulo = 4

// Supported OAuth Preset Providers
const (
	PresetGoogle  = "google"
	PresetGitHub  = "github"
	PresetApple   = "apple"
	PresetDiscord = "discord"
	PresetCustom  = "custom"
)

// Legacy alias constants for backwards compatibility
const (
	ProviderGoogle  = PresetGoogle
	ProviderGitHub  = PresetGitHub
	ProviderApple   = PresetApple
	ProviderDiscord = PresetDiscord
)

// ProviderPreset defines configuration defaults for known OAuth providers.
type ProviderPreset struct {
	AuthURL         string
	TokenURL        string
	UserInfoURL     string
	DefaultScopes   string
	ResponseMode    string
	ResponseType    string
	IDAttribute     string
	EmailAttribute  string
	NameAttribute   string
	AvatarAttribute string
}

// Presets maps standard provider names to their preset defaults.
var Presets = map[string]ProviderPreset{
	PresetGoogle: {
		AuthURL:         "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:        "https://oauth2.googleapis.com/token",
		UserInfoURL:     "https://openidconnect.googleapis.com/v1/userinfo",
		DefaultScopes:   "openid email profile",
		ResponseType:    "code",
		IDAttribute:     "sub",
		EmailAttribute:  "email",
		NameAttribute:   "name",
		AvatarAttribute: "picture",
	},
	PresetGitHub: {
		AuthURL:         "https://github.com/login/oauth/authorize",
		TokenURL:        "https://github.com/login/oauth/access_token",
		UserInfoURL:     "https://api.github.com/user",
		DefaultScopes:   "read:user user:email",
		ResponseType:    "code",
		IDAttribute:     "id",
		EmailAttribute:  "email",
		NameAttribute:   "name",
		AvatarAttribute: "avatar_url",
	},
	PresetApple: {
		AuthURL:        "https://appleid.apple.com/auth/authorize",
		TokenURL:       "https://appleid.apple.com/auth/token",
		UserInfoURL:    "https://appleid.apple.com/auth/userinfo",
		DefaultScopes:  "name email",
		ResponseMode:   "form_post",
		ResponseType:   "code id_token",
		IDAttribute:    "sub",
		EmailAttribute: "email",
		NameAttribute:  "name",
	},
	PresetDiscord: {
		AuthURL:         "https://discord.com/api/oauth2/authorize",
		TokenURL:        "https://discord.com/api/oauth2/token",
		UserInfoURL:     "https://discord.com/api/users/@me",
		DefaultScopes:   "identify email",
		ResponseType:    "code",
		IDAttribute:     "id",
		EmailAttribute:  "email",
		NameAttribute:   "username",
		AvatarAttribute: "avatar",
	},
}

// ProviderEndpoints defines standard OAuth URLs (legacy compatibility).
type ProviderEndpoints struct {
	AuthURL     string
	TokenURL    string
	UserInfoURL string
}

// Endpoints maps provider names to their endpoints for legacy compatibility.
var Endpoints = map[string]ProviderEndpoints{
	PresetGoogle: {
		AuthURL:     Presets[PresetGoogle].AuthURL,
		TokenURL:    Presets[PresetGoogle].TokenURL,
		UserInfoURL: Presets[PresetGoogle].UserInfoURL,
	},
	PresetGitHub: {
		AuthURL:     Presets[PresetGitHub].AuthURL,
		TokenURL:    Presets[PresetGitHub].TokenURL,
		UserInfoURL: Presets[PresetGitHub].UserInfoURL,
	},
	PresetApple: {
		AuthURL:     Presets[PresetApple].AuthURL,
		TokenURL:    Presets[PresetApple].TokenURL,
		UserInfoURL: Presets[PresetApple].UserInfoURL,
	},
	PresetDiscord: {
		AuthURL:     Presets[PresetDiscord].AuthURL,
		TokenURL:    Presets[PresetDiscord].TokenURL,
		UserInfoURL: Presets[PresetDiscord].UserInfoURL,
	},
}

// ProviderConfig represents dynamic or preset-based OAuth provider config.
type ProviderConfig struct {
	Name            string `json:"name,omitempty"`
	Preset          string `json:"preset,omitempty"`
	AuthURL         string `json:"auth_url,omitempty"`
	TokenURL        string `json:"token_url,omitempty"`
	UserInfoURL     string `json:"userinfo_url,omitempty"`
	ClientID        string `json:"client_id"`
	ClientSecret    string `json:"client_secret,omitempty"`
	Scope           string `json:"scope,omitempty"`
	ResponseMode    string `json:"response_mode,omitempty"`
	ResponseType    string `json:"response_type,omitempty"`
	IDAttribute     string `json:"id_attribute,omitempty"`
	EmailAttribute  string `json:"email_attribute,omitempty"`
	NameAttribute   string `json:"name_attribute,omitempty"`
	AvatarAttribute string `json:"avatar_attribute,omitempty"`
}

// UserInfo represents normalized profile data returned from an OAuth provider.
type UserInfo struct {
	Provider       string         `json:"provider"`
	ProviderUserID string         `json:"provider_user_id"`
	Email          string         `json:"email"`
	Name           string         `json:"name"`
	AvatarURL      string         `json:"avatar_url"`
	Properties     map[string]any `json:"properties"`
}

// ResolveProviderConfig merges preset defaults with custom provider config.
func ResolveProviderConfig(name string, providerConfig ProviderConfig) (ProviderConfig, error) {
	log.Tracef("resolving OAuth provider configuration for %q (preset: %q)", name, providerConfig.Preset)
	resolvedProviderConfig := providerConfig
	if resolvedProviderConfig.Name == "" {
		resolvedProviderConfig.Name = name
	}

	presetKey := resolvedProviderConfig.Preset
	if presetKey == "" {
		presetKey = name
	}

	if providerPreset, ok := Presets[presetKey]; ok {
		if resolvedProviderConfig.Preset == "" {
			resolvedProviderConfig.Preset = presetKey
		}
		if resolvedProviderConfig.AuthURL == "" {
			resolvedProviderConfig.AuthURL = providerPreset.AuthURL
		}
		if resolvedProviderConfig.TokenURL == "" {
			resolvedProviderConfig.TokenURL = providerPreset.TokenURL
		}
		if resolvedProviderConfig.UserInfoURL == "" {
			resolvedProviderConfig.UserInfoURL = providerPreset.UserInfoURL
		}
		if resolvedProviderConfig.Scope == "" {
			resolvedProviderConfig.Scope = providerPreset.DefaultScopes
		}
		if resolvedProviderConfig.ResponseMode == "" {
			resolvedProviderConfig.ResponseMode = providerPreset.ResponseMode
		}
		if resolvedProviderConfig.ResponseType == "" {
			resolvedProviderConfig.ResponseType = providerPreset.ResponseType
		}
		if resolvedProviderConfig.IDAttribute == "" {
			resolvedProviderConfig.IDAttribute = providerPreset.IDAttribute
		}
		if resolvedProviderConfig.EmailAttribute == "" {
			resolvedProviderConfig.EmailAttribute = providerPreset.EmailAttribute
		}
		if resolvedProviderConfig.NameAttribute == "" {
			resolvedProviderConfig.NameAttribute = providerPreset.NameAttribute
		}
		if resolvedProviderConfig.AvatarAttribute == "" {
			resolvedProviderConfig.AvatarAttribute = providerPreset.AvatarAttribute
		}
	}

	if resolvedProviderConfig.ResponseType == "" {
		resolvedProviderConfig.ResponseType = "code"
	}
	if resolvedProviderConfig.AuthURL == "" || resolvedProviderConfig.TokenURL == "" {
		log.Debugf("OAuth provider configuration invalid for %q: missing auth_url or token_url", resolvedProviderConfig.Name)
		return resolvedProviderConfig, fmt.Errorf("oauth provider '%s' requires auth_url and token_url", resolvedProviderConfig.Name)
	}

	return resolvedProviderConfig, nil
}

// BuildAuthorizeURL constructs the OAuth 2.0 redirect URL for the given provider.
func BuildAuthorizeURL(provider, clientID, redirectURI, state, scope string) (string, error) {
	resolvedProviderConfig, err := ResolveProviderConfig(provider, ProviderConfig{
		Name:     provider,
		ClientID: clientID,
		Scope:    scope,
	})
	if err != nil {
		return "", err
	}
	return BuildAuthorizeURLWithConfig(resolvedProviderConfig, redirectURI, state)
}

// BuildAuthorizeURLWithConfig builds authorization URL from a full ProviderConfig.
func BuildAuthorizeURLWithConfig(providerConfig ProviderConfig, redirectURI, state string) (string, error) {
	log.Debugf("building OAuth authorize URL for provider %q (client_id: %s)", providerConfig.Name, providerConfig.ClientID)
	if providerConfig.ClientID == "" {
		log.Debugf("cannot build authorize URL: client_id is required (provider: %q)", providerConfig.Name)
		return "", errors.New("client_id is required")
	}
	if providerConfig.AuthURL == "" {
		log.Debugf("cannot build authorize URL: auth_url is required (provider: %q)", providerConfig.Name)
		return "", errors.New("auth_url is required")
	}

	queryValues := url.Values{}
	queryValues.Set("client_id", providerConfig.ClientID)
	queryValues.Set("redirect_uri", redirectURI)
	responseType := providerConfig.ResponseType
	if responseType == "" {
		responseType = "code"
	}
	queryValues.Set("response_type", responseType)

	if providerConfig.Scope != "" {
		queryValues.Set("scope", providerConfig.Scope)
	}
	if providerConfig.ResponseMode != "" {
		queryValues.Set("response_mode", providerConfig.ResponseMode)
	}
	queryValues.Set("state", state)

	separator := "?"
	if strings.Contains(providerConfig.AuthURL, "?") {
		separator = "&"
	}

	authorizeURL := fmt.Sprintf("%s%s%s", providerConfig.AuthURL, separator, queryValues.Encode())
	log.Tracef("built authorize URL for provider %q: %s", providerConfig.Name, authorizeURL)
	return authorizeURL, nil
}

// HTTPClient interface for test mocking.
type HTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}

var defaultHTTPClient HTTPClient = &http.Client{Timeout: 10 * time.Second}

// SetHTTPClient overrides the HTTP client for testing.
func SetHTTPClient(httpClient HTTPClient) {
	if httpClient == nil {
		log.Debug("resetting default HTTP client for OAuth operations")
		defaultHTTPClient = &http.Client{Timeout: 10 * time.Second}
		return
	}
	log.Debug("configuring custom HTTP client for OAuth operations")
	defaultHTTPClient = httpClient
}

// ExchangeCode exchanges an authorization code for an access token and fetches user info.
func ExchangeCode(ctx context.Context, provider, clientID, clientSecret, code, redirectURI string) (*UserInfo, error) {
	resolvedProviderConfig, err := ResolveProviderConfig(provider, ProviderConfig{
		Name:         provider,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	if err != nil {
		return nil, err
	}
	return ExchangeCodeWithConfig(ctx, resolvedProviderConfig, code, redirectURI)
}

// ExchangeCodeWithConfig exchanges authorization code using resolved ProviderConfig.
func ExchangeCodeWithConfig(ctx context.Context, providerConfig ProviderConfig, code, redirectURI string) (*UserInfo, error) {
	log.Debugf("exchanging OAuth authorization code for provider %q", providerConfig.Name)
	formValues := url.Values{}
	formValues.Set("client_id", providerConfig.ClientID)
	formValues.Set("client_secret", providerConfig.ClientSecret)
	formValues.Set("code", code)
	formValues.Set("grant_type", "authorization_code")
	formValues.Set("redirect_uri", redirectURI)

	tokenRequest, _ := http.NewRequestWithContext(ctx, http.MethodPost, providerConfig.TokenURL, strings.NewReader(formValues.Encode()))
	tokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRequest.Header.Set("Accept", "application/json")

	log.Tracef("dispatching token exchange POST to %s", providerConfig.TokenURL)
	tokenResponse, err := defaultHTTPClient.Do(tokenRequest)
	if err != nil {
		log.Debugf("token exchange network request failed for provider %q: %v", providerConfig.Name, err)
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer func() {
		_ = tokenResponse.Body.Close()
	}()

	if tokenResponse.StatusCode < 200 || tokenResponse.StatusCode >= 300 {
		body, _ := io.ReadAll(tokenResponse.Body)
		log.Debugf("token exchange for provider %q returned HTTP status %d: %s", providerConfig.Name, tokenResponse.StatusCode, string(body))
		return nil, fmt.Errorf("token exchange returned status %d: %s", tokenResponse.StatusCode, string(body))
	}

	var tokenPayload struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}

	if err := json.NewDecoder(tokenResponse.Body).Decode(&tokenPayload); err != nil {
		log.Debugf("failed to parse token response JSON for provider %q: %v", providerConfig.Name, err)
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenPayload.Error != "" {
		log.Debugf("OAuth provider %q returned error payload: %s", providerConfig.Name, tokenPayload.Error)
		return nil, fmt.Errorf("oauth error: %s", tokenPayload.Error)
	}
	if tokenPayload.AccessToken == "" && tokenPayload.IDToken == "" {
		log.Debugf("OAuth provider %q response contained empty access_token and id_token", providerConfig.Name)
		return nil, errors.New("empty token in oauth response")
	}

	rawProperties := make(map[string]any)

	// If ID token is present (e.g. Apple / OIDC), parse claims as baseline
	if tokenPayload.IDToken != "" {
		log.Tracef("parsing ID token claims for provider %q", providerConfig.Name)
		if claims := parseIDToken(tokenPayload.IDToken); claims != nil {
			for key, claimValue := range claims {
				rawProperties[key] = claimValue
			}
		}
	}

	// Fetch user profile if UserInfoURL is configured and AccessToken is present
	if providerConfig.UserInfoURL != "" && tokenPayload.AccessToken != "" {
		log.Tracef("fetching user profile from %s for provider %q", providerConfig.UserInfoURL, providerConfig.Name)
		userInfoRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, providerConfig.UserInfoURL, nil)
		userInfoRequest.Header.Set("Authorization", "Bearer "+tokenPayload.AccessToken)
		userInfoRequest.Header.Set("Accept", "application/json")

		userInfoResponse, err := defaultHTTPClient.Do(userInfoRequest)
		if err != nil {
			log.Debugf("failed to fetch user profile from %s: %v", providerConfig.UserInfoURL, err)
			return nil, fmt.Errorf("failed to fetch user profile: %w", err)
		}
		defer func() {
			_ = userInfoResponse.Body.Close()
		}()

		if userInfoResponse.StatusCode < 200 || userInfoResponse.StatusCode >= 300 {
			log.Debugf("user info endpoint %s returned HTTP status %d", providerConfig.UserInfoURL, userInfoResponse.StatusCode)
			return nil, fmt.Errorf("user info returned status %d", userInfoResponse.StatusCode)
		}

		var userInfoMap map[string]any
		if err := json.NewDecoder(userInfoResponse.Body).Decode(&userInfoMap); err != nil {
			log.Debugf("failed to parse user info JSON from %s: %v", providerConfig.UserInfoURL, err)
			return nil, fmt.Errorf("failed to parse user info json: %w", err)
		}
		for key, attributeValue := range userInfoMap {
			rawProperties[key] = attributeValue
		}
	}

	userInfo := &UserInfo{
		Provider:   providerConfig.Name,
		Properties: rawProperties,
	}

	// Extract attributes using configured or preset attribute names
	extractAttributes(userInfo, rawProperties, providerConfig)

	if userInfo.ProviderUserID == "" {
		log.Debugf("could not extract provider user id from OAuth profile for provider %q", providerConfig.Name)
		return nil, errors.New("could not extract provider user id from oauth profile")
	}

	log.Debugf("successfully normalized OAuth user info for provider %q (user_id: %s, email: %s)", userInfo.Provider, userInfo.ProviderUserID, userInfo.Email)
	return userInfo, nil
}

func parseIDToken(idToken string) map[string]any {
	tokenSegments := strings.Split(idToken, ".")
	if len(tokenSegments) < 2 {
		log.Tracef("ID token format invalid: segment count %d < 2", len(tokenSegments))
		return nil
	}
	payloadSegment := tokenSegments[1]
	if remainder := len(payloadSegment) % base64PaddingModulo; remainder > 0 {
		payloadSegment += strings.Repeat("=", base64PaddingModulo-remainder)
	}
	decoded, err := base64.URLEncoding.DecodeString(payloadSegment)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(payloadSegment)
		if err != nil {
			log.Tracef("failed to decode ID token base64 payload: %v", err)
			return nil
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		log.Tracef("failed to parse ID token claims JSON: %v", err)
		return nil
	}
	log.Tracef("successfully parsed %d claims from ID token", len(claims))
	return claims
}

func extractAttributes(userInfo *UserInfo, rawProperties map[string]any, providerConfig ProviderConfig) {
	log.Tracef("extracting user attributes for provider %q", providerConfig.Name)
	// ID Attribute
	idAttribute := providerConfig.IDAttribute
	if idAttribute == "" {
		idAttribute = "sub"
	}
	if attributeValue, ok := rawProperties[idAttribute]; ok {
		userInfo.ProviderUserID = fmt.Sprintf("%v", attributeValue)
	} else if attributeValue, ok := rawProperties["id"]; ok {
		log.Tracef("falling back to 'id' property for provider user ID in %q", providerConfig.Name)
		userInfo.ProviderUserID = fmt.Sprintf("%v", attributeValue)
	} else if attributeValue, ok := rawProperties["sub"]; ok {
		log.Tracef("falling back to 'sub' property for provider user ID in %q", providerConfig.Name)
		userInfo.ProviderUserID = fmt.Sprintf("%v", attributeValue)
	}

	// Email Attribute
	emailAttribute := providerConfig.EmailAttribute
	if emailAttribute == "" {
		emailAttribute = "email"
	}
	if attributeValue, ok := rawProperties[emailAttribute].(string); ok {
		userInfo.Email = attributeValue
	}

	// Name Attribute
	nameAttribute := providerConfig.NameAttribute
	if nameAttribute == "" {
		nameAttribute = "name"
	}
	if attributeValue, ok := rawProperties[nameAttribute].(string); ok {
		userInfo.Name = attributeValue
	} else if attributeValue, ok := rawProperties["username"].(string); ok {
		log.Tracef("falling back to 'username' property for user name in %q", providerConfig.Name)
		userInfo.Name = attributeValue
	}

	// Avatar Attribute
	avatarAttribute := providerConfig.AvatarAttribute
	if avatarAttribute == "" {
		avatarAttribute = "avatar_url"
	}
	if attributeValue, ok := rawProperties[avatarAttribute].(string); ok {
		userInfo.AvatarURL = attributeValue
	} else if attributeValue, ok := rawProperties["picture"].(string); ok {
		log.Tracef("falling back to 'picture' property for avatar in %q", providerConfig.Name)
		userInfo.AvatarURL = attributeValue
	}
}
