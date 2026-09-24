package core

import (
	"encoding/json"
	"net/http"

	"gopkg.in/yaml.v3"
)

// /v1/spec.json - Client OpenAPI 3.1 JSON
func (server *Server) handleGetBaseSpecJSON(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get public OpenAPI JSON spec request")
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	openAPISpec := server.baseRouter.OutputOpenAPISpec()
	_ = json.NewEncoder(responseWriter).Encode(openAPISpec)
}

// /v1/spec.yaml - Client OpenAPI 3.1 YAML
func (server *Server) handleGetBaseSpecYAML(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get public OpenAPI YAML spec request")
	responseWriter.Header().Set("Content-Type", "application/yaml")
	responseWriter.WriteHeader(http.StatusOK)
	openAPISpec := server.baseRouter.OutputOpenAPISpec()
	_ = yaml.NewEncoder(responseWriter).Encode(openAPISpec)
}
