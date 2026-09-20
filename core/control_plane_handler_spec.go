package core

import (
	"encoding/json"
	"net/http"

	"gopkg.in/yaml.v3"
)

// /api/v1/_/spec.json - Protected Control Plane OpenAPI 3.1 JSON
func (server *Server) handleGetControlPlaneSpecJSON(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	openAPISpec := server.controlPlaneRouter.OutputOpenAPISpec()
	_ = json.NewEncoder(responseWriter).Encode(openAPISpec)
}

// /api/v1/_/spec.yaml - Protected Control Plane OpenAPI 3.1 YAML
func (server *Server) handleGetControlPlaneSpecYAML(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/yaml")
	responseWriter.WriteHeader(http.StatusOK)
	openAPISpec := server.controlPlaneRouter.OutputOpenAPISpec()
	_ = yaml.NewEncoder(responseWriter).Encode(openAPISpec)
}
