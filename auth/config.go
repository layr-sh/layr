package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// Named constants for default durations and limits to satisfy linter checks.
const (
	defaultAccessTokenExpirySeconds  = 900     // 15 minutes
	defaultRefreshTokenExpirySeconds = 2592000 // 30 days
	defaultIdleTimeoutSeconds        = 604800  // 7 days
	defaultSessionTTLSeconds         = 900     // 15 minutes
	defaultLockoutDurationSeconds    = 900     // 15 minutes
	defaultWindowDurationSeconds     = 900     // 15 minutes
	defaultMaxSignInAttempts         = 5
	defaultPasswordMinLength         = 8
	defaultTokenExpiryMinutes        = 15
)

// ConfigKey is the primary key in auth.config table.
const ConfigKey = "auth_config"

// Config represents the full dynamic configuration for layr/auth.
type Config struct {
	Password        PasswordConfig                 `json:"password"`
	Passkeys        PasskeysConfig                 `json:"passkeys"`
	EmailOTP        EmailOTPConfig                 `json:"email_otp"`
	SMSOTP          SMSOTPConfig                   `json:"sms_otp"`
	MFA             MFAConfig                      `json:"mfa"`
	Anonymous       AnonymousConfig                `json:"anonymous"`
	Sessions        SessionsConfig                 `json:"sessions"`
	OAuthProviders  map[string]OAuthProviderConfig `json:"oauth_providers"`
	OIDC            OIDCConfig                     `json:"oidc"`
	EmailDispatcher EmailDispatcherConfig          `json:"email_dispatcher"`
	SMSDispatcher   SMSDispatcherConfig            `json:"sms_dispatcher"`
	RateLimiting    RateLimitingConfig             `json:"rate_limiting"`
	Cache           CacheConfig                    `json:"cache"`
}

// RateLimitingConfig controls dynamic sign-in brute-force protection.
type RateLimitingConfig struct {
	Enabled                bool `json:"enabled"`
	MaxSignInAttempts      int  `json:"max_sign_in_attempts"`
	WindowDurationSeconds  int  `json:"window_duration_seconds"`
	LockoutDurationSeconds int  `json:"lockout_duration_seconds"`
}

// CacheConfig controls fast-path session caching in KvStore.
type CacheConfig struct {
	FastPathSessionsEnabled bool `json:"fast_path_sessions_enabled"`
	SessionTTLSeconds       int  `json:"session_ttl_seconds"`
}

// PasswordConfig defines password authentication policy.
type PasswordConfig struct {
	Enabled        bool `json:"enabled"`
	MinLength      int  `json:"min_length"`
	RequireNumbers bool `json:"require_numbers"`
	RequireSymbols bool `json:"require_symbols"`
}

// PasskeysConfig defines WebAuthn passkey authentication config.
type PasskeysConfig struct {
	Enabled          bool   `json:"enabled"`
	RelyingPartyID   string `json:"relying_party_id"`
	RelyingPartyName string `json:"relying_party_name"`
}

// EmailOTPConfig defines email one-time-password authentication config.
type EmailOTPConfig struct {
	Enabled            bool `json:"enabled"`
	TokenExpiryMinutes int  `json:"token_expiry_minutes"`
}

// SMSOTPConfig defines SMS one-time-password authentication config.
type SMSOTPConfig struct {
	Enabled            bool `json:"enabled"`
	TokenExpiryMinutes int  `json:"token_expiry_minutes"`
}

// MFAConfig defines multi-factor authentication config.
type MFAConfig struct {
	Enabled bool   `json:"enabled"`
	Issuer  string `json:"issuer"`
}

// AnonymousConfig defines guest or anonymous session config.
type AnonymousConfig struct {
	Enabled bool `json:"enabled"`
}

// SessionsConfig controls JWT and refresh token expiry durations.
type SessionsConfig struct {
	AccessTokenExpirySeconds  int  `json:"access_token_expiry_seconds"`
	RefreshTokenExpirySeconds int  `json:"refresh_token_expiry_seconds"`
	IdleTimeoutSeconds        int  `json:"idle_timeout_seconds"`
	SingleSessionPerUser      bool `json:"single_session_per_user"`
}

// OAuthProviderConfig stores credentials and endpoints for federated OAuth providers.
type OAuthProviderConfig struct {
	Enabled                bool   `json:"enabled"`
	Preset                 string `json:"preset,omitempty"`
	AuthURL                string `json:"auth_url,omitempty"`
	TokenURL               string `json:"token_url,omitempty"`
	UserInfoURL            string `json:"userinfo_url,omitempty"`
	ClientID               string `json:"client_id"`
	ClientSecret           string `json:"client_secret,omitempty"`
	ClientSecretConfigured bool   `json:"client_secret_configured,omitempty"`
	Scope                  string `json:"scope,omitempty"`
	ResponseMode           string `json:"response_mode,omitempty"`
	ResponseType           string `json:"response_type,omitempty"`
	IDAttribute            string `json:"id_attribute,omitempty"`
	EmailAttribute         string `json:"email_attribute,omitempty"`
	NameAttribute          string `json:"name_attribute,omitempty"`
	AvatarAttribute        string `json:"avatar_attribute,omitempty"`
}

// OIDCConfig defines configuration for Layr Auth as an OpenID Connect (OIDC) Identity Provider.
type OIDCConfig struct {
	Enabled bool               `json:"enabled"`
	Clients []OIDCClientConfig `json:"clients,omitempty"`
	UI      OIDCUIConfig       `json:"ui"`
}

// OIDCClientConfig defines a registered third-party OpenID Connect client application.
type OIDCClientConfig struct {
	Name                    string   `json:"name"`
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientSecretConfigured  bool     `json:"client_secret_configured,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	PostSignOutRedirectURIs []string `json:"post_sign_out_redirect_uris,omitempty"`
	Public                  bool     `json:"public"` // True for PKCE-only public clients (SPA/mobile); false for confidential clients
	Scopes                  []string `json:"scopes,omitempty"`
}

// OIDCUIConfig defines dynamic styling, branding, legal links, and authentication method enablement for the Universal Sign-In page.
type OIDCUIConfig struct {
	CustomCSS         string `json:"custom_css,omitempty"`
	LogoURL           string `json:"logo_url,omitempty"`
	PrivacyPolicyURL  string `json:"privacy_policy_url,omitempty"`
	TermsOfServiceURL string `json:"terms_of_service_url,omitempty"`
	ShowSignUp        bool   `json:"show_sign_up,omitempty"`
	ShowPassword      bool   `json:"show_password,omitempty"`
	ShowPasskeys      bool   `json:"show_passkeys,omitempty"`
	ShowOAuth         bool   `json:"show_oauth,omitempty"`
	ShowEmailOTP      bool   `json:"show_email_otp,omitempty"`
	ShowSMSOTP        bool   `json:"show_sms_otp,omitempty"`
}

// DefaultConfig returns canonical defaults for layr/auth.
func DefaultConfig() Config {
	defaultEmailDispatcherConfig := EmailDispatcherDefaultConfig()
	defaultSMSDispatcherConfig := SMSDispatcherDefaultConfig()

	projectName := core.GetConfig().Project.Name
	relyingPartyName := projectName
	if relyingPartyName == "" {
		relyingPartyName = "Layr Auth"
	}
	mfaIssuer := projectName
	if mfaIssuer == "" {
		mfaIssuer = "Layr Auth"
	}

	return Config{
		Password: PasswordConfig{
			Enabled:        true,
			MinLength:      defaultPasswordMinLength,
			RequireNumbers: true,
			RequireSymbols: false,
		},
		Passkeys: PasskeysConfig{
			Enabled:          true,
			RelyingPartyID:   "localhost",
			RelyingPartyName: relyingPartyName,
		},
		EmailOTP: EmailOTPConfig{
			Enabled:            false,
			TokenExpiryMinutes: defaultTokenExpiryMinutes,
		},
		SMSOTP: SMSOTPConfig{
			Enabled:            false,
			TokenExpiryMinutes: defaultTokenExpiryMinutes,
		},
		MFA: MFAConfig{
			Enabled: true,
			Issuer:  mfaIssuer,
		},
		Anonymous: AnonymousConfig{
			Enabled: true,
		},
		Sessions: SessionsConfig{
			AccessTokenExpirySeconds:  defaultAccessTokenExpirySeconds,
			RefreshTokenExpirySeconds: defaultRefreshTokenExpirySeconds,
			IdleTimeoutSeconds:        defaultIdleTimeoutSeconds,
			SingleSessionPerUser:      false,
		},
		OAuthProviders: map[string]OAuthProviderConfig{
			"google":  {Enabled: false, Preset: "google"},
			"github":  {Enabled: false, Preset: "github"},
			"apple":   {Enabled: false, Preset: "apple"},
			"discord": {Enabled: false, Preset: "discord"},
		},
		OIDC: OIDCConfig{
			Enabled: false,
			Clients: make([]OIDCClientConfig, 0),
			UI: OIDCUIConfig{
				CustomCSS:         "",
				LogoURL:           "",
				PrivacyPolicyURL:  "",
				TermsOfServiceURL: "",
				ShowSignUp:        false,
				ShowPassword:      true,
				ShowPasskeys:      true,
				ShowOAuth:         false,
				ShowEmailOTP:      false,
				ShowSMSOTP:        false,
			},
		},
		EmailDispatcher: defaultEmailDispatcherConfig,
		SMSDispatcher:   defaultSMSDispatcherConfig,
		RateLimiting: RateLimitingConfig{
			Enabled:                true,
			MaxSignInAttempts:      defaultMaxSignInAttempts,
			WindowDurationSeconds:  defaultWindowDurationSeconds,
			LockoutDurationSeconds: defaultLockoutDurationSeconds,
		},
		Cache: CacheConfig{
			FastPathSessionsEnabled: true,
			SessionTTLSeconds:       defaultSessionTTLSeconds,
		},
	}
}

// ConfigManager handles loading, validating, caching, and envelope encryption of auth.config.
type ConfigManager struct {
	db                    *core.DatabasePool
	cryptoKeyManager      *core.CryptoKeyManager
	serviceAccountManager *core.ServiceAccountManager
	eventBus              *core.EventBus
	rwMutex               sync.RWMutex
	config                Config
}

// NewConfigManager initializes a new ConfigManager.
func NewConfigManager(db *core.DatabasePool, cryptoKeyManager *core.CryptoKeyManager) *ConfigManager {
	return &ConfigManager{
		db:               db,
		cryptoKeyManager: cryptoKeyManager,
		config:           DefaultConfig(),
	}
}

// SetServiceAccountManager configures the service account manager for scope authorization.
func (configManager *ConfigManager) SetServiceAccountManager(serviceAccountManager *core.ServiceAccountManager) {
	configManager.serviceAccountManager = serviceAccountManager
}

// SetEventBus configures the platform event bus.
func (configManager *ConfigManager) SetEventBus(eventBus *core.EventBus) {
	configManager.eventBus = eventBus
}

func (configManager *ConfigManager) checkScope(request *http.Request, requiredScope string) bool {
	if configManager.serviceAccountManager == nil {
		return true
	}
	secretKey := core.ExtractRequestServiceAccountKey(request)
	if secretKey == "" {
		return true
	}
	clientIP := core.ExtractRequestClientIP(request)
	serviceAccount, err := configManager.serviceAccountManager.Authenticate(request.Context(), secretKey, clientIP)
	if err != nil {
		return false
	}
	return core.HasScope(serviceAccount.Scopes, requiredScope)
}

// Get returns a copy of the active in-memory configuration.
func (configManager *ConfigManager) Get() Config {
	configManager.rwMutex.RLock()
	defer configManager.rwMutex.RUnlock()
	copiedConfig := configManager.config
	if configManager.config.OAuthProviders != nil {
		copiedConfig.OAuthProviders = make(map[string]OAuthProviderConfig, len(configManager.config.OAuthProviders))
		for providerKey, providerConfig := range configManager.config.OAuthProviders {
			copiedConfig.OAuthProviders[providerKey] = providerConfig
		}
	}
	if configManager.config.OIDC.Clients != nil {
		copiedConfig.OIDC.Clients = make([]OIDCClientConfig, len(configManager.config.OIDC.Clients))
		copy(copiedConfig.OIDC.Clients, configManager.config.OIDC.Clients)
	}
	return copiedConfig
}

// Set updates the in-memory configuration with sane fallbacks.
func (configManager *ConfigManager) Set(updatedConfig Config) {
	configManager.rwMutex.Lock()
	defer configManager.rwMutex.Unlock()

	if updatedConfig.Sessions.AccessTokenExpirySeconds <= 0 {
		updatedConfig.Sessions.AccessTokenExpirySeconds = defaultAccessTokenExpirySeconds
	}
	if updatedConfig.Sessions.RefreshTokenExpirySeconds <= 0 {
		updatedConfig.Sessions.RefreshTokenExpirySeconds = defaultRefreshTokenExpirySeconds
	}
	if updatedConfig.Password.MinLength <= 0 {
		updatedConfig.Password.MinLength = defaultPasswordMinLength
	}
	if updatedConfig.EmailOTP.TokenExpiryMinutes <= 0 {
		updatedConfig.EmailOTP.TokenExpiryMinutes = defaultTokenExpiryMinutes
	}
	if updatedConfig.SMSOTP.TokenExpiryMinutes <= 0 {
		updatedConfig.SMSOTP.TokenExpiryMinutes = defaultTokenExpiryMinutes
	}
	if updatedConfig.Passkeys.RelyingPartyName == "" {
		relyingPartyName := core.GetConfig().Project.Name
		if relyingPartyName == "" {
			relyingPartyName = "Layr Auth"
		}
		updatedConfig.Passkeys.RelyingPartyName = relyingPartyName
	}
	if updatedConfig.MFA.Issuer == "" {
		mfaIssuer := core.GetConfig().Project.Name
		if mfaIssuer == "" {
			mfaIssuer = "Layr Auth"
		}
		updatedConfig.MFA.Issuer = mfaIssuer
	}
	if updatedConfig.OAuthProviders == nil {
		updatedConfig.OAuthProviders = make(map[string]OAuthProviderConfig)
	}
	if updatedConfig.OIDC.Clients == nil {
		updatedConfig.OIDC.Clients = make([]OIDCClientConfig, 0)
	}
	if !updatedConfig.RateLimiting.Enabled && updatedConfig.RateLimiting.MaxSignInAttempts == 0 {
		updatedConfig.RateLimiting.Enabled = true
	}
	if updatedConfig.RateLimiting.MaxSignInAttempts <= 0 {
		updatedConfig.RateLimiting.MaxSignInAttempts = defaultMaxSignInAttempts
	}
	if updatedConfig.RateLimiting.WindowDurationSeconds <= 0 {
		updatedConfig.RateLimiting.WindowDurationSeconds = defaultWindowDurationSeconds
	}
	if updatedConfig.RateLimiting.LockoutDurationSeconds <= 0 {
		updatedConfig.RateLimiting.LockoutDurationSeconds = defaultLockoutDurationSeconds
	}
	if !updatedConfig.Cache.FastPathSessionsEnabled && updatedConfig.Cache.SessionTTLSeconds == 0 {
		updatedConfig.Cache.FastPathSessionsEnabled = true
	}
	if updatedConfig.Cache.SessionTTLSeconds <= 0 {
		updatedConfig.Cache.SessionTTLSeconds = defaultSessionTTLSeconds
	}
	if updatedConfig.EmailDispatcher.Templates.EmailVerification.Subject == "" && updatedConfig.EmailDispatcher.Templates.PasswordReset.Subject == "" && updatedConfig.EmailDispatcher.Templates.SignInOTP.Subject == "" {
		defaultEmailDispatcherConfig := EmailDispatcherDefaultConfig()
		updatedConfig.EmailDispatcher.Templates = defaultEmailDispatcherConfig.Templates
	}
	if updatedConfig.SMSDispatcher.Templates.PhoneVerification.Text == "" && updatedConfig.SMSDispatcher.Templates.SignInOTP.Text == "" && updatedConfig.SMSDispatcher.Templates.PasswordReset.Text == "" {
		defaultSMSDispatcherConfig := SMSDispatcherDefaultConfig()
		updatedConfig.SMSDispatcher.Templates = defaultSMSDispatcherConfig.Templates
	}
	if updatedConfig.OAuthProviders != nil {
		copiedProviders := make(map[string]OAuthProviderConfig, len(updatedConfig.OAuthProviders))
		for providerKey, providerConfig := range updatedConfig.OAuthProviders {
			copiedProviders[providerKey] = providerConfig
		}
		updatedConfig.OAuthProviders = copiedProviders
	}
	if updatedConfig.OIDC.Clients != nil {
		copiedClients := make([]OIDCClientConfig, len(updatedConfig.OIDC.Clients))
		copy(copiedClients, updatedConfig.OIDC.Clients)
		updatedConfig.OIDC.Clients = copiedClients
	}

	hasOAuthCurrent := false
	for _, provider := range configManager.config.OAuthProviders {
		if provider.Enabled {
			hasOAuthCurrent = true
			break
		}
	}
	hasOAuthNew := false
	for _, provider := range updatedConfig.OAuthProviders {
		if provider.Enabled {
			hasOAuthNew = true
			break
		}
	}

	if !configManager.config.Password.Enabled && updatedConfig.Password.Enabled {
		updatedConfig.OIDC.UI.ShowPassword = true
	}
	if !configManager.config.Passkeys.Enabled && updatedConfig.Passkeys.Enabled {
		updatedConfig.OIDC.UI.ShowPasskeys = true
	}
	if !configManager.config.EmailOTP.Enabled && updatedConfig.EmailOTP.Enabled {
		updatedConfig.OIDC.UI.ShowEmailOTP = true
	}
	if !configManager.config.SMSOTP.Enabled && updatedConfig.SMSOTP.Enabled {
		updatedConfig.OIDC.UI.ShowSMSOTP = true
	}
	if !hasOAuthCurrent && hasOAuthNew {
		updatedConfig.OIDC.UI.ShowOAuth = true
	}

	if !updatedConfig.Password.Enabled {
		updatedConfig.OIDC.UI.ShowPassword = false
		updatedConfig.OIDC.UI.ShowSignUp = false
	}
	if !updatedConfig.Passkeys.Enabled {
		updatedConfig.OIDC.UI.ShowPasskeys = false
	}
	if !updatedConfig.EmailOTP.Enabled {
		updatedConfig.OIDC.UI.ShowEmailOTP = false
	}
	if !updatedConfig.SMSOTP.Enabled {
		updatedConfig.OIDC.UI.ShowSMSOTP = false
	}
	if !hasOAuthNew {
		updatedConfig.OIDC.UI.ShowOAuth = false
	}

	configManager.config = updatedConfig
}

// Load fetches the configuration from auth.config table.
func (configManager *ConfigManager) Load(ctx context.Context) error {
	log.Tracef("loading auth configuration from database")

	if configManager.db == nil {
		return nil
	}
	var rawJSON []byte
	err := configManager.db.QueryRow(ctx, "SELECT value FROM auth.config WHERE key = $1", ConfigKey).Scan(&rawJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			defaultConfig := DefaultConfig()
			configManager.Set(defaultConfig)
			log.Debugf("no existing auth config found in database; initialized and saving defaults")
			return configManager.Save(ctx, defaultConfig)
		}
		return fmt.Errorf("failed to query auth.config: %w", err)
	}

	var loadedConfig Config
	if err := json.Unmarshal(rawJSON, &loadedConfig); err != nil {
		return fmt.Errorf("failed to parse auth.config JSON: %w", err)
	}

	configManager.Set(loadedConfig)
	log.Debugf("successfully loaded and parsed auth config from database")
	return nil
}

// Save persists the configuration to auth.config table.
func (configManager *ConfigManager) Save(ctx context.Context, updatedConfig Config) error {
	log.Tracef("saving auth configuration to database")

	if configManager.db == nil {
		configManager.Set(updatedConfig)
		return nil
	}
	rawJSON, _ := json.Marshal(updatedConfig)

	query := `
		INSERT INTO auth.config (key, value, last_updated_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, last_updated_at = clock_timestamp()
	`
	if _, err := configManager.db.Exec(ctx, query, ConfigKey, rawJSON); err != nil {
		return fmt.Errorf("failed to persist auth.config: %w", err)
	}

	configManager.Set(updatedConfig)
	log.Debugf("successfully persisted auth config to database")
	return nil
}

// GetUnencrypted returns the zero-decryption representation suitable for Layr Console.
func (configManager *ConfigManager) GetUnencrypted() Config {
	config := configManager.Get()

	// Strip raw secret strings and set *_configured flags
	if config.OAuthProviders != nil {
		configProviders := make(map[string]OAuthProviderConfig)
		for providerKey, provider := range config.OAuthProviders {
			isConfigured := provider.ClientSecret != ""
			configProviders[providerKey] = OAuthProviderConfig{
				Enabled:                provider.Enabled,
				Preset:                 provider.Preset,
				AuthURL:                provider.AuthURL,
				TokenURL:               provider.TokenURL,
				UserInfoURL:            provider.UserInfoURL,
				ClientID:               provider.ClientID,
				ClientSecretConfigured: isConfigured,
				Scope:                  provider.Scope,
				ResponseMode:           provider.ResponseMode,
				ResponseType:           provider.ResponseType,
				IDAttribute:            provider.IDAttribute,
				EmailAttribute:         provider.EmailAttribute,
				NameAttribute:          provider.NameAttribute,
				AvatarAttribute:        provider.AvatarAttribute,
			}
		}
		config.OAuthProviders = configProviders
	}

	if config.OIDC.Clients != nil {
		configClients := make([]OIDCClientConfig, len(config.OIDC.Clients))
		for index, client := range config.OIDC.Clients {
			isConfigured := client.ClientSecret != ""
			configClients[index] = OIDCClientConfig{
				Name:                    client.Name,
				ClientID:                client.ClientID,
				ClientSecretConfigured:  isConfigured,
				RedirectURIs:            client.RedirectURIs,
				PostSignOutRedirectURIs: client.PostSignOutRedirectURIs,
				Public:                  client.Public,
				Scopes:                  client.Scopes,
			}
		}
		config.OIDC.Clients = configClients
	}

	isPasswordConfigured := config.EmailDispatcher.SMTP.Password != ""
	isSigningSecretConfigured := config.EmailDispatcher.Webhook.SigningSecret != ""
	config.EmailDispatcher.SMTP = EmailDispatcherSMTPConfig{
		Host:               config.EmailDispatcher.SMTP.Host,
		Port:               config.EmailDispatcher.SMTP.Port,
		Username:           config.EmailDispatcher.SMTP.Username,
		PasswordConfigured: isPasswordConfigured,
		TLSMode:            config.EmailDispatcher.SMTP.TLSMode,
		InsecureSkipVerify: config.EmailDispatcher.SMTP.InsecureSkipVerify,
	}
	config.EmailDispatcher.Webhook = EmailDispatcherWebhookConfig{
		URL:                     config.EmailDispatcher.Webhook.URL,
		SigningSecretConfigured: isSigningSecretConfigured,
		TimeoutSeconds:          config.EmailDispatcher.Webhook.TimeoutSeconds,
	}

	isAuthTokenConfigured := config.SMSDispatcher.Twilio.AuthToken != ""
	isSMSSigningSecretConfigured := config.SMSDispatcher.Webhook.SigningSecret != ""
	config.SMSDispatcher.Twilio = SMSDispatcherTwilioConfig{
		AccountSID:          config.SMSDispatcher.Twilio.AccountSID,
		AuthTokenConfigured: isAuthTokenConfigured,
		FromNumber:          config.SMSDispatcher.Twilio.FromNumber,
	}
	config.SMSDispatcher.Webhook = SMSDispatcherWebhookConfig{
		URL:                     config.SMSDispatcher.Webhook.URL,
		SigningSecretConfigured: isSMSSigningSecretConfigured,
		TimeoutSeconds:          config.SMSDispatcher.Webhook.TimeoutSeconds,
	}

	return config
}

// HandleGetConfig handles GET /api/v1/_/auth/config returning sanitized config without raw secrets.
func (configManager *ConfigManager) HandleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Tracef("HandleGetConfig invoked")

	if !configManager.checkScope(request, "auth:config.read") {
		log.Debugf("HandleGetConfig rejected: missing auth:config.read scope")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:config.read required", "LAYR_AUTH_001")
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(configManager.GetUnencrypted())
}

// HandlePutConfig handles PUT /api/v1/_/auth/config with write-only secret updates.
func (configManager *ConfigManager) HandlePutConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Tracef("HandlePutConfig invoked")

	if !configManager.checkScope(request, "auth:config.write") {
		log.Debugf("HandlePutConfig rejected: missing auth:config.write scope")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:config.write required", "LAYR_AUTH_001")
		return
	}

	currentConfig := configManager.Get()
	inputConfig := configManager.Get()
	bodyBytes, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload", "LAYR_AUTH_001")
		return
	}
	defer func() {
		_ = request.Body.Close()
	}()

	if err := json.Unmarshal(bodyBytes, &inputConfig); err != nil {
		log.Debugf("HandlePutConfig rejected: invalid JSON payload: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload", "LAYR_AUTH_001")
		return
	}

	var rawPut struct {
		OIDC struct {
			UI struct {
				ShowPassword *bool `json:"show_password"`
				ShowSignUp   *bool `json:"show_sign_up"`
				ShowPasskeys *bool `json:"show_passkeys"`
				ShowEmailOTP *bool `json:"show_email_otp"`
				ShowSMSOTP   *bool `json:"show_sms_otp"`
				ShowOAuth    *bool `json:"show_oauth"`
			} `json:"ui"`
		} `json:"oidc"`
	}
	_ = json.Unmarshal(bodyBytes, &rawPut)

	// Handle OAuth secrets: if new plaintext provided, envelope-encrypt; if omitted, preserve current
	if inputConfig.OAuthProviders != nil {
		for providerKey, provider := range inputConfig.OAuthProviders {
			currentOAuthProviderConfig := currentConfig.OAuthProviders[providerKey]
			secret := strings.TrimSpace(provider.ClientSecret)

			if secret != "" && !strings.HasPrefix(secret, "enc:v1:") {
				if configManager.cryptoKeyManager != nil {
					encryptedSecret, _ := configManager.cryptoKeyManager.EncryptField([]byte(secret))
					provider.ClientSecret = encryptedSecret
				}
			} else if secret == "" {
				provider.ClientSecret = currentOAuthProviderConfig.ClientSecret
			}
			inputConfig.OAuthProviders[providerKey] = provider
		}
	} else {
		inputConfig.OAuthProviders = currentConfig.OAuthProviders
	}

	// Handle OIDC clients secrets: if new plaintext provided, envelope-encrypt; if omitted, preserve current
	if inputConfig.OIDC.Clients != nil {
		currentOIDCClientConfigsByID := make(map[string]OIDCClientConfig, len(currentConfig.OIDC.Clients))
		for _, currentOIDCClientConfig := range currentConfig.OIDC.Clients {
			currentOIDCClientConfigsByID[currentOIDCClientConfig.ClientID] = currentOIDCClientConfig
		}

		for index, client := range inputConfig.OIDC.Clients {
			currentOIDCClientConfig := currentOIDCClientConfigsByID[client.ClientID]
			secret := strings.TrimSpace(client.ClientSecret)

			if secret != "" && !strings.HasPrefix(secret, "enc:v1:") {
				if configManager.cryptoKeyManager != nil {
					encryptedSecret, _ := configManager.cryptoKeyManager.EncryptField([]byte(secret))
					client.ClientSecret = encryptedSecret
				}
			} else if secret == "" {
				client.ClientSecret = currentOIDCClientConfig.ClientSecret
			}
			inputConfig.OIDC.Clients[index] = client
		}
	} else {
		inputConfig.OIDC.Clients = currentConfig.OIDC.Clients
	}

	// Handle Email secrets: if new plaintext provided, envelope-encrypt; if omitted, preserve current
	if inputConfig.EmailDispatcher.Driver != nil && strings.TrimSpace(*inputConfig.EmailDispatcher.Driver) == "" {
		inputConfig.EmailDispatcher.Driver = nil
	}
	emailSMTPPassword := strings.TrimSpace(inputConfig.EmailDispatcher.SMTP.Password)
	if emailSMTPPassword != "" && !strings.HasPrefix(emailSMTPPassword, "enc:v1:") {
		if configManager.cryptoKeyManager != nil {
			encryptedSecret, _ := configManager.cryptoKeyManager.EncryptField([]byte(emailSMTPPassword))
			inputConfig.EmailDispatcher.SMTP.Password = encryptedSecret
		}
	} else if emailSMTPPassword == "" {
		inputConfig.EmailDispatcher.SMTP.Password = currentConfig.EmailDispatcher.SMTP.Password
	}

	emailWebhookSigningSecret := strings.TrimSpace(inputConfig.EmailDispatcher.Webhook.SigningSecret)
	if emailWebhookSigningSecret != "" && !strings.HasPrefix(emailWebhookSigningSecret, "enc:v1:") {
		if configManager.cryptoKeyManager != nil {
			encryptedSecret, _ := configManager.cryptoKeyManager.EncryptField([]byte(emailWebhookSigningSecret))
			inputConfig.EmailDispatcher.Webhook.SigningSecret = encryptedSecret
		}
	} else if emailWebhookSigningSecret == "" {
		inputConfig.EmailDispatcher.Webhook.SigningSecret = currentConfig.EmailDispatcher.Webhook.SigningSecret
	}

	// Handle SMS secrets: if new plaintext provided, envelope-encrypt; if omitted, preserve current
	if inputConfig.SMSDispatcher.Driver != nil && strings.TrimSpace(*inputConfig.SMSDispatcher.Driver) == "" {
		inputConfig.SMSDispatcher.Driver = nil
	}
	smsTwilioAuthToken := strings.TrimSpace(inputConfig.SMSDispatcher.Twilio.AuthToken)
	if smsTwilioAuthToken != "" && !strings.HasPrefix(smsTwilioAuthToken, "enc:v1:") {
		if configManager.cryptoKeyManager != nil {
			encryptedSecret, _ := configManager.cryptoKeyManager.EncryptField([]byte(smsTwilioAuthToken))
			inputConfig.SMSDispatcher.Twilio.AuthToken = encryptedSecret
		}
	} else if smsTwilioAuthToken == "" {
		inputConfig.SMSDispatcher.Twilio.AuthToken = currentConfig.SMSDispatcher.Twilio.AuthToken
	}

	smsWebhookSigningSecret := strings.TrimSpace(inputConfig.SMSDispatcher.Webhook.SigningSecret)
	if smsWebhookSigningSecret != "" && !strings.HasPrefix(smsWebhookSigningSecret, "enc:v1:") {
		if configManager.cryptoKeyManager != nil {
			encryptedSecret, _ := configManager.cryptoKeyManager.EncryptField([]byte(smsWebhookSigningSecret))
			inputConfig.SMSDispatcher.Webhook.SigningSecret = encryptedSecret
		}
	} else if smsWebhookSigningSecret == "" {
		inputConfig.SMSDispatcher.Webhook.SigningSecret = currentConfig.SMSDispatcher.Webhook.SigningSecret
	}

	if inputConfig.EmailOTP.Enabled && !IsEmailDeliveryReady(inputConfig.EmailDispatcher) {
		log.Debugf("HandlePutConfig rejected: email delivery unconfigured while EmailOTP is enabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, "Email delivery is unavailable because SMTP is not configured by the console user", "LAYR_AUTH_EMAIL_UNCONFIGURED")
		return
	}
	if inputConfig.SMSOTP.Enabled && !IsSMSDeliveryReady(inputConfig.SMSDispatcher) {
		log.Debugf("HandlePutConfig rejected: sms delivery unconfigured while SMSOTP is enabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, "SMS delivery is unavailable because an SMS provider is not configured", "LAYR_AUTH_SMS_UNCONFIGURED")
		return
	}

	hasOAuthInput := false
	for _, provider := range inputConfig.OAuthProviders {
		if provider.Enabled {
			hasOAuthInput = true
			break
		}
	}
	hasOAuthCurrent := false
	for _, provider := range currentConfig.OAuthProviders {
		if provider.Enabled {
			hasOAuthCurrent = true
			break
		}
	}

	// 1. Prevent user to update showXxx method to true when the method isn't enabled in the config
	if rawPut.OIDC.UI.ShowPassword != nil && *rawPut.OIDC.UI.ShowPassword && !inputConfig.Password.Enabled {
		log.Debugf("HandlePutConfig rejected: show_password cannot be enabled when password auth is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_password in UI config when password authentication is disabled", "LAYR_AUTH_001")
		return
	}
	if rawPut.OIDC.UI.ShowSignUp != nil && *rawPut.OIDC.UI.ShowSignUp && !inputConfig.Password.Enabled {
		log.Debugf("HandlePutConfig rejected: show_sign_up cannot be enabled when password auth is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_sign_up in UI config when password authentication is disabled", "LAYR_AUTH_001")
		return
	}
	if rawPut.OIDC.UI.ShowPasskeys != nil && *rawPut.OIDC.UI.ShowPasskeys && !inputConfig.Passkeys.Enabled {
		log.Debugf("HandlePutConfig rejected: show_passkeys cannot be enabled when passkey auth is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_passkeys in UI config when passkey authentication is disabled", "LAYR_AUTH_001")
		return
	}
	if rawPut.OIDC.UI.ShowEmailOTP != nil && *rawPut.OIDC.UI.ShowEmailOTP && !inputConfig.EmailOTP.Enabled {
		log.Debugf("HandlePutConfig rejected: show_email_otp cannot be enabled when email OTP auth is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_email_otp in UI config when email OTP authentication is disabled", "LAYR_AUTH_001")
		return
	}
	if rawPut.OIDC.UI.ShowSMSOTP != nil && *rawPut.OIDC.UI.ShowSMSOTP && !inputConfig.SMSOTP.Enabled {
		log.Debugf("HandlePutConfig rejected: show_sms_otp cannot be enabled when SMS OTP auth is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_sms_otp in UI config when SMS OTP authentication is disabled", "LAYR_AUTH_001")
		return
	}
	if rawPut.OIDC.UI.ShowOAuth != nil && *rawPut.OIDC.UI.ShowOAuth && !hasOAuthInput {
		log.Debugf("HandlePutConfig rejected: show_oauth cannot be enabled when no OAuth providers are enabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_oauth in UI config when no OAuth providers are enabled", "LAYR_AUTH_001")
		return
	}

	// 2. Updating an auth method enabled to true, automatically set showXxx to true in ui config
	if !currentConfig.Password.Enabled && inputConfig.Password.Enabled && rawPut.OIDC.UI.ShowPassword == nil {
		inputConfig.OIDC.UI.ShowPassword = true
	}
	if !currentConfig.Passkeys.Enabled && inputConfig.Passkeys.Enabled && rawPut.OIDC.UI.ShowPasskeys == nil {
		inputConfig.OIDC.UI.ShowPasskeys = true
	}
	if !currentConfig.EmailOTP.Enabled && inputConfig.EmailOTP.Enabled && rawPut.OIDC.UI.ShowEmailOTP == nil {
		inputConfig.OIDC.UI.ShowEmailOTP = true
	}
	if !currentConfig.SMSOTP.Enabled && inputConfig.SMSOTP.Enabled && rawPut.OIDC.UI.ShowSMSOTP == nil {
		inputConfig.OIDC.UI.ShowSMSOTP = true
	}
	if !hasOAuthCurrent && hasOAuthInput && rawPut.OIDC.UI.ShowOAuth == nil {
		inputConfig.OIDC.UI.ShowOAuth = true
	}

	// 3. Setting config auth method to false, automatically make ui config showXxx to false
	if !inputConfig.Password.Enabled {
		inputConfig.OIDC.UI.ShowPassword = false
		inputConfig.OIDC.UI.ShowSignUp = false
	}
	if !inputConfig.Passkeys.Enabled {
		inputConfig.OIDC.UI.ShowPasskeys = false
	}
	if !inputConfig.EmailOTP.Enabled {
		inputConfig.OIDC.UI.ShowEmailOTP = false
	}
	if !inputConfig.SMSOTP.Enabled {
		inputConfig.OIDC.UI.ShowSMSOTP = false
	}
	if !hasOAuthInput {
		inputConfig.OIDC.UI.ShowOAuth = false
	}

	if err := configManager.Save(request.Context(), inputConfig); err != nil {
		log.Debugf("HandlePutConfig failed to persist configuration: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to persist configuration", "LAYR_AUTH_001")
		return
	}

	if configManager.eventBus != nil {
		configManager.eventBus.Publish(request.Context(), NewConfigUpdatedEvent(ConfigKey, ConfigUpdatedEventData(configManager.GetUnencrypted())))
	}

	log.Debugf("HandlePutConfig successfully saved and sanitized configuration")
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(configManager.GetUnencrypted())
}

// DecryptSecret decrypts an envelope-encrypted secret for internal runtime execution.
func (configManager *ConfigManager) DecryptSecret(encrypted string) (string, error) {
	log.Tracef("decrypting secret")

	if encrypted == "" {
		return "", nil
	}
	if configManager.cryptoKeyManager == nil {
		return "", errors.New("key manager is unavailable")
	}
	decryptedSecret, err := configManager.cryptoKeyManager.DecryptField(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt field: %w", err)
	}
	return string(decryptedSecret), nil
}

// GetOIDCClient retrieves an OIDC client configuration by client ID.
func (configManager *ConfigManager) GetOIDCClient(clientID string) (*OIDCClientConfig, bool) {
	log.Tracef("fetching OIDC client for clientID=%s", clientID)

	configManager.rwMutex.RLock()
	defer configManager.rwMutex.RUnlock()

	if !configManager.config.OIDC.Enabled {
		return nil, false
	}

	for _, client := range configManager.config.OIDC.Clients {
		if client.ClientID == clientID {
			copiedOIDCClientConfig := client
			return &copiedOIDCClientConfig, true
		}
	}
	return nil, false
}
