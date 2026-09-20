package core

// Empty represents an empty JSON payload or bodyless response.
type Empty struct{}

// GetHealthResponse represents the /healthz probe response model.
type GetHealthResponse struct {
	Status        string  `json:"status"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	Timestamp     string  `json:"timestamp"`
}

// GetReadinessResponse represents the /readyz probe response model.
type GetReadinessResponse struct {
	Status          string   `json:"status"`
	Database        string   `json:"database"`
	EnabledServices []string `json:"enabled_services,omitempty"`
}

// ManifestProjectInfo represents project metadata in manifest discovery.
type ManifestProjectInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ManifestServerInfo represents host and port in manifest discovery.
type ManifestServerInfo struct {
	ListenAddr string `json:"listen_addr"`
	BaseURL    string `json:"base_url"`
}

// GetManifestResponse represents the project manifest discovery model.
type GetManifestResponse struct {
	Project         ManifestProjectInfo `json:"project"`
	EnabledServices []string            `json:"enabled_services"`
	Server          ManifestServerInfo  `json:"server"`
	PublishableKey  string              `json:"publishable_key,omitempty"`
}
