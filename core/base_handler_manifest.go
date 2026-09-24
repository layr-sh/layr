package core

import (
	"encoding/json"
	"net/http"
)

// /v1/manifest - Dynamic cluster & manifest discovery
func (server *Server) handleGetManifest(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get cluster manifest request")
	publishableKey := server.kernel.cryptoKeyManager.DerivePublishableKey()
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	config := GetConfig()
	_ = json.NewEncoder(responseWriter).Encode(GetManifestResponse{
		Project: ManifestProjectInfo{
			Name:        config.Project.Name,
			Description: config.Project.Description,
		},
		EnabledServices: config.GetEnabledServices(),
		Server: ManifestServerInfo{
			ListenAddr: config.Server.ListenAddr,
			BaseURL:    config.Server.BaseURL,
		},
		PublishableKey: publishableKey,
	})
}
