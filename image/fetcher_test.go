package image

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
	"layr.sh/filestorage"
)

func TestImageFetcherUnit(t *testing.T) {
	t.Parallel()

	allMigrations := append(filestorage.Migrations, Migrations...)
	kernel, cleanup := core.SetupTestKernel(t, allMigrations)
	defer cleanup()

	configManager := NewConfigManager(kernel)
	fetcher := NewFetcher(kernel, configManager)
	ctx := context.Background()
	httpRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	t.Run("ssrf protection blocks private and loopback addresses", func(t *testing.T) {
		blockedURLs := []string{
			"http://127.0.0.1/test.jpg",
			"http://localhost/test.jpg",
			"http://10.0.0.1/test.jpg",
			"http://192.168.1.5/test.jpg",
			"http://169.254.169.254/latest/meta-data",
			"http://[::1]/test.jpg",
		}

		for _, blockedURL := range blockedURLs {
			_, _, _, fetchErr := fetcher.Fetch(ctx, blockedURL, httpRequest)
			require.Error(t, fetchErr)
			require.True(t, errors.Is(fetchErr, ErrSSRFBlocked), "expected ErrSSRFBlocked for %s, got: %v", blockedURL, fetchErr)
		}
	})

	t.Run("allowed domains restriction", func(t *testing.T) {
		domainConfigManager := NewConfigManager(kernel)
		domainConfigManager.SetMemoryConfig(Config{
			AllowedDomains: []string{"trusted.com", "*.cdn.com"},
		})
		domainFetcher := NewFetcher(kernel, domainConfigManager)

		_, _, _, fetchErr := domainFetcher.Fetch(ctx, "http://untrusted.com/image.jpg", httpRequest)
		require.Error(t, fetchErr)
		require.True(t, errors.Is(fetchErr, ErrDomainNotAllowed))
	})

	t.Run("unsupported protocol scheme", func(t *testing.T) {
		_, _, _, fetchErr := fetcher.Fetch(ctx, "ftp://example.com/file.jpg", httpRequest)
		require.Error(t, fetchErr)
		require.Contains(t, fetchErr.Error(), "unsupported protocol scheme")
	})

	t.Run("local storage missing bucket returns not found", func(t *testing.T) {
		_, _, _, fetchErr := fetcher.Fetch(ctx, "nonexistent-bucket/image.jpg", httpRequest)
		require.Error(t, fetchErr)
		require.True(t, errors.Is(fetchErr, ErrStorageNotFound))
	})

	t.Run("restricted ip detection", func(t *testing.T) {
		restrictedAddresses := []string{
			"127.0.0.1",
			"10.0.0.1",
			"192.168.1.1",
			"172.16.0.1",
			"169.254.1.1",
			"100.64.0.1",
			"::1",
		}
		for _, address := range restrictedAddresses {
			targetIP := net.ParseIP(address)
			require.NotNil(t, targetIP)
			require.True(t, isPrivateOrRestrictedIP(targetIP), "expected %s to be restricted", address)
		}

		publicAddresses := []string{
			"8.8.8.8",
			"1.1.1.1",
			"93.184.216.34",
		}
		for _, address := range publicAddresses {
			targetIP := net.ParseIP(address)
			require.NotNil(t, targetIP)
			require.False(t, isPrivateOrRestrictedIP(targetIP), "expected %s to be public", address)
		}
	})

	t.Run("empty and invalid paths", func(t *testing.T) {
		_, _, _, emptyErr := fetcher.Fetch(ctx, "   ", httpRequest)
		require.Error(t, emptyErr)
		require.Contains(t, emptyErr.Error(), "source url cannot be empty")

		activeKernel := core.NewTestKernel(nil)
		formatFetcher := NewFetcher(activeKernel, configManager)
		_, _, _, invalidPathErr := formatFetcher.Fetch(ctx, "nobucketdivider", httpRequest)
		require.Error(t, invalidPathErr)
		require.Contains(t, invalidPathErr.Error(), "invalid storage path")
	})

	t.Run("remote http url validation and resolution errors", func(t *testing.T) {
		_, _, _, missingHostErr := fetcher.fetchRemoteHTTP(ctx, "http://")
		require.Error(t, missingHostErr)
		require.Contains(t, missingHostErr.Error(), "missing hostname")

		_, _, _, invalidURLErr := fetcher.fetchRemoteHTTP(ctx, "http://[invalid-ipv6")
		require.Error(t, invalidURLErr)
		require.Contains(t, invalidURLErr.Error(), "invalid remote url")

		_, _, _, dnsErr := fetcher.fetchRemoteHTTP(ctx, "http://this-does-not-exist-at-all-987654321.com/test.jpg")
		require.Error(t, dnsErr)
		require.Contains(t, dnsErr.Error(), "dns resolution failed")
	})

	t.Run("remote http mock transport execution", func(t *testing.T) {
		mockTransport := &mockImageTransport{}
		customClientFetcher := NewFetcher(kernel, configManager)
		customClientFetcher.httpClient.Transport = mockTransport

		// 1. Transport error
		mockTransport.roundTripFunc = func(request *http.Request) (*http.Response, error) {
			return nil, errors.New("connection reset by peer")
		}
		// Bypass DNS/SSRF by using 8.8.8.8
		_, _, _, transportErr := customClientFetcher.fetchRemoteHTTP(ctx, "http://8.8.8.8/test.jpg")
		require.Error(t, transportErr)
		require.Contains(t, transportErr.Error(), "remote http fetch failed")

		// 2. Non-200 Status
		mockTransport.roundTripFunc = func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewReader([]byte("not found"))),
			}, nil
		}
		_, _, _, notFoundErr := customClientFetcher.fetchRemoteHTTP(ctx, "http://8.8.8.8/test.jpg")
		require.Error(t, notFoundErr)
		require.Contains(t, notFoundErr.Error(), "remote origin returned status 404")

		// 3. Successful 200 Fetch
		mockTransport.roundTripFunc = func(request *http.Request) (*http.Response, error) {
			responseHeader := make(http.Header)
			responseHeader.Set("Content-Type", "image/png")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     responseHeader,
				Body:       io.NopCloser(bytes.NewReader([]byte("png-binary-stream"))),
			}, nil
		}
		readCloser, length, contentType, successErr := customClientFetcher.fetchRemoteHTTP(ctx, "http://8.8.8.8/test.jpg")
		require.NoError(t, successErr)
		require.Equal(t, "image/png", contentType)
		require.Equal(t, int64(17), length)
		_ = readCloser.Close()

		// 4. Read error on response body
		mockTransport.roundTripFunc = func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(&errorFetcherTestReader{}),
			}, nil
		}
		_, _, _, readErr := customClientFetcher.fetchRemoteHTTP(ctx, "http://8.8.8.8/test.jpg")
		require.Error(t, readErr)
		require.Contains(t, readErr.Error(), "failed reading remote response body")
	})

	t.Run("authorize storage access variants", func(t *testing.T) {
		// Unauthenticated request with no query params
		plainRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
		require.False(t, fetcher.authorizeStorageAccess(plainRequest, "bucket", "key", ""))

		// AuthContext with user role
		userAuthContext := core.AuthContext{
			UserID: "01918a24-7777-7000-8000-000000000001",
		}
		userCtx := core.WithAuthContext(ctx, userAuthContext)
		userRequest := httptest.NewRequestWithContext(userCtx, http.MethodGet, "/test", nil)
		require.True(t, fetcher.authorizeStorageAccess(userRequest, "bucket", "key", ""))

		// AuthContext with service account scope
		serviceAccountAuthContext := core.AuthContext{
			ServiceAccountID: "sa-1",
			JWT: core.JWTClaims{
				Subject: "sa-1",
				Role:    "service_role",
				Scope:   core.ScopeFileStorageObjectRead,
			},
		}
		serviceAccountCtx := core.WithAuthContext(ctx, serviceAccountAuthContext)
		serviceAccountRequest := httptest.NewRequestWithContext(serviceAccountCtx, http.MethodGet, "/test", nil)
		require.True(t, fetcher.authorizeStorageAccess(serviceAccountRequest, "bucket", "key", ""))

		// Service account header check
		createdServiceAccount, createErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
			Name:   "Fetcher Verification Account",
			Scopes: []string{core.ScopeFileStorageObjectRead},
		})
		require.NoError(t, createErr)

		headerRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
		headerRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
		require.True(t, fetcher.authorizeStorageAccess(headerRequest, "bucket", "key", ""))
	})

	t.Run("domain allowlist wildcard matching and empty strings", func(t *testing.T) {
		wildcardConfigManager := NewConfigManager(kernel)
		wildcardConfigManager.SetMemoryConfig(Config{
			AllowedDomains: []string{"", "example.com", "*.cdn.net", "1.1.1.1"},
		})
		wildcardFetcher := NewFetcher(kernel, wildcardConfigManager)

		// Subdomain match
		mockTransport := &mockImageTransport{
			roundTripFunc: func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte("content"))),
				}, nil
			},
		}
		wildcardFetcher.httpClient.Transport = mockTransport

		// Successful fetch through Fetch()
		readCloser, length, contentType, fetchErr := wildcardFetcher.Fetch(ctx, "http://1.1.1.1/test.png", httpRequest)
		require.NoError(t, fetchErr)
		require.NotNil(t, readCloser)
		require.Equal(t, int64(7), length)
		require.Empty(t, contentType)
		_ = readCloser.Close()
	})
}

type mockImageTransport struct {
	roundTripFunc func(request *http.Request) (*http.Response, error)
}

func (mockTransport *mockImageTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return mockTransport.roundTripFunc(request)
}

type errorFetcherTestReader struct{}

func (errorFetcherTestReader) Read(destinationSlice []byte) (int, error) {
	return 0, errors.New("simulated fetcher stream read error")
}
