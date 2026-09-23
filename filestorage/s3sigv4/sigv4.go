// Package s3sigv4 implements AWS Signature Version 4 authentication and validation.
package s3sigv4

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
	"layr.sh/logger"
)

var log = logger.New("filestorage/s3sigv4")

// Standard S3 SigV4 error definitions.
var (
	ErrMissingAuthHeader     = errors.New("missing or invalid authorization header")
	ErrInvalidAlgorithm      = errors.New("unsupported signature algorithm, must be AWS4-HMAC-SHA256")
	ErrInvalidAccessKeyID    = errors.New("invalid access key id")
	ErrSignatureDoesNotMatch = errors.New("signature does not match")
	ErrRequestExpired        = errors.New("request timestamp outside allowed replay window")
	ErrInvalidTimestamp      = errors.New("invalid request timestamp format")
)

// Validator validates AWS Signature Version 4 on incoming HTTP requests.
type Validator struct {
	kernel *core.Kernel
}

// NewValidator initializes a new SigV4 validator.
func NewValidator(kernel *core.Kernel) *Validator {
	return &Validator{
		kernel: kernel,
	}
}

type authCredentials struct {
	AccessKeyID      string
	Date             string
	Region           string
	Service          string
	SignedHeaders    string
	Signature        string
	RequestTimestamp string
	CredentialScope  string
}

func parseAuthorizationHeader(header string) (*authCredentials, error) {
	if !strings.HasPrefix(header, "AWS4-HMAC-SHA256 ") {
		return nil, ErrInvalidAlgorithm
	}

	parts := strings.Split(strings.TrimPrefix(header, "AWS4-HMAC-SHA256 "), ",")
	parsedCredentials := &authCredentials{}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) != 2 {
			continue
		}
		key := strings.TrimSpace(keyValue[0])
		value := strings.TrimSpace(keyValue[1])

		switch key {
		case "Credential":
			credentialParts := strings.Split(value, "/")
			if len(credentialParts) != 5 || credentialParts[4] != "aws4_request" {
				return nil, fmt.Errorf("malformed Credential parameter")
			}
			parsedCredentials.AccessKeyID = credentialParts[0]
			parsedCredentials.Date = credentialParts[1]
			parsedCredentials.Region = credentialParts[2]
			parsedCredentials.Service = credentialParts[3]
			parsedCredentials.CredentialScope = strings.Join(credentialParts[1:], "/")
		case "SignedHeaders":
			parsedCredentials.SignedHeaders = value
		case "Signature":
			parsedCredentials.Signature = value
		}
	}

	if parsedCredentials.AccessKeyID == "" || parsedCredentials.Signature == "" || parsedCredentials.SignedHeaders == "" {
		return nil, ErrMissingAuthHeader
	}

	return parsedCredentials, nil
}

func parseQueryParams(values url.Values) (*authCredentials, error) {
	algorithm := values.Get("X-Amz-Algorithm")
	if algorithm != "AWS4-HMAC-SHA256" {
		return nil, ErrInvalidAlgorithm
	}

	credential := values.Get("X-Amz-Credential")
	credentialParts := strings.Split(credential, "/")
	if len(credentialParts) != 5 || credentialParts[4] != "aws4_request" {
		return nil, fmt.Errorf("malformed X-Amz-Credential query parameter")
	}

	date := values.Get("X-Amz-Date")
	signedHeaders := values.Get("X-Amz-SignedHeaders")
	signature := values.Get("X-Amz-Signature")

	if date == "" || signedHeaders == "" || signature == "" {
		return nil, ErrMissingAuthHeader
	}

	return &authCredentials{
		AccessKeyID:      credentialParts[0],
		Date:             credentialParts[1],
		Region:           credentialParts[2],
		Service:          credentialParts[3],
		CredentialScope:  strings.Join(credentialParts[1:], "/"),
		SignedHeaders:    signedHeaders,
		Signature:        signature,
		RequestTimestamp: date,
	}, nil
}

func hmacSHA256(key []byte, data []byte) []byte {
	hmacHash := hmac.New(sha256.New, key)
	hmacHash.Write(data)
	return hmacHash.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func buildCanonicalHeaders(request *http.Request, signedHeaders string) string {
	headersList := strings.Split(signedHeaders, ";")
	var builder strings.Builder

	for _, headerName := range headersList {
		headerNameLower := strings.ToLower(strings.TrimSpace(headerName))
		var value string
		if headerNameLower == "host" {
			value = request.Host
			if value == "" {
				value = request.URL.Host
			}
		} else {
			headerValues := request.Header.Values(headerNameLower)
			if len(headerValues) == 0 {
				headerValues = request.Header.Values(headerName)
			}
			trimmedValues := make([]string, len(headerValues))
			for index, headerValue := range headerValues {
				trimmedValues[index] = strings.TrimSpace(headerValue)
			}
			value = strings.Join(trimmedValues, ",")
		}
		builder.WriteString(headerNameLower)
		builder.WriteString(":")
		builder.WriteString(value)
		builder.WriteString("\n")
	}

	return builder.String()
}

func buildCanonicalQueryString(values url.Values, isQueryAuth bool) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if isQueryAuth && key == "X-Amz-Signature" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var pairs []string
	for _, key := range keys {
		paramValues := values[key]
		sort.Strings(paramValues)
		for _, paramValue := range paramValues {
			escapedKey := url.QueryEscape(key)
			escapedValue := url.QueryEscape(paramValue)
			pairs = append(pairs, escapedKey+"="+escapedValue)
		}
	}

	return strings.Join(pairs, "&")
}

// Validate authenticates the incoming S3 request and returns the caller's ServiceAccount entity.
func (validator *Validator) Validate(request *http.Request) (*core.ServiceAccount, error) {
	log.Trace("validating AWS SigV4 request signature")
	var credentials *authCredentials
	var isQueryAuth bool

	authHeader := request.Header.Get("Authorization")
	if authHeader != "" {
		parsedCredentials, parseErr := parseAuthorizationHeader(authHeader)
		if parseErr != nil {
			return nil, parseErr
		}
		credentials = parsedCredentials
		credentials.RequestTimestamp = request.Header.Get("X-Amz-Date")
		if credentials.RequestTimestamp == "" {
			credentials.RequestTimestamp = request.Header.Get("Date")
		}
	} else if request.URL.Query().Get("X-Amz-Algorithm") != "" {
		parsedCredentials, parseErr := parseQueryParams(request.URL.Query())
		if parseErr != nil {
			return nil, parseErr
		}
		credentials = parsedCredentials
		isQueryAuth = true
	} else {
		return nil, ErrMissingAuthHeader
	}

	// 1. Verify replay window (within 15 minutes)
	requestTime, parseErr := time.Parse("20060102T150405Z", credentials.RequestTimestamp)
	if parseErr != nil {
		// Fallback to RFC1123 format if Date header was provided
		requestTime, parseErr = time.Parse(time.RFC1123, credentials.RequestTimestamp)
	}
	if parseErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidTimestamp, parseErr)
	}

	now := time.Now().UTC()
	diff := now.Sub(requestTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > 15*time.Minute {
		return nil, ErrRequestExpired
	}

	// 2. Query S3 credential from file_storage.s3_credentials
	ctx := request.Context()
	const selectCredentialSQL = `
		SELECT service_account_id, encrypted_secret_key
		FROM file_storage.s3_credentials
		WHERE access_key_id = $1;
	`
	var serviceAccountID uuid.UUID
	var encryptedSecretKey string
	scanErr := validator.kernel.DB().QueryRow(ctx, selectCredentialSQL, credentials.AccessKeyID).Scan(&serviceAccountID, &encryptedSecretKey)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return nil, ErrInvalidAccessKeyID
		}
		return nil, fmt.Errorf("failed to query s3 credential: %w", scanErr)
	}

	// 3. Decrypt secret key
	decryptedBytes, decryptErr := validator.kernel.CryptoKeyManager().DecryptField(encryptedSecretKey)
	if decryptErr != nil {
		return nil, fmt.Errorf("failed to decrypt s3 secret access key: %w", decryptErr)
	}
	secretAccessKey := string(decryptedBytes)

	// 4. Calculate signing key
	kDate := hmacSHA256([]byte("AWS4"+secretAccessKey), []byte(credentials.Date))
	kRegion := hmacSHA256(kDate, []byte(credentials.Region))
	kService := hmacSHA256(kRegion, []byte(credentials.Service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))

	// 5. Build Canonical Request
	canonicalURI := request.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalQuery := buildCanonicalQueryString(request.URL.Query(), isQueryAuth)
	canonicalHeaders := buildCanonicalHeaders(request, credentials.SignedHeaders)

	hashedPayload := request.Header.Get("X-Amz-Content-Sha256")
	if hashedPayload == "" {
		if isQueryAuth {
			hashedPayload = "UNSIGNED-PAYLOAD"
		} else if request.Body != nil {
			bodyBytes, readErr := io.ReadAll(request.Body)
			if readErr != nil {
				return nil, fmt.Errorf("failed to read body for hashing: %w", readErr)
			}
			request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			hashedPayload = sha256Hex(bodyBytes)
		} else {
			hashedPayload = sha256Hex(nil)
		}
	}

	canonicalRequest := strings.Join([]string{
		request.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		credentials.SignedHeaders,
		hashedPayload,
	}, "\n")

	// 6. Build String to Sign
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		credentials.RequestTimestamp,
		credentials.CredentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	// 7. Verify Signature
	calculatedSignature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))
	if subtle.ConstantTimeCompare([]byte(calculatedSignature), []byte(credentials.Signature)) != 1 {
		return nil, ErrSignatureDoesNotMatch
	}

	// 8. Retrieve linked core.service_accounts record and inherit scopes
	const selectServiceAccountSQL = `
		SELECT id, name, description, key_prefix, scopes, is_enabled, allowed_ips, expires_at
		FROM core.service_accounts
		WHERE id = $1;
	`
	var serviceAccount core.ServiceAccount
	var rawScopes []byte
	serviceAccountScanErr := validator.kernel.DB().QueryRow(ctx, selectServiceAccountSQL, serviceAccountID).Scan(
		&serviceAccount.ID,
		&serviceAccount.Name,
		&serviceAccount.Description,
		&serviceAccount.KeyPrefix,
		&rawScopes,
		&serviceAccount.IsEnabled,
		&serviceAccount.AllowedIPs,
		&serviceAccount.ExpiresAt,
	)
	if serviceAccountScanErr != nil {
		if errors.Is(serviceAccountScanErr, pgx.ErrNoRows) {
			return nil, core.ErrServiceAccountNotFound
		}
		return nil, fmt.Errorf("failed querying linked service account: %w", serviceAccountScanErr)
	}

	_ = json.Unmarshal(rawScopes, &serviceAccount.Scopes)

	// 9. Enforce status constraints
	if !serviceAccount.IsEnabled {
		return nil, core.ErrServiceAccountDisabled
	}
	if serviceAccount.ExpiresAt != nil && now.After(*serviceAccount.ExpiresAt) {
		return nil, core.ErrServiceAccountExpired
	}

	clientIP := core.ExtractRequestClientIP(request)
	if len(serviceAccount.AllowedIPs) > 0 && !isIPAllowed(clientIP, serviceAccount.AllowedIPs) {
		return nil, core.ErrServiceAccountIPBlocked
	}

	return &serviceAccount, nil
}

func isIPAllowed(clientIP string, allowedIPs []string) bool {
	if len(allowedIPs) == 0 {
		return true
	}
	parsedClientIP := net.ParseIP(clientIP)
	if parsedClientIP == nil {
		return false
	}

	for _, allowedRange := range allowedIPs {
		if strings.Contains(allowedRange, "/") {
			_, ipNet, err := net.ParseCIDR(allowedRange)
			if err == nil && ipNet.Contains(parsedClientIP) {
				return true
			}
		} else {
			if allowedRange == clientIP {
				return true
			}
		}
	}
	return false
}
