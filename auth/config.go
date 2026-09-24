// Package auth provides authentication engines, credential verification, and session lifecycle management.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	defaultAdaptiveFailedAttempts    = 5
	defaultKnownDevicesMaxDays       = 30
)

// ConfigKey is the primary key in auth.config table.
const ConfigKey = "runtime"

// ErrInvalidConfig is returned when runtime configuration validation fails.
var ErrInvalidConfig = errors.New("invalid auth configuration")

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
	Threat          ThreatConfig                   `json:"threat"`
}

// ThreatConfig controls automated attack defense, bot challenges, and fraud mitigation.
type ThreatConfig struct {
	BotProtection       BotProtectionConfig `json:"bot_protection"`
	NotifyOnNewDevice   bool                `json:"notify_on_new_device"`
	KnownDevicesMaxDays int                 `json:"known_devices_max_days"`
}

// BotProtectionConfig configures CAPTCHA challenge requirements.
type BotProtectionConfig struct {
	Enabled                bool   `json:"enabled"`
	Provider               string `json:"provider"` // "turnstile", "recaptcha", "hcaptcha"
	SecretKey              string `json:"secret_key,omitempty"`
	SecretKeyConfigured    bool   `json:"secret_key_configured,omitempty"`
	SiteKey                string `json:"site_key,omitempty"`
	Mode                   string `json:"mode"` // "always" or "adaptive"
	AdaptiveFailedAttempts int    `json:"adaptive_failed_attempts"`
}

// PasswordBreachConfig defines HaveIBeenPwned breach checking settings.
type PasswordBreachConfig struct {
	Enabled  bool `json:"enabled"`
	FailOpen bool `json:"fail_open"`
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
	Enabled        bool                 `json:"enabled"`
	MinLength      int                  `json:"min_length"`
	RequireNumbers bool                 `json:"require_numbers"`
	RequireSymbols bool                 `json:"require_symbols"`
	BreachCheck    PasswordBreachConfig `json:"breach_check"`
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
	Enabled      bool     `json:"enabled"`
	Policy       string   `json:"policy"`        // "always" or "adaptive"
	RiskTriggers []string `json:"risk_triggers"` // e.g. ["new_device", "new_ip", "excessive_failed_attempts"]
	Issuer       string   `json:"issuer"`
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

// ResourceServerConfig defines a registered external Resource Server (API) target for M2M tokens.
type ResourceServerConfig struct {
	Name        string   `json:"name"`                  // e.g. "Billing API"
	Identifier  string   `json:"identifier"`            // e.g. "https://billing.example.com"
	Description string   `json:"description,omitempty"` // Optional description
	Scopes      []string `json:"scopes,omitempty"`      // Scopes exposed by this backend, e.g. ["invoices:read", "invoices:write"]
}

// OIDCConfig defines configuration for Layr Auth as an OpenID Connect (OIDC) Identity Provider.
type OIDCConfig struct {
	Enabled         bool                   `json:"enabled"`
	Clients         []OIDCClientConfig     `json:"clients,omitempty"`
	ResourceServers []ResourceServerConfig `json:"resource_servers,omitempty"`
	UI              OIDCUIConfig           `json:"ui"`
}

// OIDCClientConfig defines a registered third-party OpenID Connect client application.
type OIDCClientConfig struct {
	Name                               string   `json:"name"`
	ClientID                           string   `json:"client_id"`
	ClientSecret                       string   `json:"client_secret,omitempty"`
	ClientSecretConfigured             bool     `json:"client_secret_configured,omitempty"`
	RedirectURIs                       []string `json:"redirect_uris"`
	PostSignOutRedirectURIs            []string `json:"post_sign_out_redirect_uris,omitempty"`
	Public                             bool     `json:"public"` // True for PKCE-only public clients (SPA/mobile); false for confidential clients
	Scopes                             []string `json:"scopes,omitempty"`
	BackChannelSignOutURI              string   `json:"backchannel_sign_out_uri,omitempty"`
	BackChannelSignOutSessionRequired  bool     `json:"backchannel_sign_out_session_required,omitempty"`
	FrontChannelSignOutURI             string   `json:"frontchannel_sign_out_uri,omitempty"`
	FrontChannelSignOutSessionRequired bool     `json:"frontchannel_sign_out_session_required,omitempty"`
}

// UnmarshalJSON supports both standard Layr sign-out JSON keys and standard OIDC logout wire aliases.
func (clientConfig *OIDCClientConfig) UnmarshalJSON(data []byte) error {
	type Alias OIDCClientConfig
	var raw struct {
		Alias
		BackChannelLogoutURI              string `json:"backchannel_logout_uri"`
		BackChannelLogoutSessionRequired  *bool  `json:"backchannel_logout_session_required"`
		FrontChannelLogoutURI             string `json:"frontchannel_logout_uri"`
		FrontChannelLogoutSessionRequired *bool  `json:"frontchannel_logout_session_required"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*clientConfig = OIDCClientConfig(raw.Alias)
	if clientConfig.BackChannelSignOutURI == "" && raw.BackChannelLogoutURI != "" {
		clientConfig.BackChannelSignOutURI = raw.BackChannelLogoutURI
	}
	if !clientConfig.BackChannelSignOutSessionRequired && raw.BackChannelLogoutSessionRequired != nil {
		clientConfig.BackChannelSignOutSessionRequired = *raw.BackChannelLogoutSessionRequired
	}
	if clientConfig.FrontChannelSignOutURI == "" && raw.FrontChannelLogoutURI != "" {
		clientConfig.FrontChannelSignOutURI = raw.FrontChannelLogoutURI
	}
	if !clientConfig.FrontChannelSignOutSessionRequired && raw.FrontChannelLogoutSessionRequired != nil {
		clientConfig.FrontChannelSignOutSessionRequired = *raw.FrontChannelLogoutSessionRequired
	}
	return nil
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
			BreachCheck: PasswordBreachConfig{
				Enabled:  false,
				FailOpen: true,
			},
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
			Enabled:      true,
			Policy:       "always",
			RiskTriggers: []string{"new_device", "new_ip", "excessive_failed_attempts"},
			Issuer:       mfaIssuer,
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
			Enabled:         false,
			Clients:         make([]OIDCClientConfig, 0),
			ResourceServers: make([]ResourceServerConfig, 0),
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
		Threat: ThreatConfig{
			BotProtection: BotProtectionConfig{
				Enabled:                false,
				Provider:               "turnstile",
				Mode:                   "adaptive",
				AdaptiveFailedAttempts: defaultAdaptiveFailedAttempts,
			},
			NotifyOnNewDevice:   true,
			KnownDevicesMaxDays: defaultKnownDevicesMaxDays,
		},
	}
}

// Validate verifies whether the configuration settings are valid.
func (config Config) Validate() error {
	if config.Sessions.AccessTokenExpirySeconds <= 0 {
		return fmt.Errorf("sessions access_token_expiry_seconds must be greater than 0, got %d", config.Sessions.AccessTokenExpirySeconds)
	}
	if config.Sessions.RefreshTokenExpirySeconds <= 0 {
		return fmt.Errorf("sessions refresh_token_expiry_seconds must be greater than 0, got %d", config.Sessions.RefreshTokenExpirySeconds)
	}
	if config.Sessions.IdleTimeoutSeconds <= 0 {
		return fmt.Errorf("sessions idle_timeout_seconds must be greater than 0, got %d", config.Sessions.IdleTimeoutSeconds)
	}
	if config.Password.MinLength < 0 {
		return fmt.Errorf("password min_length cannot be negative, got %d", config.Password.MinLength)
	}
	if config.Threat.BotProtection.Enabled {
		provider := strings.ToLower(strings.TrimSpace(config.Threat.BotProtection.Provider))
		switch provider {
		case "turnstile", "cloudflare", "recaptcha", "google", "hcaptcha":
		default:
			return fmt.Errorf("unsupported bot protection provider: %s", config.Threat.BotProtection.Provider)
		}
		if config.Threat.BotProtection.SecretKey == "" && !config.Threat.BotProtection.SecretKeyConfigured {
			return errors.New("bot protection secret key is required when enabled")
		}
		mode := strings.ToLower(strings.TrimSpace(config.Threat.BotProtection.Mode))
		if mode != "" && mode != "always" && mode != "adaptive" {
			return fmt.Errorf("invalid bot protection mode: %s", config.Threat.BotProtection.Mode)
		}
	}
	return nil
}

// ConfigManager handles loading, validating, caching, and envelope encryption of auth.config.
type ConfigManager struct {
	kernel  *core.Kernel
	rwMutex sync.RWMutex
	config  Config
}

// NewConfigManager initializes a new ConfigManager.
func NewConfigManager(kernel *core.Kernel) *ConfigManager {
	return &ConfigManager{
		kernel: kernel,
		config: DefaultConfig(),
	}
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
	if configManager.config.OIDC.ResourceServers != nil {
		copiedConfig.OIDC.ResourceServers = make([]ResourceServerConfig, len(configManager.config.OIDC.ResourceServers))
		for index, resourceServer := range configManager.config.OIDC.ResourceServers {
			resourceServerConfig := resourceServer
			if resourceServer.Scopes != nil {
				resourceServerConfig.Scopes = make([]string, len(resourceServer.Scopes))
				copy(resourceServerConfig.Scopes, resourceServer.Scopes)
			}
			copiedConfig.OIDC.ResourceServers[index] = resourceServerConfig
		}
	}
	if configManager.config.MFA.RiskTriggers != nil {
		copiedConfig.MFA.RiskTriggers = make([]string, len(configManager.config.MFA.RiskTriggers))
		copy(copiedConfig.MFA.RiskTriggers, configManager.config.MFA.RiskTriggers)
	}
	return copiedConfig
}

// SetMemoryConfig updates the in-memory configuration with sane fallbacks.
func (configManager *ConfigManager) SetMemoryConfig(updatedConfig Config) {
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
	if updatedConfig.MFA.Policy == "" {
		updatedConfig.MFA.Policy = "always"
	}
	if updatedConfig.MFA.Policy == "adaptive" && len(updatedConfig.MFA.RiskTriggers) == 0 {
		updatedConfig.MFA.RiskTriggers = []string{"new_device", "new_ip", "excessive_failed_attempts"}
	}
	if updatedConfig.Threat.KnownDevicesMaxDays <= 0 {
		updatedConfig.Threat.KnownDevicesMaxDays = defaultKnownDevicesMaxDays
	}
	if updatedConfig.Threat.BotProtection.Mode == "" {
		updatedConfig.Threat.BotProtection.Mode = "adaptive"
	}
	if updatedConfig.Threat.BotProtection.AdaptiveFailedAttempts <= 0 {
		updatedConfig.Threat.BotProtection.AdaptiveFailedAttempts = defaultAdaptiveFailedAttempts
	}
	if updatedConfig.MFA.RiskTriggers != nil {
		copiedRiskTriggers := make([]string, len(updatedConfig.MFA.RiskTriggers))
		copy(copiedRiskTriggers, updatedConfig.MFA.RiskTriggers)
		updatedConfig.MFA.RiskTriggers = copiedRiskTriggers
	}
	if updatedConfig.OAuthProviders == nil {
		updatedConfig.OAuthProviders = make(map[string]OAuthProviderConfig)
	}
	if updatedConfig.OIDC.Clients == nil {
		updatedConfig.OIDC.Clients = make([]OIDCClientConfig, 0)
	}
	if updatedConfig.OIDC.ResourceServers == nil {
		updatedConfig.OIDC.ResourceServers = make([]ResourceServerConfig, 0)
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
	if updatedConfig.OIDC.ResourceServers != nil {
		copiedResourceServers := make([]ResourceServerConfig, len(updatedConfig.OIDC.ResourceServers))
		for index, resourceServer := range updatedConfig.OIDC.ResourceServers {
			resourceServerConfig := resourceServer
			if resourceServer.Scopes != nil {
				resourceServerConfig.Scopes = make([]string, len(resourceServer.Scopes))
				copy(resourceServerConfig.Scopes, resourceServer.Scopes)
			}
			copiedResourceServers[index] = resourceServerConfig
		}
		updatedConfig.OIDC.ResourceServers = copiedResourceServers
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

	var rawJSON []byte
	err := configManager.kernel.DB().QueryRow(ctx, "SELECT value FROM auth.config WHERE key = $1", ConfigKey).Scan(&rawJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			defaultConfig := DefaultConfig()
			log.Debugf("no existing auth config found in database; initialized and saving defaults")
			return configManager.Set(ctx, defaultConfig)
		}
		return fmt.Errorf("failed to query auth.config: %w", err)
	}

	var loadedConfig Config
	if err := json.Unmarshal(rawJSON, &loadedConfig); err != nil {
		return fmt.Errorf("failed to parse auth.config JSON: %w", err)
	}

	if err := loadedConfig.Validate(); err != nil {
		return fmt.Errorf("stored auth config is invalid: %w", err)
	}

	configManager.SetMemoryConfig(loadedConfig)
	log.Debugf("successfully loaded and parsed auth config from database")
	return nil
}

// Set persists the configuration to auth.config table and updates memory snapshot.
func (configManager *ConfigManager) Set(ctx context.Context, updatedConfig Config) error {
	log.Tracef("saving auth configuration to database")

	if err := updatedConfig.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	configManager.SetMemoryConfig(updatedConfig)
	activeConfig := configManager.Get()

	rawJSON, _ := json.Marshal(activeConfig)

	const upsertSQLStatement = `
		INSERT INTO auth.config (key, value, last_updated_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, last_updated_at = clock_timestamp();
	`
	if _, err := configManager.kernel.DB().Exec(ctx, upsertSQLStatement, ConfigKey, rawJSON); err != nil {
		return fmt.Errorf("failed to persist auth.config: %w", err)
	}

	configManager.kernel.EventBus().Publish(ctx, NewConfigUpdatedEvent("auth.config", ConfigUpdatedEventData(configManager.GetUnencrypted())))
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
				Name:                               client.Name,
				ClientID:                           client.ClientID,
				ClientSecretConfigured:             isConfigured,
				RedirectURIs:                       client.RedirectURIs,
				PostSignOutRedirectURIs:            client.PostSignOutRedirectURIs,
				Public:                             client.Public,
				Scopes:                             client.Scopes,
				BackChannelSignOutURI:              client.BackChannelSignOutURI,
				BackChannelSignOutSessionRequired:  client.BackChannelSignOutSessionRequired,
				FrontChannelSignOutURI:             client.FrontChannelSignOutURI,
				FrontChannelSignOutSessionRequired: client.FrontChannelSignOutSessionRequired,
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

	isCaptchaSecretConfigured := config.Threat.BotProtection.SecretKey != ""
	config.Threat.BotProtection = BotProtectionConfig{
		Enabled:                config.Threat.BotProtection.Enabled,
		Provider:               config.Threat.BotProtection.Provider,
		SecretKeyConfigured:    isCaptchaSecretConfigured,
		SiteKey:                config.Threat.BotProtection.SiteKey,
		Mode:                   config.Threat.BotProtection.Mode,
		AdaptiveFailedAttempts: config.Threat.BotProtection.AdaptiveFailedAttempts,
	}

	return config
}

// DecryptSecret decrypts an envelope-encrypted secret for internal runtime execution.
func (configManager *ConfigManager) DecryptSecret(encrypted string) (string, error) {
	log.Tracef("decrypting secret")

	if encrypted == "" {
		return "", nil
	}
	decryptedSecret, err := configManager.kernel.CryptoKeyManager().DecryptField(encrypted)
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
