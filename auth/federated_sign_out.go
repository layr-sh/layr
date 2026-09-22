package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"layr.sh/core"
)

const defaultOutboundSignOutTimeout = 5 * time.Second

// HTTPClient defines an injectable interface for making outbound HTTP requests (supports mocking in tests).
type HTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}

// ClientSessionInfo encapsulates target client and session context for sign-out dispatch.
type ClientSessionInfo struct {
	ClientID  string
	SessionID string
	UserID    string
}

// dispatchBackChannelSignOut sends signed OIDC Back-Channel Sign-Out tokens via HTTP POST to registered client URIs.
func dispatchBackChannelSignOut(
	ctx context.Context,
	httpClient HTTPClient,
	jwtSigner *core.JWTSigner,
	clients []OIDCClientConfig,
	targets []ClientSessionInfo,
) {
	if httpClient == nil || len(targets) == 0 {
		return
	}

	clientMap := make(map[string]OIDCClientConfig, len(clients))
	for _, oidcClientConfig := range clients {
		clientMap[oidcClientConfig.ClientID] = oidcClientConfig
	}

	for _, target := range targets {
		oidcClientConfig, exists := clientMap[target.ClientID]
		if !exists || oidcClientConfig.BackChannelSignOutURI == "" {
			continue
		}

		sessionID := target.SessionID
		signOutToken, err := jwtSigner.GenerateSignOutToken(target.UserID, sessionID, oidcClientConfig.ClientID)
		if err != nil {
			log.Warnf("failed to generate back-channel sign-out token for client %s: %v", oidcClientConfig.ClientID, err)
			continue
		}

		formValues := url.Values{
			"logout_token": {signOutToken},
		}

		requestCtx, cancel := context.WithTimeout(ctx, defaultOutboundSignOutTimeout)
		httpRequest, err := http.NewRequestWithContext(requestCtx, http.MethodPost, oidcClientConfig.BackChannelSignOutURI, strings.NewReader(formValues.Encode()))
		if err != nil {
			cancel()
			log.Warnf("failed to construct back-channel sign-out request for client %s: %v", oidcClientConfig.ClientID, err)
			continue
		}
		httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		httpRequest.Header.Set("Cache-Control", "no-cache, no-store")
		httpRequest.Header.Set("Pragma", "no-cache")

		httpResponse, err := httpClient.Do(httpRequest)
		cancel()
		if err != nil {
			log.Warnf("back-channel sign-out HTTP POST to %s failed: %v", oidcClientConfig.BackChannelSignOutURI, err)
			continue
		}
		_ = httpResponse.Body.Close()

		if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
			log.Warnf("back-channel sign-out HTTP POST to %s returned non-2xx status: %d", oidcClientConfig.BackChannelSignOutURI, httpResponse.StatusCode)
		} else {
			log.Debugf("back-channel sign-out delivered successfully to client %s (%s)", oidcClientConfig.ClientID, oidcClientConfig.BackChannelSignOutURI)
		}
	}
}

// buildFrontChannelSignOutURLs constructs front-channel iframe URLs with iss and sid parameters.
func buildFrontChannelSignOutURLs(
	baseURL string,
	clients []OIDCClientConfig,
	targets []ClientSessionInfo,
) []string {
	normalizedBaseURL := strings.TrimRight(baseURL, "/")
	clientMap := make(map[string]OIDCClientConfig, len(clients))
	for _, oidcClientConfig := range clients {
		clientMap[oidcClientConfig.ClientID] = oidcClientConfig
	}

	seenURLs := make(map[string]bool)
	frontChannelURLs := make([]string, 0)

	for _, target := range targets {
		oidcClientConfig, exists := clientMap[target.ClientID]
		if !exists || oidcClientConfig.FrontChannelSignOutURI == "" {
			continue
		}

		parsedURL, err := url.Parse(oidcClientConfig.FrontChannelSignOutURI)
		if err != nil {
			continue
		}

		queryValues := parsedURL.Query()
		queryValues.Set("iss", normalizedBaseURL)
		if oidcClientConfig.FrontChannelSignOutSessionRequired && target.SessionID != "" {
			queryValues.Set("sid", target.SessionID)
		}
		parsedURL.RawQuery = queryValues.Encode()

		signOutURL := parsedURL.String()
		if !seenURLs[signOutURL] {
			seenURLs[signOutURL] = true
			frontChannelURLs = append(frontChannelURLs, signOutURL)
		}
	}

	return frontChannelURLs
}
