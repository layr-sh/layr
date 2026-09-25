package function

import (
	"errors"
	"time"

	"uuid"
)

// Standard sentinel errors for control plane operations.
var (
	ErrEndpointAlreadyExists     = errors.New("endpoint with this name already exists")
	ErrBundleTooLarge            = errors.New("code bundle exceeds maximum allowed size")
	ErrInvalidEndpointName       = errors.New("invalid endpoint name")
	ErrCustomDomainConflict      = errors.New("custom domain already in use")
	ErrCustomDomainNotFound      = errors.New("custom domain not found")
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
	Endpoints []Endpoint `json:"endpoints"`
	Count     int        `json:"count"`
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
	Deployments []Deployment `json:"deployments"`
	Count       int          `json:"count"`
}

// GetStatsResponse represents runtime metrics and health telemetry across runners.
type GetStatsResponse struct {
	Runtimes       map[string]*RunnerHealth `json:"runtimes"`
	TotalEndpoints int                      `json:"total_endpoints"`
}

// ListExecutionsResponse represents the paginated collection of execution logs.
type ListExecutionsResponse struct {
	Executions []Execution `json:"executions"`
	Count      int         `json:"count"`
}

// CustomDomain represents a registered custom domain hostname in function.custom_domains.
type CustomDomain struct {
	ID        uuid.UUID `json:"id"`
	Domain    string    `json:"domain"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateCustomDomainInput defines the payload for registering a standalone custom domain.
type CreateCustomDomainInput struct {
	Domain string `json:"domain"`
}

// ListCustomDomainsResponse defines the control plane listing output for custom domains.
type ListCustomDomainsResponse struct {
	Data  []CustomDomain `json:"data"`
	Total int            `json:"total"`
}

// CustomDomainRoute represents an endpoint route mapping on a custom domain in function.custom_domain_routes.
type CustomDomainRoute struct {
	ID             uuid.UUID `json:"id"`
	CustomDomainID uuid.UUID `json:"custom_domain_id"`
	Domain         string    `json:"domain,omitempty"`
	EndpointID     uuid.UUID `json:"endpoint_id"`
	PathPrefix     string    `json:"path_prefix"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
