package image

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"uuid"

	"layr.sh/core"
)

// Standard fetcher errors.
var (
	ErrStorageAccessDenied = errors.New("forbidden: access denied to private storage object")
	ErrStorageNotFound     = errors.New("storage object or bucket not found")
	ErrSSRFBlocked         = errors.New("ssrf blocked: host resolves to private or restricted network address")
	ErrDomainNotAllowed    = errors.New("source domain is not in the allowed domains list")
)

const (
	defaultRemoteFetchTimeout = 15 * time.Second
	defaultMaxResponseBytes   = 52428800 // 50MB
)

// Fetcher handles retrieving source assets from either layr/storage or remote HTTP/HTTPS origins.
type Fetcher struct {
	kernel        *core.Kernel
	configManager *ConfigManager
	httpClient    *http.Client
}

// NewFetcher initializes an asset fetcher instance.
func NewFetcher(kernel *core.Kernel, configManager *ConfigManager) *Fetcher {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Fetcher{
		kernel:        kernel,
		configManager: configManager,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   defaultRemoteFetchTimeout,
		},
	}
}

// Fetch loads binary content from local storage or remote origins with SSRF protection.
func (fetcher *Fetcher) Fetch(ctx context.Context, sourceURL string, request *http.Request) (io.ReadCloser, int64, string, error) {
	trimmedSourceURL := strings.TrimSpace(sourceURL)
	if trimmedSourceURL == "" {
		return nil, 0, "", errors.New("source url cannot be empty")
	}

	// 1. Check for protocol schemes
	if strings.Contains(trimmedSourceURL, "://") {
		if !strings.HasPrefix(trimmedSourceURL, "http://") && !strings.HasPrefix(trimmedSourceURL, "https://") {
			return nil, 0, "", fmt.Errorf("unsupported protocol scheme: %s", trimmedSourceURL)
		}
		readCloser, length, contentType, remoteErr := fetcher.fetchRemoteHTTP(ctx, trimmedSourceURL)
		if remoteErr != nil {
			return nil, 0, "", remoteErr
		}
		return readCloser, length, contentType, nil
	}

	// 2. Direct layr/storage Check (<bucket>/<key...>)
	return fetcher.fetchLocalStorage(ctx, trimmedSourceURL, request)
}

func (fetcher *Fetcher) fetchLocalStorage(ctx context.Context, storagePath string, request *http.Request) (io.ReadCloser, int64, string, error) {
	cleanStoragePath := storagePath
	if queryIndex := strings.Index(cleanStoragePath, "?"); queryIndex != -1 {
		cleanStoragePath = cleanStoragePath[:queryIndex]
	}

	pathParts := strings.SplitN(cleanStoragePath, "/", 2)
	if len(pathParts) != 2 || pathParts[0] == "" || pathParts[1] == "" {
		return nil, 0, "", fmt.Errorf("invalid storage path %q, expected <bucket>/<key...>", storagePath)
	}

	bucketName := pathParts[0]
	objectKey := pathParts[1]

	const bucketQuerySQL = `
		SELECT id, is_public, backend, backend_config
		FROM file_storage.buckets
		WHERE name = $1;
	`

	var bucketID uuid.UUID
	var isPublic bool
	var backend string
	var rawBackendConfig []byte

	queryErr := fetcher.kernel.DB().QueryRow(ctx, bucketQuerySQL, bucketName).Scan(
		&bucketID,
		&isPublic,
		&backend,
		&rawBackendConfig,
	)
	if queryErr != nil {
		return nil, 0, "", fmt.Errorf("%w: bucket %q query failed: %v", ErrStorageNotFound, bucketName, queryErr)
	}

	// Authorization Enforcement
	if !isPublic {
		if !fetcher.authorizeStorageAccess(request, bucketName, objectKey, storagePath) {
			return nil, 0, "", ErrStorageAccessDenied
		}
	}

	// Retrieve Object Metadata
	const objectQuerySQL = `
		SELECT id, content_type, size_bytes
		FROM file_storage.objects
		WHERE bucket_id = $1 AND object_key = $2;
	`

	var objectID uuid.UUID
	var contentType string
	var sizeBytes int64

	objectQueryErr := fetcher.kernel.DB().QueryRow(ctx, objectQuerySQL, bucketID, objectKey).Scan(
		&objectID,
		&contentType,
		&sizeBytes,
	)
	if objectQueryErr != nil {
		return nil, 0, "", fmt.Errorf("%w: object %q in bucket %q: %v", ErrStorageNotFound, objectKey, bucketName, objectQueryErr)
	}

	// Database chunk retrieval
	const chunksQuerySQL = `
		SELECT chunk_data
		FROM file_storage.chunks
		WHERE object_id = $1
		ORDER BY chunk_index ASC;
	`

	var assembledBuffer bytes.Buffer
	chunkRows, chunksErr := fetcher.kernel.DB().Query(ctx, chunksQuerySQL, objectID)
	if chunksErr == nil {
		defer chunkRows.Close()
		for chunkRows.Next() {
			var chunkData []byte
			_ = chunkRows.Scan(&chunkData)
			assembledBuffer.Write(chunkData)
		}
		chunksErr = chunkRows.Err()
	}
	if chunksErr != nil {
		return nil, 0, "", fmt.Errorf("failed to query object chunks: %w", chunksErr)
	}

	return io.NopCloser(bytes.NewReader(assembledBuffer.Bytes())), int64(assembledBuffer.Len()), contentType, nil
}

func (fetcher *Fetcher) authorizeStorageAccess(request *http.Request, bucketName string, objectKey string, storagePath string) bool {
	// 1. Presigned Token Query Parameter Check
	queryValues := request.URL.Query()
	if queryValues.Get("token") == "" && strings.Contains(storagePath, "?") {
		if parsedURL, parseURLErr := url.Parse(storagePath); parseURLErr == nil {
			queryValues = parsedURL.Query()
		}
	}

	token := queryValues.Get("token")
	expires := queryValues.Get("expires")
	operation := queryValues.Get("op")
	if token != "" && expires != "" {
		if operation == "read" || operation == "" {
			expiresUnix, parseErr := strconv.ParseInt(expires, 10, 64)
			if parseErr == nil && time.Now().Unix() <= expiresUnix {
				signingKey := fetcher.kernel.CryptoKeyManager().DeriveSubkey("layr-presign-signing-key")
				payload := fmt.Sprintf("%s:%s:read:%d", bucketName, objectKey, expiresUnix)
				hmacHash := hmac.New(sha256.New, signingKey)
				hmacHash.Write([]byte(payload))
				expectedSignature := hex.EncodeToString(hmacHash.Sum(nil))
				if token == expectedSignature {
					return true
				}
			}
		}
	}

	// 2. Service Account Key Header Check
	serviceAccountKey := core.ExtractRequestServiceAccountKey(request)
	if serviceAccountKey != "" {
		clientIP := core.ExtractRequestClientIP(request)
		serviceAccount, authErr := fetcher.kernel.ServiceAccountManager().Authenticate(request.Context(), serviceAccountKey, clientIP)
		if authErr == nil && (core.HasScope(serviceAccount.Scopes, core.ScopeFileStorageObjectRead) || core.HasScope(serviceAccount.Scopes, "*")) {
			return true
		}
	}

	// 3. Authenticated AuthContext Check
	authContext := core.GetAuthContext(request.Context())
	if authContext.IsServiceAccount() {
		if authContext.HasScope(core.ScopeFileStorageObjectRead) || authContext.HasScope("*") {
			return true
		}
	} else if authContext.IsAuthenticated() {
		return true
	}

	return false
}

func (fetcher *Fetcher) fetchRemoteHTTP(ctx context.Context, remoteURL string) (io.ReadCloser, int64, string, error) {
	parsedURL, parseErr := url.Parse(remoteURL)
	if parseErr != nil {
		return nil, 0, "", fmt.Errorf("invalid remote url: %w", parseErr)
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return nil, 0, "", errors.New("missing hostname in remote url")
	}

	// Domain Allowlist Check
	config := fetcher.configManager.Get()
	if len(config.AllowedDomains) > 0 {
		domainAllowed := false
		for _, allowed := range config.AllowedDomains {
			allowed = strings.TrimSpace(strings.ToLower(allowed))
			if allowed == "" {
				continue
			}
			if strings.EqualFold(hostname, allowed) || strings.HasSuffix(strings.ToLower(hostname), "."+allowed) {
				domainAllowed = true
				break
			}
		}
		if !domainAllowed {
			log.Warnf("ssrf blocked: domain %s not allowed for %s", hostname, remoteURL)
			fetcher.kernel.EventBus().Publish(ctx, NewThreatSSRFBlockedEvent(remoteURL, ThreatSSRFBlockedEventData{
				SourceURL: remoteURL,
				Reason:    fmt.Sprintf("domain %s not in allowed list", hostname),
			}))
			return nil, 0, "", fmt.Errorf("%w: %s", ErrDomainNotAllowed, hostname)
		}
	}

	// SSRF Protection: Resolve and Verify IPs
	ipAddresses, lookupErr := net.DefaultResolver.LookupIPAddr(ctx, hostname)
	if lookupErr != nil {
		return nil, 0, "", fmt.Errorf("dns resolution failed for %s: %w", hostname, lookupErr)
	}

	for _, ipAddress := range ipAddresses {
		if isPrivateOrRestrictedIP(ipAddress.IP) {
			log.Warnf("ssrf blocked: host %s resolved to private/restricted ip %s", hostname, ipAddress.IP.String())
			fetcher.kernel.EventBus().Publish(ctx, NewThreatSSRFBlockedEvent(remoteURL, ThreatSSRFBlockedEventData{
				SourceURL:  remoteURL,
				ResolvedIP: ipAddress.IP.String(),
				Reason:     fmt.Sprintf("%s resolves to restricted address %s", hostname, ipAddress.IP.String()),
			}))
			return nil, 0, "", fmt.Errorf("%w: %s resolves to restricted address %s", ErrSSRFBlocked, hostname, ipAddress.IP.String())
		}
	}

	httpRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	httpRequest.Header.Set("User-Agent", "layr-image/1.0")

	httpResponse, doErr := fetcher.httpClient.Do(httpRequest)
	if doErr != nil {
		return nil, 0, "", fmt.Errorf("remote http fetch failed: %w", doErr)
	}

	if httpResponse.StatusCode != http.StatusOK {
		_ = httpResponse.Body.Close()
		return nil, 0, "", fmt.Errorf("remote origin returned status %d", httpResponse.StatusCode)
	}

	contentType := httpResponse.Header.Get("Content-Type")

	// Read limited buffer to protect against unbounded streams
	limitedReader := io.LimitReader(httpResponse.Body, defaultMaxResponseBytes)
	bufferedBytes, readErr := io.ReadAll(limitedReader)
	_ = httpResponse.Body.Close()
	if readErr != nil {
		return nil, 0, "", fmt.Errorf("failed reading remote response body: %w", readErr)
	}

	return io.NopCloser(bytes.NewReader(bufferedBytes)), int64(len(bufferedBytes)), contentType, nil
}

const (
	ipv4TotalMaskBits    = 32
	cgnatNetworkBits     = 10
	linkLocalNetworkBits = 16
)

func isPrivateOrRestrictedIP(targetIP net.IP) bool {
	if targetIP.IsLoopback() || targetIP.IsLinkLocalUnicast() || targetIP.IsLinkLocalMulticast() || targetIP.IsUnspecified() {
		return true
	}

	if targetIP.IsPrivate() {
		return true
	}

	// Carrier Grade NAT: 100.64.0.0/10
	cgnatIPNet := &net.IPNet{
		IP:   net.ParseIP("100.64.0.0"),
		Mask: net.CIDRMask(cgnatNetworkBits, ipv4TotalMaskBits),
	}
	if cgnatIPNet.Contains(targetIP) {
		return true
	}

	// Link-Local: 169.254.0.0/16
	linkLocalIPNet := &net.IPNet{
		IP:   net.ParseIP("169.254.0.0"),
		Mask: net.CIDRMask(linkLocalNetworkBits, ipv4TotalMaskBits),
	}
	return linkLocalIPNet.Contains(targetIP)
}
