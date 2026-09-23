package image

import (
	"context"
	"fmt"

	"layr.sh/core"
)

func init() {
	factory := func(kernel *core.Kernel) (core.ServiceRunner, error) {
		return NewService(kernel), nil
	}

	core.RegisterServiceFactory("image", factory)
}

// Service encapsulates the Image transformation service coordinator, dependencies, and HTTP handlers.
type Service struct {
	kernel              *core.Kernel
	configManager       *ConfigManager
	presetManager       *PresetManager
	cacheManager        *CacheManager
	engine              *Engine
	fetcher             *Fetcher
	baseHandler         *BaseHandler
	controlPlaneHandler *ControlPlaneHandler
}

// NewService initializes the Image service coordinator.
func NewService(kernel *core.Kernel) *Service {
	configManager := NewConfigManager(kernel)
	presetManager := NewPresetManager(kernel)
	cacheManager := NewCacheManager(kernel)
	engine := NewEngine(configManager)
	fetcher := NewFetcher(kernel, configManager)

	service := &Service{
		kernel:        kernel,
		configManager: configManager,
		presetManager: presetManager,
		cacheManager:  cacheManager,
		engine:        engine,
		fetcher:       fetcher,
	}
	service.baseHandler = NewBaseHandler(service)
	service.controlPlaneHandler = NewControlPlaneHandler(service)

	return service
}

// Kernel returns the parent kernel instance.
func (service *Service) Kernel() *core.Kernel {
	return service.kernel
}

// Start loads runtime configuration and presets from PostgreSQL.
func (service *Service) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before start: %w", err)
	}

	if err := service.configManager.Load(ctx); err != nil {
		return fmt.Errorf("failed to load image config: %w", err)
	}

	if err := service.presetManager.Load(ctx); err != nil {
		return fmt.Errorf("failed to load presets: %w", err)
	}

	return nil
}

// Stop gracefully shuts down active workers or resources.
func (service *Service) Stop() {
}

// BaseHandler returns the underlying public HTTP base handler.
func (service *Service) BaseHandler() *BaseHandler {
	return service.baseHandler
}

// ControlPlaneHandler returns the underlying administrative control plane handler.
func (service *Service) ControlPlaneHandler() *ControlPlaneHandler {
	return service.controlPlaneHandler
}

// ConfigManager returns the dynamic runtime configuration manager.
func (service *Service) ConfigManager() *ConfigManager {
	return service.configManager
}

// PresetManager returns the preset manager.
func (service *Service) PresetManager() *PresetManager {
	return service.presetManager
}

// CacheManager returns the cache manager.
func (service *Service) CacheManager() *CacheManager {
	return service.cacheManager
}

// Engine returns the transformation engine.
func (service *Service) Engine() *Engine {
	return service.engine
}

// Fetcher returns the asset fetcher.
func (service *Service) Fetcher() *Fetcher {
	return service.fetcher
}
