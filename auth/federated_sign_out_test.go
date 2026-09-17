package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"layr.sh/core"
)

type mockSignOutHTTPClient struct {
	doFunc func(request *http.Request) (*http.Response, error)
}

func (mockClient *mockSignOutHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return mockClient.doFunc(request)
}

func TestAuthFederatedSignOutUnit(t *testing.T) {
	ctx := context.Background()
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}
	jwtSigner, err := core.NewJWTSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create jwt signer: %v", err)
	}

	clients := []OIDCClientConfig{
		{
			ClientID:                           "client-success",
			BackChannelSignOutURI:              "https://example.com/backchannel-success",
			BackChannelSignOutSessionRequired:  true,
			FrontChannelSignOutURI:             "https://example.com/frontchannel-success",
			FrontChannelSignOutSessionRequired: true,
		},
		{
			ClientID:                           "client-fail-500",
			BackChannelSignOutURI:              "https://example.com/backchannel-500",
			FrontChannelSignOutURI:             "https://example.com/frontchannel-duplicate",
			FrontChannelSignOutSessionRequired: false,
		},
		{
			ClientID:               "client-network-error",
			BackChannelSignOutURI:  "https://example.com/backchannel-error",
			FrontChannelSignOutURI: "https://example.com/frontchannel-duplicate", // Duplicate URL for deduplication test
		},
		{
			ClientID:               "client-no-uris",
			BackChannelSignOutURI:  "",
			FrontChannelSignOutURI: "",
		},
		{
			ClientID:               "client-invalid-front-uri",
			FrontChannelSignOutURI: "::invalid-url::",
		},
		{
			ClientID:              "client-invalid-back-uri",
			BackChannelSignOutURI: "::invalid-url::",
		},
	}

	targets := []ClientSessionInfo{
		{ClientID: "client-success", SessionID: "session-1", UserID: "user-1"},
		{ClientID: "client-fail-500", SessionID: "session-2", UserID: "user-1"},
		{ClientID: "client-network-error", SessionID: "session-3", UserID: "user-1"},
		{ClientID: "client-no-uris", SessionID: "session-4", UserID: "user-1"},
		{ClientID: "client-not-registered", SessionID: "session-5", UserID: "user-1"},
		{ClientID: "client-invalid-front-uri", SessionID: "session-6", UserID: "user-1"},
		{ClientID: "client-invalid-back-uri", SessionID: "session-7", UserID: "user-1"},
		{ClientID: "client-success", SessionID: "", UserID: ""},
	}

	// 1. Guard clauses for dispatchBackChannelSignOut
	dispatchBackChannelSignOut(ctx, nil, jwtSigner, clients, targets)
	mockClient := &mockSignOutHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("OK"))}, nil
		},
	}
	dispatchBackChannelSignOut(ctx, mockClient, nil, clients, targets)
	dispatchBackChannelSignOut(ctx, mockClient, jwtSigner, clients, nil)

	// 2. Dispatch back-channel with mock responses
	var postedBodies []string
	dispatchMockClient := &mockSignOutHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			bodyBytes, readErr := io.ReadAll(request.Body)
			if readErr != nil {
				return nil, fmt.Errorf("reading request body: %w", readErr)
			}
			postedBodies = append(postedBodies, string(bodyBytes))

			if strings.Contains(request.URL.String(), "backchannel-500") {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("Internal Server Error")),
				}, nil
			}
			if strings.Contains(request.URL.String(), "backchannel-error") {
				return nil, errors.New("network failure simulation")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("OK")),
			}, nil
		},
	}

	dispatchBackChannelSignOut(ctx, dispatchMockClient, jwtSigner, clients, targets)
	if len(postedBodies) != 3 {
		t.Fatalf("expected 3 outbound requests dispatched, got: %d", len(postedBodies))
	}
	if !strings.Contains(postedBodies[0], "logout_token=") {
		t.Fatalf("expected logout_token form value in payload, got: %s", postedBodies[0])
	}

	// 3. Test front-channel URL construction
	frontChannelURLs := buildFrontChannelSignOutURLs("https://layr.example.com", clients, targets[:6])
	// Expected: frontchannel-success and frontchannel-duplicate (deduplicated), invalid-url skipped, no-uris skipped, not-registered skipped
	if len(frontChannelURLs) != 2 {
		t.Fatalf("expected 2 unique front channel URLs, got: %d (%v)", len(frontChannelURLs), frontChannelURLs)
	}

	foundSuccess := false
	foundDuplicate := false
	for _, frontChannelURL := range frontChannelURLs {
		if strings.Contains(frontChannelURL, "frontchannel-success") {
			foundSuccess = true
			if !strings.Contains(frontChannelURL, "iss=https%3A%2F%2Flayr.example.com") || !strings.Contains(frontChannelURL, "sid=session-1") {
				t.Fatalf("missing iss or sid in success url: %s", frontChannelURL)
			}
		}
		if strings.Contains(frontChannelURL, "frontchannel-duplicate") {
			foundDuplicate = true
		}
	}

	if !foundSuccess || !foundDuplicate {
		t.Fatalf("expected both success and duplicate urls in result: %v", frontChannelURLs)
	}

	// Test front-channel with target lacking session ID
	noSessionTargets := []ClientSessionInfo{
		{ClientID: "client-success", SessionID: "", UserID: "user-1"},
	}
	noSessionURLs := buildFrontChannelSignOutURLs("https://layr.example.com", clients, noSessionTargets)
	if len(noSessionURLs) != 1 || strings.Contains(noSessionURLs[0], "sid=") {
		t.Fatalf("expected 1 url without sid parameter, got: %v", noSessionURLs)
	}
}
