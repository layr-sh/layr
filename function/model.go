package function

import (
	"time"

	"uuid"
)

// Endpoint represents a serverless execution endpoint in function.endpoints.
type Endpoint struct {
	ID                 uuid.UUID  `json:"id"`
	Name               string     `json:"name"`
	Description        *string    `json:"description,omitempty"`
	Runtime            string     `json:"runtime"`
	Entrypoint         string     `json:"entrypoint"`
	MemoryLimitMB      int        `json:"memory_limit_mb"`
	TimeoutSeconds     int        `json:"timeout_seconds"`
	IsPublic           bool       `json:"is_public"`
	ActiveDeploymentID *uuid.UUID `json:"active_deployment_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Deployment represents an immutable deployment version containing bundled code in function.deployments.
type Deployment struct {
	ID                   uuid.UUID             `json:"id"`
	EndpointID           uuid.UUID             `json:"endpoint_id"`
	Version              int                   `json:"version"`
	BundleFormat         string                `json:"bundle_format"`
	BundleContent        []byte                `json:"-"`
	BundleHash           string                `json:"bundle_hash"`
	BundleFiles          map[string]any        `json:"bundle_files"`
	EnvironmentVariables map[string]string     `json:"environment_variables"`
	WorkerdRuntimeConfig *WorkerdRuntimeConfig `json:"workerd_runtime_config,omitempty"`
	Status               string                `json:"status"`
	CreatedAt            time.Time             `json:"created_at"`
}

// Execution represents an immutable record of an endpoint invocation and execution logs.
type Execution struct {
	ID           uuid.UUID  `json:"id"`
	EndpointID   uuid.UUID  `json:"endpoint_id"`
	DeploymentID *uuid.UUID `json:"deployment_id,omitempty"`
	Method       string     `json:"method"`
	Path         string     `json:"path"`
	StatusCode   int        `json:"status_code"`
	DurationMs   int64      `json:"duration_ms"`
	Stdout       string     `json:"stdout"`
	Stderr       string     `json:"stderr"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// CustomDomain represents a registered custom domain hostname in function.custom_domains.
type CustomDomain struct {
	ID        uuid.UUID `json:"id"`
	Domain    string    `json:"domain"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
