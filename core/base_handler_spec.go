package core

import (
	"encoding/json"
	"net/http"

	"gopkg.in/yaml.v3"
)

// /api/v1/spec.json - Client OpenAPI 3.1 JSON
func (server *Server) handleBaseSpecJSON(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	openAPISpec := server.baseRouter.OutputOpenAPISpec()
	_ = json.NewEncoder(responseWriter).Encode(openAPISpec)
}

// /api/v1/spec.yaml - Client OpenAPI 3.1 YAML
func (server *Server) handleBaseSpecYAML(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/yaml")
	responseWriter.WriteHeader(http.StatusOK)
	openAPISpec := server.baseRouter.OutputOpenAPISpec()
	_ = yaml.NewEncoder(responseWriter).Encode(openAPISpec)
}
