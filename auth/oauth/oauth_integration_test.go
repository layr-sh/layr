package oauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockHTTPClient struct {
	doFunc func(request *http.Request) (*http.Response, error)
}

func (mock *mockHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return mock.doFunc(request)
}

func makeDummyJWT(claimsJSON string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(claimsJSON))
	signature := base64.RawURLEncoding.EncodeToString([]byte("signature"))
	return fmt.Sprintf("%s.%s.%s", header, payload, signature)
}

func TestOauthExchangeCodeIntegration(t *testing.T) {
	ctx := context.Background()

	// Unsupported provider error
	if _, err := ExchangeCode(ctx, "unknown", "id", "sec", "code", "cb"); err == nil {
		t.Fatalf("expected error on unknown provider")
	}

	// 1. Success Google Exchange
	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"mock-token","token_type":"Bearer"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sub":"google-user-123","email":"test@gmail.com","name":"Google User","picture":"https://avatar.png"}`)),
			}, nil
		},
	})
	defer SetHTTPClient(nil)

	userInfo, err := ExchangeCode(ctx, PresetGoogle, "id", "sec", "code", "cb")
	if err != nil || userInfo.ProviderUserID != "google-user-123" || userInfo.Email != "test@gmail.com" || userInfo.AvatarURL != "https://avatar.png" {
		t.Fatalf("unexpected user info: %+v (err: %v)", userInfo, err)
	}

	// 2. Success GitHub Exchange
	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "access_token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"gh-token"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"id":98765,"email":"gh@example.com","name":"GitHub User","avatar_url":"https://gh.png"}`)),
			}, nil
		},
	})

	userInfo, err = ExchangeCode(ctx, PresetGitHub, "id", "sec", "code", "cb")
	if err != nil || userInfo.ProviderUserID != "98765" || userInfo.Email != "gh@example.com" {
		t.Fatalf("unexpected GitHub user info: %+v (err: %v)", userInfo, err)
	}

	// 3. Success Discord Exchange
	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"dc-token"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"id":"discord-456","email":"dc@example.com","username":"DiscordUser","avatar":"https://discord.png"}`)),
			}, nil
		},
	})

	userInfo, err = ExchangeCode(ctx, PresetDiscord, "id", "sec", "code", "cb")
	if err != nil || userInfo.ProviderUserID != "discord-456" || userInfo.Name != "DiscordUser" || userInfo.AvatarURL != "https://discord.png" {
		t.Fatalf("unexpected Discord user info: %+v (err: %v)", userInfo, err)
	}

	// 4. Success Apple Exchange with ID Token
	appleJWT := makeDummyJWT(`{"sub":"apple-sub-789","email":"apple@icloud.com","name":"Apple User"}`)
	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"id_token":"%s","token_type":"Bearer"}`, appleJWT))),
			}, nil
		},
	})

	userInfo, err = ExchangeCode(ctx, PresetApple, "id", "sec", "code", "cb")
	if err != nil || userInfo.ProviderUserID != "apple-sub-789" || userInfo.Email != "apple@icloud.com" {
		t.Fatalf("unexpected Apple user info: %+v (err: %v)", userInfo, err)
	}

	// 5. Custom Provider with custom attribute mappings
	customProviderConfig := ProviderConfig{
		Name:            "custom-corp",
		AuthURL:         "https://corp.com/oauth/auth",
		TokenURL:        "https://corp.com/oauth/token",
		UserInfoURL:     "https://corp.com/oauth/user",
		ClientID:        "corp-id",
		ClientSecret:    "corp-secret",
		IDAttribute:     "uid",
		EmailAttribute:  "mail",
		NameAttribute:   "display_name",
		AvatarAttribute: "photo_url",
	}

	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"corp-token"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"uid":"corp-999","mail":"corp@company.com","display_name":"Corp User","photo_url":"https://photo.png"}`)),
			}, nil
		},
	})

	userInfo, err = ExchangeCodeWithConfig(ctx, customProviderConfig, "code1", "cb1")
	if err != nil || userInfo.ProviderUserID != "corp-999" || userInfo.Email != "corp@company.com" || userInfo.Name != "Corp User" || userInfo.AvatarURL != "https://photo.png" {
		t.Fatalf("unexpected custom user info: %+v (err: %v)", userInfo, err)
	}

	// 6. Custom Provider with fallback attributes
	fallbackProviderConfig := ProviderConfig{
		Name:         "fallback-idp",
		AuthURL:      "https://fallback.com/auth",
		TokenURL:     "https://fallback.com/token",
		UserInfoURL:  "https://fallback.com/user",
		ClientID:     "fb-id",
		ClientSecret: "fb-sec",
	}
	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"fb-token"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"id":"fallback-user-id","email":"fb@example.com","username":"fbuser","picture":"https://fb.png"}`)),
			}, nil
		},
	})
	userInfo, err = ExchangeCodeWithConfig(ctx, fallbackProviderConfig, "code-fb", "cb-fb")
	if err != nil || userInfo.ProviderUserID != "fallback-user-id" || userInfo.Name != "fbuser" || userInfo.AvatarURL != "https://fb.png" {
		t.Fatalf("unexpected fallback user info: %+v (err: %v)", userInfo, err)
	}

	// 7. Custom Provider with sub fallback
	subFallbackProviderConfig := ProviderConfig{
		Name:         "sub-idp",
		AuthURL:      "https://sub.com/auth",
		TokenURL:     "https://sub.com/token",
		UserInfoURL:  "https://sub.com/user",
		ClientID:     "sub-id",
		ClientSecret: "sub-sec",
		IDAttribute:  "custom_missing",
	}
	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"sub-token"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sub":"sub-fallback-123","email":"sub@example.com"}`)),
			}, nil
		},
	})
	userInfo, err = ExchangeCodeWithConfig(ctx, subFallbackProviderConfig, "code-sub", "cb-sub")
	if err != nil || userInfo.ProviderUserID != "sub-fallback-123" {
		t.Fatalf("unexpected sub fallback user info: %+v (err: %v)", userInfo, err)
	}

	// 8. Error handling
	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return nil, errors.New("network error")
		},
	})
	if _, err := ExchangeCode(ctx, PresetGoogle, "id", "sec", "code", "cb"); err == nil {
		t.Fatalf("expected network error")
	}

	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"invalid_grant"}`))),
			}, nil
		},
	})
	if _, err := ExchangeCode(ctx, PresetGoogle, "id", "sec", "code", "cb"); err == nil {
		t.Fatalf("expected error on 400 token response")
	}

	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`invalid-json`))),
			}, nil
		},
	})
	if _, err := ExchangeCode(ctx, PresetGoogle, "id", "sec", "code", "cb"); err == nil {
		t.Fatalf("expected error on invalid json")
	}

	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"access_denied"}`))),
			}, nil
		},
	})
	if _, err := ExchangeCode(ctx, PresetGoogle, "id", "sec", "code", "cb"); err == nil {
		t.Fatalf("expected error on oauth error field")
	}

	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"access_token":""}`))),
			}, nil
		},
	})
	if _, err := ExchangeCode(ctx, PresetGoogle, "id", "sec", "code", "cb"); err == nil {
		t.Fatalf("expected error on empty token")
	}

	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"valid"}`)),
				}, nil
			}
			return nil, errors.New("user info network error")
		},
	})
	if _, err := ExchangeCode(ctx, PresetGoogle, "id", "sec", "code", "cb"); err == nil {
		t.Fatalf("expected error on user info network error")
	}

	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"valid"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(`unauthorized`)),
			}, nil
		},
	})
	if _, err := ExchangeCode(ctx, PresetGoogle, "id", "sec", "code", "cb"); err == nil {
		t.Fatalf("expected error on user info 401")
	}

	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"valid"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`!bad-json!`)),
			}, nil
		},
	})
	if _, err := ExchangeCode(ctx, PresetGoogle, "id", "sec", "code", "cb"); err == nil {
		t.Fatalf("expected error on user info bad json")
	}

	SetHTTPClient(&mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"valid"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		},
	})
	if _, err := ExchangeCode(ctx, PresetGoogle, "id", "sec", "code", "cb"); err == nil {
		t.Fatalf("expected error on missing user id")
	}
}
