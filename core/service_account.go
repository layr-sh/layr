package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
	"uuid"
)

// Standard Service Account Errors
var (
	ErrServiceAccountNotFound           = errors.New("service account not found")
	ErrServiceAccountDisabled           = errors.New("service account is disabled")
	ErrServiceAccountExpired            = errors.New("service account has expired")
	ErrServiceAccountIPBlocked          = errors.New("request IP not allowed for this service account")
	ErrInsufficientPermissions          = errors.New("insufficient service account scope permissions")
	ErrRootAccountProtected             = errors.New("cannot delete or revoke the last root account with wildcard * scope")
	ErrServiceAccountOwnedByConsoleUser = errors.New("cannot delete service account owned by a console user; delete the console user instead")
)

type contextKey string

const serviceAccountContextKey contextKey = "core_service_account"

const (
	secretKeyEntropyBytes  = 32
	secretKeyPrefixLength  = 8
	minimumSecretKeyLength = 16
)

// WithServiceAccount stores the authenticated service account in context.
func WithServiceAccount(ctx context.Context, serviceAccount *ServiceAccount) context.Context {
	return context.WithValue(ctx, serviceAccountContextKey, serviceAccount)
}

// GetServiceAccount retrieves the authenticated service account from context if present.
func GetServiceAccount(ctx context.Context) *ServiceAccount {
	if serviceAccount, ok := ctx.Value(serviceAccountContextKey).(*ServiceAccount); ok {
		return serviceAccount
	}
	return nil
}

// ServiceAccount represents a machine identity stored in core.service_accounts.
type ServiceAccount struct {
	ID            string     `json:"id"`
	ConsoleUserID *string    `json:"console_user_id,omitempty"`
	Name          string     `json:"name"`
	Description   *string    `json:"description,omitempty"`
	KeyPrefix     string     `json:"key_prefix"`
	KeyHash       string     `json:"-"`
	Scopes        []string   `json:"scopes"`
	IsEnabled     bool       `json:"is_enabled"`
	AllowedIPs    []string   `json:"allowed_ips,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastUpdatedAt time.Time  `json:"last_updated_at"`
}

// CreateServiceAccountInput holds the input parameters for creating a new Service Account.
type CreateServiceAccountInput struct {
	ConsoleUserID *string    `json:"console_user_id,omitempty"`
	Name          string     `json:"name"`
	Description   *string    `json:"description,omitempty"`
	Scopes        []string   `json:"scopes"`
	AllowedIPs    []string   `json:"allowed_ips,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// UpdateServiceAccountInput holds the update payload for a Service Account.
type UpdateServiceAccountInput struct {
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	Scopes      []string   `json:"scopes,omitempty"`
	IsEnabled   *bool      `json:"is_enabled,omitempty"`
	AllowedIPs  []string   `json:"allowed_ips,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// ServiceAccountWithSecretKey returned when creating a new Service Account, containing the plaintext secret.
type ServiceAccountWithSecretKey struct {
	ServiceAccount
	SecretKey string `json:"secret_key"`
}

// ServiceAccountManager handles Service Account database lifecycle and authentication.
type ServiceAccountManager struct {
	db *DatabasePool
}

// NewServiceAccountManager creates a new manager instance.
func NewServiceAccountManager(db *DatabasePool) *ServiceAccountManager {
	return &ServiceAccountManager{db: db}
}

// GenerateServiceAccountSecretKey creates a high-entropy secret key and returns (prefix, secretKey, hash).
func GenerateServiceAccountSecretKey() (string, string, string) {
	randomBytes := make([]byte, secretKeyEntropyBytes)
	_, _ = rand.Read(randomBytes)
	secretKey := hex.EncodeToString(randomBytes)
	prefix := secretKey[:secretKeyPrefixLength]
	digest := sha256.Sum256([]byte(secretKey))
	hash := hex.EncodeToString(digest[:])
	return prefix, secretKey, hash
}

// HashSecretKey returns the SHA-256 hex digest of a key string.
func HashSecretKey(secretKey string) string {
	digest := sha256.Sum256([]byte(secretKey))
	return hex.EncodeToString(digest[:])
}

// Create creates a new Service Account and returns the plaintext key once.
func (serviceAccountManager *ServiceAccountManager) Create(ctx context.Context, createServiceAccountInput CreateServiceAccountInput) (*ServiceAccountWithSecretKey, error) {
	log.Debugf("creating service account %q", createServiceAccountInput.Name)
	if serviceAccountManager.db == nil {
		return nil, fmt.Errorf("db connection pool not available")
	}
	if strings.TrimSpace(createServiceAccountInput.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}

	scopes := createServiceAccountInput.Scopes
	if len(scopes) == 0 {
		scopes = []string{ScopeRoot}
	}

	prefix, secretKey, hash := GenerateServiceAccountSecretKey()
	scopesJSON, _ := json.Marshal(scopes)

	allowedIPs := []string{}
	if len(createServiceAccountInput.AllowedIPs) > 0 {
		allowedIPs = createServiceAccountInput.AllowedIPs
	}

	serviceAccountID := uuid.NewV7().String()
	now := time.Now().UTC()

	query := `
		INSERT INTO core.service_accounts (
			id, name, description, key_prefix, key_hash, scopes, is_enabled, allowed_ips, expires_at, console_user_id, created_at, last_updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, true, $7, $8, $9, $10, $10)
		RETURNING id, name, description, key_prefix, scopes, is_enabled, allowed_ips, expires_at, console_user_id, created_at, last_updated_at
	`

	var serviceAccount ServiceAccount
	var scopesRaw []byte

	err := serviceAccountManager.db.QueryRow(ctx, query,
		serviceAccountID, createServiceAccountInput.Name, createServiceAccountInput.Description, prefix, hash, scopesJSON, allowedIPs, createServiceAccountInput.ExpiresAt, createServiceAccountInput.ConsoleUserID, now,
	).Scan(
		&serviceAccount.ID, &serviceAccount.Name, &serviceAccount.Description, &serviceAccount.KeyPrefix, &scopesRaw, &serviceAccount.IsEnabled, &serviceAccount.AllowedIPs, &serviceAccount.ExpiresAt, &serviceAccount.ConsoleUserID, &serviceAccount.CreatedAt, &serviceAccount.LastUpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create service account: %w", err)
	}

	_ = json.Unmarshal(scopesRaw, &serviceAccount.Scopes)

	log.Tracef("created service account %s with prefix %s", serviceAccountID, prefix)
	return &ServiceAccountWithSecretKey{
		ServiceAccount: serviceAccount,
		SecretKey:      secretKey,
	}, nil
}

// Authenticate validates a presented secret key against core.service_accounts.
func (serviceAccountManager *ServiceAccountManager) Authenticate(ctx context.Context, secretKey, clientIP string) (*ServiceAccount, error) {
	if serviceAccountManager.db == nil {
		return nil, fmt.Errorf("db connection pool not available")
	}
	secretKey = strings.TrimSpace(secretKey)
	if len(secretKey) < minimumSecretKeyLength {
		log.Tracef("service account key length %d is below minimum %d", len(secretKey), minimumSecretKeyLength)
		return nil, ErrServiceAccountNotFound
	}

	prefix := secretKey[:secretKeyPrefixLength]
	hash := HashSecretKey(secretKey)
	log.Tracef("authenticating service account key (prefix: %s, clientIP: %s)", prefix, clientIP)

	query := `
		SELECT id, name, description, key_prefix, key_hash, scopes, is_enabled, allowed_ips, expires_at, console_user_id, last_used_at, created_at, last_updated_at
		FROM core.service_accounts
		WHERE key_prefix = $1
	`

	rows, err := serviceAccountManager.db.Query(ctx, query, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to query service accounts: %w", err)
	}
	defer rows.Close()

	var matchedServiceAccount *ServiceAccount
	for rows.Next() {
		var serviceAccount ServiceAccount
		var scopesRaw []byte
		var keyHash string

		_ = rows.Scan(
			&serviceAccount.ID, &serviceAccount.Name, &serviceAccount.Description, &serviceAccount.KeyPrefix, &keyHash, &scopesRaw, &serviceAccount.IsEnabled, &serviceAccount.AllowedIPs, &serviceAccount.ExpiresAt, &serviceAccount.ConsoleUserID, &serviceAccount.LastUsedAt, &serviceAccount.CreatedAt, &serviceAccount.LastUpdatedAt,
		)

		if keyHash == hash {
			_ = json.Unmarshal(scopesRaw, &serviceAccount.Scopes)
			matchedServiceAccount = &serviceAccount
			break
		}
	}

	if matchedServiceAccount == nil {
		log.Debugf("service account authentication failed: no matching active key for prefix %s", prefix)
		return nil, ErrServiceAccountNotFound
	}

	if !matchedServiceAccount.IsEnabled {
		log.Warnf("service account authentication rejected: account %s (%q) is disabled", matchedServiceAccount.ID, matchedServiceAccount.Name)
		return nil, ErrServiceAccountDisabled
	}

	if matchedServiceAccount.ExpiresAt != nil && time.Now().UTC().After(*matchedServiceAccount.ExpiresAt) {
		log.Warnf("service account authentication rejected: account %s (%q) expired at %s", matchedServiceAccount.ID, matchedServiceAccount.Name, matchedServiceAccount.ExpiresAt.Format(time.RFC3339))
		return nil, ErrServiceAccountExpired
	}

	if len(matchedServiceAccount.AllowedIPs) > 0 && clientIP != "" {
		if !isIPAllowed(clientIP, matchedServiceAccount.AllowedIPs) {
			log.Warnf("service account authentication rejected: client IP %s not in allowed list %v for account %s", clientIP, matchedServiceAccount.AllowedIPs, matchedServiceAccount.ID)
			return nil, ErrServiceAccountIPBlocked
		}
	}

	log.Debugf("authenticated service account %s (%q)", matchedServiceAccount.ID, matchedServiceAccount.Name)
	// Update last_used_at asynchronously (detached from request cancellation)
	go func(serviceAccountID string) {
		now := time.Now().UTC()
		_, _ = serviceAccountManager.db.Exec(context.WithoutCancel(ctx), `
			UPDATE core.service_accounts SET last_used_at = $1 WHERE id = $2
		`, now, serviceAccountID)
	}(matchedServiceAccount.ID)

	return matchedServiceAccount, nil
}

// List returns all service accounts.
func (serviceAccountManager *ServiceAccountManager) List(ctx context.Context) ([]ServiceAccount, error) {
	log.Trace("listing service accounts from database")
	if serviceAccountManager.db == nil {
		return nil, fmt.Errorf("db connection pool not available")
	}
	query := `
		SELECT id, name, description, key_prefix, scopes, is_enabled, allowed_ips, expires_at, console_user_id, last_used_at, created_at, last_updated_at
		FROM core.service_accounts
		ORDER BY created_at DESC
	`
	rows, err := serviceAccountManager.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list service accounts: %w", err)
	}
	defer rows.Close()

	results := make([]ServiceAccount, 0)
	for rows.Next() {
		var serviceAccount ServiceAccount
		var scopesRaw []byte
		_ = rows.Scan(
			&serviceAccount.ID, &serviceAccount.Name, &serviceAccount.Description, &serviceAccount.KeyPrefix, &scopesRaw, &serviceAccount.IsEnabled, &serviceAccount.AllowedIPs, &serviceAccount.ExpiresAt, &serviceAccount.ConsoleUserID, &serviceAccount.LastUsedAt, &serviceAccount.CreatedAt, &serviceAccount.LastUpdatedAt,
		)
		_ = json.Unmarshal(scopesRaw, &serviceAccount.Scopes)
		results = append(results, serviceAccount)
	}
	log.Tracef("listed %d service accounts", len(results))
	return results, nil
}

// Get returns a service account by UUID.
func (serviceAccountManager *ServiceAccountManager) Get(ctx context.Context, serviceAccountID string) (*ServiceAccount, error) {
	log.Tracef("retrieving service account %s", serviceAccountID)
	if serviceAccountManager.db == nil {
		return nil, fmt.Errorf("db connection pool not available")
	}
	query := `
		SELECT id, name, description, key_prefix, scopes, is_enabled, allowed_ips, expires_at, console_user_id, last_used_at, created_at, last_updated_at
		FROM core.service_accounts
		WHERE id = $1
	`
	var serviceAccount ServiceAccount
	var scopesRaw []byte

	err := serviceAccountManager.db.QueryRow(ctx, query, serviceAccountID).Scan(
		&serviceAccount.ID, &serviceAccount.Name, &serviceAccount.Description, &serviceAccount.KeyPrefix, &scopesRaw, &serviceAccount.IsEnabled, &serviceAccount.AllowedIPs, &serviceAccount.ExpiresAt, &serviceAccount.ConsoleUserID, &serviceAccount.LastUsedAt, &serviceAccount.CreatedAt, &serviceAccount.LastUpdatedAt,
	)
	if err != nil {
		return nil, ErrServiceAccountNotFound
	}
	_ = json.Unmarshal(scopesRaw, &serviceAccount.Scopes)
	return &serviceAccount, nil
}

// Update updates service account metadata, scopes, or state.
func (serviceAccountManager *ServiceAccountManager) Update(ctx context.Context, serviceAccountID string, updateServiceAccountInput UpdateServiceAccountInput) (*ServiceAccount, error) {
	log.Debugf("updating service account %s", serviceAccountID)
	currentServiceAccount, err := serviceAccountManager.Get(ctx, serviceAccountID)
	if err != nil {
		return nil, err
	}

	// Check root protection if modifying scopes or disabling
	if HasScope(currentServiceAccount.Scopes, ScopeRoot) {
		if (updateServiceAccountInput.IsEnabled != nil && !*updateServiceAccountInput.IsEnabled) || (len(updateServiceAccountInput.Scopes) > 0 && !HasScope(updateServiceAccountInput.Scopes, ScopeRoot)) {
			if err := serviceAccountManager.assertOtherRootExists(ctx, serviceAccountID); err != nil {
				return nil, err
			}
		}
	}

	name := currentServiceAccount.Name
	if updateServiceAccountInput.Name != nil && strings.TrimSpace(*updateServiceAccountInput.Name) != "" {
		name = *updateServiceAccountInput.Name
	}
	description := currentServiceAccount.Description
	if updateServiceAccountInput.Description != nil {
		description = updateServiceAccountInput.Description
	}
	isEnabled := currentServiceAccount.IsEnabled
	if updateServiceAccountInput.IsEnabled != nil {
		isEnabled = *updateServiceAccountInput.IsEnabled
	}
	scopes := currentServiceAccount.Scopes
	if len(updateServiceAccountInput.Scopes) > 0 {
		scopes = updateServiceAccountInput.Scopes
	}
	allowedIPs := currentServiceAccount.AllowedIPs
	if updateServiceAccountInput.AllowedIPs != nil {
		allowedIPs = updateServiceAccountInput.AllowedIPs
	}
	expiresAt := currentServiceAccount.ExpiresAt
	if updateServiceAccountInput.ExpiresAt != nil {
		expiresAt = updateServiceAccountInput.ExpiresAt
	}

	scopesJSON, _ := json.Marshal(scopes)
	now := time.Now().UTC()

	query := `
		UPDATE core.service_accounts
		SET name = $1, description = $2, scopes = $3, is_enabled = $4, allowed_ips = $5, expires_at = $6, last_updated_at = $7
		WHERE id = $8
		RETURNING id, name, description, key_prefix, scopes, is_enabled, allowed_ips, expires_at, console_user_id, last_used_at, created_at, last_updated_at
	`

	var serviceAccount ServiceAccount
	var scopesRaw []byte

	_ = serviceAccountManager.db.QueryRow(ctx, query,
		name, description, scopesJSON, isEnabled, allowedIPs, expiresAt, now, serviceAccountID,
	).Scan(
		&serviceAccount.ID, &serviceAccount.Name, &serviceAccount.Description, &serviceAccount.KeyPrefix, &scopesRaw, &serviceAccount.IsEnabled, &serviceAccount.AllowedIPs, &serviceAccount.ExpiresAt, &serviceAccount.ConsoleUserID, &serviceAccount.LastUsedAt, &serviceAccount.CreatedAt, &serviceAccount.LastUpdatedAt,
	)

	_ = json.Unmarshal(scopesRaw, &serviceAccount.Scopes)
	log.Tracef("updated service account %s successfully", serviceAccountID)
	return &serviceAccount, nil
}

// Delete removes a service account with console ownership and root protection assertion.
func (serviceAccountManager *ServiceAccountManager) Delete(ctx context.Context, serviceAccountID string) error {
	log.Debugf("deleting service account %s", serviceAccountID)
	currentServiceAccount, err := serviceAccountManager.Get(ctx, serviceAccountID)
	if err != nil {
		return err
	}

	if currentServiceAccount.ConsoleUserID != nil && *currentServiceAccount.ConsoleUserID != "" {
		return ErrServiceAccountOwnedByConsoleUser
	}

	if HasScope(currentServiceAccount.Scopes, ScopeRoot) {
		if rootCheckErr := serviceAccountManager.assertOtherRootExists(ctx, serviceAccountID); rootCheckErr != nil {
			return rootCheckErr
		}
	}

	_, err = serviceAccountManager.db.Exec(ctx, "DELETE FROM core.service_accounts WHERE id = $1", serviceAccountID)
	if err == nil {
		log.Tracef("deleted service account %s successfully", serviceAccountID)
	}
	return err
}

func (serviceAccountManager *ServiceAccountManager) assertOtherRootExists(ctx context.Context, excludeID string) error {
	log.Tracef("asserting existence of another active root service account (excluding %s)", excludeID)
	query := `
		SELECT COUNT(*) FROM core.service_accounts
		WHERE id != $1 AND is_enabled = true AND scopes @> '["*"]'::jsonb
	`
	var count int
	_ = serviceAccountManager.db.QueryRow(ctx, query, excludeID).Scan(&count)
	if count == 0 {
		return ErrRootAccountProtected
	}
	return nil
}

func isIPAllowed(clientIP string, allowedPatterns []string) bool {
	parsedClientIP := net.ParseIP(strings.TrimSpace(clientIP))
	if parsedClientIP == nil {
		return false
	}

	for _, pattern := range allowedPatterns {
		pattern = strings.TrimSpace(pattern)
		if strings.Contains(pattern, "/") {
			_, ipNet, err := net.ParseCIDR(pattern)
			if err == nil && ipNet.Contains(parsedClientIP) {
				return true
			}
		} else {
			patternIP := net.ParseIP(pattern)
			if patternIP != nil && patternIP.Equal(parsedClientIP) {
				return true
			}
		}
	}
	return false
}

// ExtractRequestServiceAccountKey extracts service account key from X-Layr-Service-Account-Key or Bearer Authorization header.
func ExtractRequestServiceAccountKey(request *http.Request) string {
	if key := request.Header.Get("X-Layr-Service-Account-Key"); key != "" {
		return strings.TrimSpace(key)
	}
	authHeader := request.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return ""
}

// ServiceAccountAuthMiddleware authenticates service account keys and injects the account into request context.
func ServiceAccountAuthMiddleware(serviceAccountManager *ServiceAccountManager) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if serviceAccountManager == nil {
				handler.ServeHTTP(responseWriter, request)
				return
			}
			secretKey := ExtractRequestServiceAccountKey(request)
			if secretKey == "" {
				handler.ServeHTTP(responseWriter, request)
				return
			}
			log.Tracef("evaluating service account authentication for request %s", request.URL.Path)
			clientIP := ExtractRequestClientIP(request)
			serviceAccount, err := serviceAccountManager.Authenticate(request.Context(), secretKey, clientIP)
			if err != nil {
				// Invalid service account key
				WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, err.Error(), "LAYR_CORE_006")
				return
			}
			ctx := WithServiceAccount(request.Context(), serviceAccount)
			handler.ServeHTTP(responseWriter, request.WithContext(ctx))
		})
	}
}

// RequireScopeMiddleware enforces that the request has a service account with the required scope.
func RequireScopeMiddleware(requiredScope string) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			serviceAccount := GetServiceAccount(request.Context())
			if serviceAccount == nil {
				WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "service account authentication required", "LAYR_CORE_006")
				return
			}
			if !HasScope(serviceAccount.Scopes, requiredScope) {
				WriteErrorResponse(responseWriter, request, http.StatusForbidden, ErrInsufficientPermissions.Error(), "LAYR_CORE_007")
				return
			}
			handler.ServeHTTP(responseWriter, request)
		})
	}
}
