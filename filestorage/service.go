package filestorage

import (
	"context"
	"fmt"

	"layr.sh/core"
	"layr.sh/filestorage/s3sigv4"
)

func init() {
	factory := func(kernel *core.Kernel) (core.ServiceRunner, error) {
		return NewService(kernel), nil
	}

	core.RegisterServiceFactory("file_storage", factory)
	core.RegisterServiceFactory("filestorage", factory)
}

// Service encapsulates the File Storage service coordinator and handlers.
type Service struct {
	kernel              *core.Kernel
	configManager       *ConfigManager
	databaseEngine      *Engine
	s3Engine            *Engine
	sigv4Validator      *s3sigv4.Validator
	baseHandler         *BaseHandler
	controlPlaneHandler *ControlPlaneHandler
}

// NewService initializes the File Storage service coordinator.
func NewService(kernel *core.Kernel) *Service {
	configManager := NewConfigManager(kernel)
	databaseEngine := NewDatabaseFileStorageEngine(kernel)
	s3Engine := NewS3FileStorageEngine(kernel)
	sigv4Validator := s3sigv4.NewValidator(kernel)

	service := &Service{
		kernel:         kernel,
		configManager:  configManager,
		databaseEngine: databaseEngine,
		s3Engine:       s3Engine,
		sigv4Validator: sigv4Validator,
	}
	service.baseHandler = NewBaseHandler(service)
	service.controlPlaneHandler = NewControlPlaneHandler(service)

	return service
}

// Kernel returns the parent kernel instance.
func (service *Service) Kernel() *core.Kernel {
	return service.kernel
}

// Start loads runtime configuration from PostgreSQL.
func (service *Service) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before start: %w", err)
	}

	return service.configManager.Load(ctx)
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

// DatabaseEngine returns the underlying database file storage engine.
func (service *Service) DatabaseEngine() *Engine {
	return service.databaseEngine
}

// S3Engine returns the underlying S3 file storage engine.
func (service *Service) S3Engine() *Engine {
	return service.s3Engine
}

// SigV4Validator returns the underlying S3 SigV4 validator.
func (service *Service) SigV4Validator() *s3sigv4.Validator {
	return service.sigv4Validator
}
