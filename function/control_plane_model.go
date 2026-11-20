package function

import (
	"errors"
	"time"

	"uuid"

	"layr.sh/core"
)

// Standard sentinel errors for control plane operations.
var (
	ErrEndpointAlreadyExists     = errors.New("endpoint with this name already exists")
	ErrBundleTooLarge            = errors.New("code bundle exceeds maximum allowed size")
	ErrInvalidEndpointName       = errors.New("invalid endpoint name")
	ErrCustomDomainConflict      = errors.New("custom domain already in use")
	ErrCustomDomainNotFound      = errors.New("custom domain not found")
	ErrCustomDomainNotVerified   = errors.New("custom domain must be verified before attaching endpoint routes")
	ErrCustomDomainRouteConflict = errors.New("custom domain route conflict")
	ErrCustomDomainRouteNotFound = errors.New("custom domain route not found")
)

// CreateEndpointInput defines the payload for registering a new serverless endpoint.
type CreateEndpointInput struct {
	Name           string  `json:"name"`
	Description    *string `json:"description,omitempty"`
	Runtime        string  `json:"runtime,omitempty"`
	Entrypoint     string  `json:"entrypoint,omitempty"`
	MemoryLimitMB  int     `json:"memory_limit_mb,omitempty"`
	TimeoutSeconds int     `json:"timeout_seconds,omitempty"`
	IsPublic       *bool   `json:"is_public,omitempty"`
}

// UpdateEndpointInput defines the payload for updating endpoint metadata.
type UpdateEndpointInput struct {
	Description    *string `json:"description,omitempty"`
	Runtime        *string `json:"runtime,omitempty"`
	Entrypoint     *string `json:"entrypoint,omitempty"`
	MemoryLimitMB  *int    `json:"memory_limit_mb,omitempty"`
	TimeoutSeconds *int    `json:"timeout_seconds,omitempty"`
	IsPublic       *bool   `json:"is_public,omitempty"`
}

// ListEndpointsResponse represents the response containing a collection of endpoints.
type ListEndpointsResponse struct {
	Data  []Endpoint `json:"data"`
	Count int        `json:"count"`
}

// CreateDeploymentInput defines the request body for uploading and deploying a code bundle or file tree.
type CreateDeploymentInput struct {
	BundleFormat         string                `json:"bundle_format,omitempty"`
	BundleContent        string                `json:"bundle_content,omitempty"`
	BundleFiles          map[string]string     `json:"bundle_files,omitempty"`
	EnvironmentVariables map[string]string     `json:"environment_variables,omitempty"`
	WorkerdRuntimeConfig *WorkerdRuntimeConfig `json:"workerd_runtime_config,omitempty"`
}

// RollbackDeploymentInput defines the request body for rolling back to a previous deployment.
type RollbackDeploymentInput struct {
	TargetVersion int `json:"target_version"`
}

// ListDeploymentsResponse represents the response containing deployment version history.
type ListDeploymentsResponse struct {
	Data  []Deployment `json:"data"`
	Count int          `json:"count"`
}

// GetStatsResponse represents runtime metrics and health telemetry across runners.
type GetStatsResponse struct {
	Runtimes          map[string]*RunnerHealth `json:"runtimes"`
	TotalEndpoints    int                      `json:"total_endpoints"`
	PublicEndpoints   int                      `json:"public_endpoints"`
	PrivateEndpoints  int                      `json:"private_endpoints"`
	DeployedEndpoints int                      `json:"deployed_endpoints"`
	TotalDeployments  int64                    `json:"total_deployments"`

	BundleSizeBytes             int64           `json:"bundle_size_bytes"`
	TotalDomains                int64           `json:"total_domains"`
	DomainRoutes                int64           `json:"domain_routes"`
	DomainsNeedingTLS           int64           `json:"domains_needing_tls"`
	TotalExecutions             int64           `json:"total_executions"`
	ExecutionsLast24Hours       int64           `json:"executions_last_24_hours"`
	FailedExecutionsLast24Hours int64           `json:"failed_executions_last_24_hours"`
	ClientErrorsLast24Hours     int64           `json:"client_errors_last_24_hours"`
	ServerErrorsLast24Hours     int64           `json:"server_errors_last_24_hours"`
	AverageDurationMs           *float64        `json:"average_duration_ms,omitempty"`
	P95DurationMs               *float64        `json:"p95_duration_ms,omitempty"`
	LastExecutedAt              *time.Time      `json:"last_executed_at,omitempty"`
	WindowStartedAt             time.Time       `json:"window_started_at"`
	GeneratedAt                 time.Time       `json:"generated_at"`
	Endpoints                   []EndpointUsage `json:"endpoints"`
}

// EndpointUsage summarizes retained execution logs within the statistics window.
type EndpointUsage struct {
	Name              string     `json:"name"`
	Runtime           string     `json:"runtime"`
	IsPublic          bool       `json:"is_public"`
	IsDeployed        bool       `json:"is_deployed"`
	Executions        int64      `json:"executions"`
	FailedExecutions  int64      `json:"failed_executions"`
	AverageDurationMs *float64   `json:"average_duration_ms,omitempty"`
	P95DurationMs     *float64   `json:"p95_duration_ms,omitempty"`
	LastExecutedAt    *time.Time `json:"last_executed_at,omitempty"`
}

// ListExecutionsResponse represents the paginated collection of execution logs.
type ListExecutionsResponse struct {
	Data []Execution `json:"data"`
	core.PaginationOutput
}

// CreateCustomDomainInput defines the payload for registering a standalone custom domain.
type CreateCustomDomainInput struct {
	Domain       string  `json:"domain"`
	TLSEnabled   *bool   `json:"tls_enabled,omitempty"`
	TLSAcmeEmail *string `json:"tls_acme_email,omitempty"`
}

// UpdateCustomDomainInput defines the payload for modifying custom domain configuration at runtime.
type UpdateCustomDomainInput struct {
	TLSEnabled   *bool   `json:"tls_enabled,omitempty"`
	TLSAcmeEmail *string `json:"tls_acme_email,omitempty"`
}

// ListCustomDomainsResponse defines the control plane listing output for custom domains.
type ListCustomDomainsResponse struct {
	Data  []CustomDomain `json:"data"`
	Total int            `json:"total"`
}

// CreateCustomDomainRouteInput defines the payload for attaching an endpoint to a custom domain route.
type CreateCustomDomainRouteInput struct {
	CustomDomainID uuid.UUID `json:"custom_domain_id"`
	PathPrefix     *string   `json:"path_prefix,omitempty"`
}

// ListCustomDomainRoutesResponse defines the control plane listing output for custom domain routes.
type ListCustomDomainRoutesResponse struct {
	Data  []CustomDomainRoute `json:"data"`
	Total int                 `json:"total"`
}
