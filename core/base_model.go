package core

// HealthResponse represents the /healthz probe response model.
type HealthResponse struct {
	Status        string  `json:"status"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	Timestamp     string  `json:"timestamp"`
}

// ReadyResponse represents the /readyz probe response model.
type ReadyResponse struct {
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

// ManifestResponse represents the project manifest discovery model.
type ManifestResponse struct {
	Project         ManifestProjectInfo `json:"project"`
	EnabledServices []string            `json:"enabled_services"`
	Server          ManifestServerInfo  `json:"server"`
	PublishableKey  string              `json:"publishable_key,omitempty"`
}
