package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"layr.sh/auth/passkey"
	"layr.sh/auth/password"
	"layr.sh/auth/totp"
	"layr.sh/core"
)

func init() {
	core.RegisterServiceFactory("auth", func(kernel *core.Kernel) (core.ServiceRunner, error) {
		return NewService(kernel), nil
	})
}

// Service coordinates all authentication engines (OIDC, passwords, passkeys, OTP, OAuth).
type Service struct {
	kernel              *core.Kernel
	configManager       *ConfigManager
	passkeyManager      *passkey.Manager
	totpManager         *totp.Manager
	hasher              *password.Hasher
	emailDispatcher     *EmailDispatcher
	smsDispatcher       *SMSDispatcher
	baseHandler         *BaseHandler
	controlPlaneHandler *ControlPlaneHandler
	httpClient          HTTPClient
	dummyPasswordHash   string
}

// NewService initializes the layr/auth service with the active kernel.
func NewService(kernel *core.Kernel) *Service {
	configManager := NewConfigManager(kernel)
	config := configManager.Get()

	emailDispatcher := NewEmailDispatcher(kernel, func() *EmailDispatcherConfig {
		emailDispatcherConfig := configManager.Get().EmailDispatcher
		return &emailDispatcherConfig
	})

	smsDispatcher := NewSMSDispatcher(kernel, func() *SMSDispatcherConfig {
		smsDispatcherConfig := configManager.Get().SMSDispatcher
		return &smsDispatcherConfig
	})

	hasher := password.NewHasher()
	dummyHash, _ := hasher.Hash("antigravity_timing_dummy_password")

	service := &Service{
		kernel:            kernel,
		configManager:     configManager,
		hasher:            hasher,
		passkeyManager:    passkey.NewManager(config.Passkeys.RelyingPartyID, config.Passkeys.RelyingPartyName),
		totpManager:       totp.NewManager(config.MFA.Issuer),
		emailDispatcher:   emailDispatcher,
		smsDispatcher:     smsDispatcher,
		httpClient:        &http.Client{Timeout: 5 * time.Second},
		dummyPasswordHash: dummyHash,
	}

	service.baseHandler = NewBaseHandler(service)
	service.controlPlaneHandler = NewControlPlaneHandler(service)

	return service
}

// Start loads dynamic configuration and initializes HTTP handlers.
func (service *Service) Start(ctx context.Context) error {
	if err := service.configManager.Load(ctx); err != nil {
		return fmt.Errorf("failed to load auth config: %w", err)
	}
	config := service.configManager.Get()
	service.passkeyManager = passkey.NewManager(config.Passkeys.RelyingPartyID, config.Passkeys.RelyingPartyName)
	service.totpManager = totp.NewManager(config.MFA.Issuer)
	return nil
}

// Stop terminates active service routines.
func (service *Service) Stop() {
	log.Tracef("stopping auth service")
}

// Kernel returns the parent kernel instance.
func (service *Service) Kernel() *core.Kernel {
	return service.kernel
}

// ConfigManager returns the dynamic runtime configuration manager.
func (service *Service) ConfigManager() *ConfigManager {
	return service.configManager
}

// BaseHandler returns the underlying public HTTP base handler.
func (service *Service) BaseHandler() *BaseHandler {
	return service.baseHandler
}

// ControlPlaneHandler returns the underlying administrative control plane handler.
func (service *Service) ControlPlaneHandler() *ControlPlaneHandler {
	return service.controlPlaneHandler
}

// Hasher returns the password hasher.
func (service *Service) Hasher() *password.Hasher {
	return service.hasher
}

// PasskeyManager returns the WebAuthn passkey manager.
func (service *Service) PasskeyManager() *passkey.Manager {
	return service.passkeyManager
}

// TOTPManager returns the TOTP manager.
func (service *Service) TOTPManager() *totp.Manager {
	return service.totpManager
}

// EmailDispatcher returns the transactional email dispatcher.
func (service *Service) EmailDispatcher() *EmailDispatcher {
	return service.emailDispatcher
}

// SMSDispatcher returns the SMS dispatcher.
func (service *Service) SMSDispatcher() *SMSDispatcher {
	return service.smsDispatcher
}

// HTTPClient returns the outbound HTTP client.
func (service *Service) HTTPClient() HTTPClient {
	return service.httpClient
}
