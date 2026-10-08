// Package console provides the administrative control plane, operator identity, and dynamic configuration engine.
package console

import (
	"context"
	"fmt"

	"layr.sh/auth/password"
	"layr.sh/auth/totp"
	"layr.sh/core"
)

func init() {
	factory := func(kernel *core.Kernel) (core.ServiceRunner, error) {
		return NewService(kernel), nil
	}

	core.RegisterServiceFactory("console", factory)
}

// Service encapsulates the console control plane and configuration management service.
type Service struct {
	kernel              *core.Kernel
	configManager       *ConfigManager
	hasher              *password.Hasher
	totpManager         *totp.Manager
	controlPlaneHandler *ControlPlaneHandler
}

// NewService initializes the Console service coordinator.
func NewService(kernel *core.Kernel) *Service {
	configManager := NewConfigManager(kernel)
	service := &Service{
		kernel:        kernel,
		configManager: configManager,
		hasher:        password.NewHasher(),
		totpManager:   totp.NewManager("Layr Console"),
	}
	service.controlPlaneHandler = NewControlPlaneHandler(service)
	return service
}

// Kernel returns the parent kernel instance.
func (service *Service) Kernel() *core.Kernel {
	return service.kernel
}

// Start loads runtime configuration.
func (service *Service) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before start: %w", err)
	}

	if err := service.configManager.Load(ctx); err != nil {
		return fmt.Errorf("failed to load console config: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the console service.
func (service *Service) Stop() {
	log.Trace("stopping console service")
}

// ConfigManager returns the dynamic runtime configuration manager.
func (service *Service) ConfigManager() *ConfigManager {
	return service.configManager
}

// ControlPlaneHandler returns the administrative control plane handler.
func (service *Service) ControlPlaneHandler() *ControlPlaneHandler {
	return service.controlPlaneHandler
}
