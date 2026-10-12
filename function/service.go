// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"layr.sh/core"
)

func init() {
	core.RegisterServiceFactory("function", func(kernel *core.Kernel) (core.ServiceRunner, error) {
		return NewService(kernel), nil
	})
}

type hostValidationCacheEntry struct {
	valid     bool
	expiresAt time.Time
}

// Service encapsulates the serverless function execution engine coordinator.
type Service struct {
	kernel                 *core.Kernel
	configManager          *ConfigManager
	engine                 *Engine
	baseHandler            *BaseHandler
	controlPlaneHandler    *ControlPlaneHandler
	hostValidationCacheMap sync.Map
	registerHostRoutesOnce sync.Once
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

// RegisterHostRoutes registers host-based ingress interceptors and host validators on the HTTP gateway server.
func (service *Service) RegisterHostRoutes(server *core.Server) {
	if server == nil {
		return
	}
	service.registerHostRoutesOnce.Do(func() {
		server.RegisterHostHandler(service.baseHandler.HandleCustomDomain)
		server.RegisterHostValidator(service.validateCustomDomainHost)
		server.DBCertCache().AddCertPutCallback(service.handleCertPut)
	})
}

const (
	certUpdateTimeout      = 5 * time.Second
	hostValidationCacheTTL = 1 * time.Minute
)

// InvalidateHostValidationCache clears cached host validation for a domain or all domains if domain is empty.
func (service *Service) InvalidateHostValidationCache(domain string) {
	if domain == "" {
		service.hostValidationCacheMap.Range(func(key, value any) bool {
			service.hostValidationCacheMap.Delete(key)
			return true
		})
		return
	}
	normalized, err := NormalizeAndValidateDomain(domain)
	if err == nil {
		service.hostValidationCacheMap.Delete(normalized)
	} else {
		service.hostValidationCacheMap.Delete(domain)
	}
}

// handleCertPut updates custom domain TLS status when a certificate is stored in DBCertCache.
func (service *Service) handleCertPut(key string, certData []byte) {
	var certDER []byte
	remaining := certData
	for {
		var block *pem.Block
		block, remaining = pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			certDER = block.Bytes
			break
		}
	}

	if certDER == nil {
		return
	}

	certificate, parseErr := x509.ParseCertificate(certDER)
	if parseErr != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), certUpdateTimeout)
	defer cancel()

	const updateSQL = `
		UPDATE function.custom_domains
		SET tls_status = 'active',
			tls_issued_at = $1,
			tls_expires_at = $2,
			tls_error_message = NULL,
			updated_at = clock_timestamp()
		WHERE domain = $3 AND tls_enabled = true
		RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
	`
	var customDomain CustomDomain
	if scanErr := service.kernel.DB().QueryRow(ctx, updateSQL, certificate.NotBefore, certificate.NotAfter, key).Scan(
		&customDomain.ID,
		&customDomain.Domain,
		&customDomain.Status,
		&customDomain.TLSEnabled,
		&customDomain.TLSStatus,
		&customDomain.TLSAcmeEmail,
		&customDomain.TLSErrorMessage,
		&customDomain.TLSIssuedAt,
		&customDomain.TLSExpiresAt,
		&customDomain.CreatedAt,
		&customDomain.UpdatedAt,
	); scanErr == nil {
		service.InvalidateHostValidationCache(key)
		service.kernel.EventBus().Publish(
			ctx,
			NewCustomDomainUpdatedEvent(customDomain.ID.String(), CustomDomainUpdatedEventData(customDomain)),
		)
	}
}

// validateCustomDomainHost validates whether an incoming SNI hostname is a configured active custom domain with TLS enabled.
func (service *Service) validateCustomDomainHost(ctx context.Context, host string) (bool, error) {
	normalizedHost, err := NormalizeAndValidateDomain(host)
	if err != nil {
		return false, nil
	}

	if storedEntry, ok := service.hostValidationCacheMap.Load(normalizedHost); ok {
		if entry, ok := storedEntry.(hostValidationCacheEntry); ok {
			if time.Now().Before(entry.expiresAt) {
				return entry.valid, nil
			}
			service.hostValidationCacheMap.Delete(normalizedHost)
		}
	}

	const query = `
		SELECT EXISTS(
			SELECT 1 FROM function.custom_domains
			WHERE domain = $1 AND status = 'active' AND tls_enabled = true AND tls_status != 'failed'
		);
	`
	var exists bool
	if queryErr := service.kernel.DB().QueryRow(ctx, query, normalizedHost).Scan(&exists); queryErr != nil {
		return false, fmt.Errorf("failed to validate custom domain host %q: %w", host, queryErr)
	}

	service.hostValidationCacheMap.Store(normalizedHost, hostValidationCacheEntry{
		valid:     exists,
		expiresAt: time.Now().Add(hostValidationCacheTTL),
	})

	return exists, nil
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
