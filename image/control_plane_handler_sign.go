package image

import (
	"encoding/json"
	"net/http"
	"strings"

	"layr.sh/core"
)

// handleSignURL handles POST /v1/_/image/sign generating a signed image URL.
func (controlPlaneHandler *ControlPlaneHandler) handleSignURL(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign image URL request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeImageSignWrite) {
		return
	}

	var signURLInput SignURLInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&signURLInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	rawPath := strings.TrimSpace(signURLInput.Path)
	if rawPath == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Path cannot be empty")
		return
	}

	targetPath := rawPath
	if !strings.HasPrefix(targetPath, "/") {
		targetPath = "/" + targetPath
	}

	signingKey := controlPlaneHandler.configManager.SigningKey()
	signingSalt := controlPlaneHandler.configManager.SigningSalt()

	signature := SignPath(signingKey, signingSalt, targetPath)
	fullURL := "/v1/image/" + signature + targetPath

	controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewURLSignedEvent(targetPath, URLSignedEventData{
		Path:      targetPath,
		URL:       fullURL,
		Signature: signature,
	}))

	log.Debugf("signed url generated for path %s", targetPath)

	signURLResponse := SignURLResponse{
		URL:       fullURL,
		Signature: signature,
	}
	core.WriteJSONResponse(responseWriter, http.StatusOK, signURLResponse)
}
