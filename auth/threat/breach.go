// Package threat provides advanced threat protection and fraud mitigation features.
package threat

import (
	"context"
	"crypto/sha1" //nolint:gosec
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"layr.sh/core"
)

// HTTPClient interface for mockable HTTP outbound requests.
type HTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}

const (
	defaultCacheTTL       = 24 * time.Hour
	defaultHTTPTimeout    = 5 * time.Second
	pwnedHashPrefixLength = 5
)

var pwnedPasswordsAPIBaseURL = "https://api.pwnedpasswords.com/range/"

// CheckPwnedPassword checks if a password exists in the HaveIBeenPwned database using the k-Anonymity model.
// Returns isBreached, breachCount, and error.
func CheckPwnedPassword(ctx context.Context, httpClient HTTPClient, kvStore *core.KVStore, password string, failOpen bool) (bool, int64, error) {
	if len(password) == 0 {
		return false, 0, nil
	}

	sha1Hash := sha1.New() //nolint:gosec
	sha1Hash.Write([]byte(password))
	sha1Hex := strings.ToUpper(hex.EncodeToString(sha1Hash.Sum(nil)))

	prefix := sha1Hex[:pwnedHashPrefixLength]
	suffix := sha1Hex[pwnedHashPrefixLength:]
	cacheKey := fmt.Sprintf("auth:threat:pwned:%s", prefix)

	if kvStore != nil {
		cachedContent, getErr := kvStore.Get(ctx, cacheKey)
		if getErr == nil && cachedContent != "" {
			isBreached, count := searchSuffix(cachedContent, suffix)
			return isBreached, count, nil
		}
	}

	requestURL := pwnedPasswordsAPIBaseURL + prefix
	httpRequest, createErr := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if createErr != nil {
		return false, 0, fmt.Errorf("failed to create breach check request: %w", createErr)
	}

	httpRequest.Header.Set("Add-Padding", "true")
	httpRequest.Header.Set("User-Agent", "Layr-Auth-Threat-Protection/1.0")

	activeHTTPClient := httpClient
	if activeHTTPClient == nil {
		activeHTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	httpResponse, doErr := activeHTTPClient.Do(httpRequest)
	if doErr != nil {
		if failOpen {
			log.Warnf("HaveIBeenPwned API unreachable (%v); fail-open allowing password", doErr)
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to query HaveIBeenPwned API: %w", doErr)
	}

	defer func() {
		_ = httpResponse.Body.Close()
	}()

	if httpResponse.StatusCode != http.StatusOK {
		if failOpen {
			log.Warnf("HaveIBeenPwned API returned unexpected status %d; fail-open allowing password", httpResponse.StatusCode)
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("HaveIBeenPwned API returned status %d", httpResponse.StatusCode)
	}

	bodyBytes, readErr := io.ReadAll(httpResponse.Body)
	if readErr != nil {
		if failOpen {
			log.Warnf("failed to read HaveIBeenPwned response (%v); fail-open allowing password", readErr)
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to read HaveIBeenPwned response: %w", readErr)
	}

	responseString := string(bodyBytes)
	if kvStore != nil {
		_ = kvStore.Set(ctx, cacheKey, responseString, defaultCacheTTL)
	}

	isBreached, count := searchSuffix(responseString, suffix)
	return isBreached, count, nil
}

func searchSuffix(content string, suffix string) (bool, int64) {
	lines := strings.Split(content, "\n")
	for _, rawLine := range lines {
		trimmedLine := strings.TrimSpace(rawLine)
		if trimmedLine == "" {
			continue
		}
		tokens := strings.SplitN(trimmedLine, ":", 2)
		if len(tokens) == 2 && strings.EqualFold(tokens[0], suffix) {
			count, parseErr := strconv.ParseInt(tokens[1], 10, 64)
			if parseErr != nil {
				count = 1
			}
			return true, count
		}
	}
	return false, 0
}
