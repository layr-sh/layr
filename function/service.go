// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"encoding/json"
	"fmt"

	"layr.sh/core"
)

func init() {
	factory := func(kernel *core.Kernel) (core.ServiceRunner, error) {
		return NewService(kernel), nil
	}

	core.RegisterServiceFactory("function", factory)
}

// Service encapsulates the serverless function execution engine coordinator.
type Service struct {
	kernel              *core.Kernel
	configManager       *ConfigManager
	engine              *Engine
	baseHandler         *BaseHandler
	controlPlaneHandler *ControlPlaneHandler
}

// NewService initializes the Function service coordinator.
func NewService(kernel *core.Kernel) *Service {
	configManager := NewConfigManager(kernel)
	engine := NewEngine()

	workerdRunner := NewWorkerdRunner(configManager)
	engine.RegisterRunner(workerdRunner)

	service := &Service{
		kernel:        kernel,
		configManager: configManager,
		engine:        engine,
	}

	service.baseHandler = NewBaseHandler(service)
	service.controlPlaneHandler = NewControlPlaneHandler(service)

	return service
}

// Kernel returns the parent kernel instance.
func (service *Service) Kernel() *core.Kernel {
	return service.kernel
}

// ConfigManager returns the dynamic runtime configuration manager.
func (service *Service) ConfigManager() *ConfigManager {
	return service.configManager
}

// Engine returns the pluggable runtime execution engine coordinator.
func (service *Service) Engine() *Engine {
	return service.engine
}

// BaseHandler returns the public data plane HTTP handler.
func (service *Service) BaseHandler() *BaseHandler {
	return service.baseHandler
}

// ControlPlaneHandler returns the administrative control plane HTTP handler.
func (service *Service) ControlPlaneHandler() *ControlPlaneHandler {
	return service.controlPlaneHandler
}

// RegisterHostRoutes registers host-based ingress interceptors on the HTTP gateway server.
func (service *Service) RegisterHostRoutes(server *core.Server) {
	server.RegisterHostHandler(service.baseHandler.HandleCustomDomain)
}

// Start loads runtime configuration, starts the execution engine, and loads active endpoints.
func (service *Service) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before start: %w", err)
	}

	if loadErr := service.configManager.Load(ctx); loadErr != nil {
		return fmt.Errorf("failed to load function config: %w", loadErr)
	}

	if startErr := service.engine.Start(ctx); startErr != nil {
		return fmt.Errorf("failed to start execution engine: %w", startErr)
	}

	if loadEndpointsErr := service.loadActiveEndpoints(ctx); loadEndpointsErr != nil {
		log.Warnf("failed to load active endpoints: %v", loadEndpointsErr)
	}

	return nil
}

// loadActiveEndpoints queries all endpoints that have active deployments and loads them into the engine.
func (service *Service) loadActiveEndpoints(ctx context.Context) error {
	const querySQL = `
		SELECT f.id, f.name, f.description, f.runtime, f.entrypoint, f.memory_limit_mb, f.timeout_seconds, f.is_public, f.active_deployment_id, f.created_at, f.updated_at,
		       d.id, d.endpoint_id, d.version, d.bundle_format, d.bundle_content, d.bundle_hash, d.bundle_files, d.environment_variables, d.workerd_runtime_config, d.status, d.created_at
		FROM function.endpoints f
		JOIN function.deployments d ON f.active_deployment_id = d.id;
	`

	rows, queryErr := service.kernel.DB().Query(ctx, querySQL)
	if queryErr != nil {
		return fmt.Errorf("failed to query active endpoints: %w", queryErr)
	}
	defer rows.Close()

	for rows.Next() {
		var storedEndpoint Endpoint
		var deployment Deployment
		var rawFilesJSON []byte
		var rawEnvironmentJSON []byte
		var rawWorkerdRuntimeConfigJSON []byte

		_ = rows.Scan(
			&storedEndpoint.ID,
			&storedEndpoint.Name,
			&storedEndpoint.Description,
			&storedEndpoint.Runtime,
			&storedEndpoint.Entrypoint,
			&storedEndpoint.MemoryLimitMB,
			&storedEndpoint.TimeoutSeconds,
			&storedEndpoint.IsPublic,
			&storedEndpoint.ActiveDeploymentID,
			&storedEndpoint.CreatedAt,
			&storedEndpoint.UpdatedAt,
			&deployment.ID,
			&deployment.EndpointID,
			&deployment.Version,
			&deployment.BundleFormat,
			&deployment.BundleContent,
			&deployment.BundleHash,
			&rawFilesJSON,
			&rawEnvironmentJSON,
			&rawWorkerdRuntimeConfigJSON,
			&deployment.Status,
			&deployment.CreatedAt,
		)

		if len(rawFilesJSON) > 0 {
			_ = json.Unmarshal(rawFilesJSON, &deployment.BundleFiles)
		}
		if len(rawEnvironmentJSON) > 0 {
			_ = json.Unmarshal(rawEnvironmentJSON, &deployment.EnvironmentVariables)
		}
		if len(rawWorkerdRuntimeConfigJSON) > 0 && string(rawWorkerdRuntimeConfigJSON) != "null" {
			_ = json.Unmarshal(rawWorkerdRuntimeConfigJSON, &deployment.WorkerdRuntimeConfig)
		}

		if deployErr := service.engine.Deploy(ctx, &storedEndpoint, &deployment); deployErr != nil {
			log.Warnf("failed to deploy active endpoint %s on startup: %v", storedEndpoint.Name, deployErr)
		}
	}

	return nil
}

// Stop gracefully stops the execution engine and cleans up running runners.
func (service *Service) Stop() {
	if stopErr := service.engine.Stop(); stopErr != nil {
		log.Warnf("failed to stop execution engine: %v", stopErr)
	}
}
