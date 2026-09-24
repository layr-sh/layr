package image

import (
	"errors"
	"net/http"
	"strings"

	"layr.sh/core"
)

// handleInfo handles GET /v1/image/info/{signature}/{path...}.
func (baseHandler *BaseHandler) handleInfo(responseWriter http.ResponseWriter, request *http.Request) {
	signature := request.PathValue("signature")
	rawPath := request.PathValue("path")
	log.Tracef("handling image info request: path=%s", rawPath)
	if signature == "" || rawPath == "" {
		baseHandler.writeInfoError(responseWriter, request, rawPath, http.StatusBadRequest, "Signature and info path are required")
		return
	}

	fullPath := "/" + strings.TrimPrefix(rawPath, "/")
	if request.URL.RawQuery != "" {
		fullPath += "?" + request.URL.RawQuery
	}

	// 1. Signature Verification
	signingKey := baseHandler.configManager.SigningKey()
	signingSalt := baseHandler.configManager.SigningSalt()
	allowInsecure := baseHandler.configManager.Get().AllowInsecure

	if !VerifySignature(signingKey, signingSalt, signature, fullPath, allowInsecure) {
		baseHandler.writeInfoError(responseWriter, request, fullPath, http.StatusForbidden, "Invalid URL signature")
		return
	}

	// 2. Parse Path & Info Options
	optionsString, sourceURL, _, parseErr := ParseURLPath(fullPath)
	if parseErr != nil {
		baseHandler.writeInfoError(responseWriter, request, fullPath, http.StatusBadRequest, parseErr.Error())
		return
	}

	infoOptions := ParseInfoOptions(optionsString)

	// 3. Fetch Source Asset
	bodyReadCloser, _, _, fetchErr := baseHandler.fetcher.Fetch(request.Context(), sourceURL, request)
	if fetchErr != nil {
		if errors.Is(fetchErr, ErrStorageNotFound) {
			baseHandler.writeInfoError(responseWriter, request, sourceURL, http.StatusNotFound, "Image asset not found")
			return
		}
		if errors.Is(fetchErr, ErrStorageAccessDenied) {
			baseHandler.writeInfoError(responseWriter, request, sourceURL, http.StatusForbidden, "Access denied to storage asset")
			return
		}
		baseHandler.writeInfoError(responseWriter, request, sourceURL, http.StatusBadGateway, fetchErr.Error())
		return
	}
	defer func() { _ = bodyReadCloser.Close() }()

	// 4. Introspect Metadata
	info, inspectErr := baseHandler.engine.Inspect(bodyReadCloser, infoOptions)
	if inspectErr != nil {
		baseHandler.writeInfoError(responseWriter, request, sourceURL, http.StatusUnprocessableEntity, inspectErr.Error())
		return
	}

	baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectCompletedEvent(sourceURL, InspectCompletedEventData{
		SourceURL: sourceURL,
		Format:    info.Format,
		ByteSize:  info.Size,
	}))

	log.Debugf("image metadata inspection succeeded for %s: format=%s dimensions=%dx%d", sourceURL, info.Format, info.Width, info.Height)
	core.WriteJSONResponse(responseWriter, http.StatusOK, info)
}

// handleInfoProbe handles POST /v1/image/info inspecting uploaded binary or multipart image data.
func (baseHandler *BaseHandler) handleInfoProbe(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling image info probe request")
	infoOptions := NewDefaultInfoOptions()

	contentType := request.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		log.Trace("handling multipart image info probe request")
		const maxMultipartMemory = 32 << 20 // 32MB
		if parseMultipartErr := request.ParseMultipartForm(maxMultipartMemory); parseMultipartErr != nil {
			baseHandler.writeInfoError(responseWriter, request, "upload", http.StatusBadRequest, "Failed to parse multipart payload")
			return
		}

		file, _, fileErr := request.FormFile("file")
		if fileErr != nil {
			file, _, fileErr = request.FormFile("image")
		}
		if fileErr != nil {
			baseHandler.writeInfoError(responseWriter, request, "upload", http.StatusBadRequest, "No file uploaded in multipart form")
			return
		}
		defer func() { _ = file.Close() }()

		info, inspectErr := baseHandler.engine.Inspect(file, infoOptions)
		if inspectErr != nil {
			baseHandler.writeInfoError(responseWriter, request, "upload", http.StatusUnprocessableEntity, inspectErr.Error())
			return
		}

		baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectCompletedEvent("upload", InspectCompletedEventData{
			SourceURL: "upload",
			Format:    info.Format,
			ByteSize:  info.Size,
		}))

		log.Debugf("multipart image info probe succeeded: format=%s dimensions=%dx%d", info.Format, info.Width, info.Height)
		core.WriteJSONResponse(responseWriter, http.StatusOK, info)
		return
	}

	// Raw binary body inspection
	log.Trace("handling raw binary image info probe request")
	defer func() { _ = request.Body.Close() }()
	info, inspectErr := baseHandler.engine.Inspect(request.Body, infoOptions)
	if inspectErr != nil {
		baseHandler.writeInfoError(responseWriter, request, "upload", http.StatusUnprocessableEntity, inspectErr.Error())
		return
	}

	baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectCompletedEvent("upload", InspectCompletedEventData{
		SourceURL: "upload",
		Format:    info.Format,
		ByteSize:  info.Size,
	}))

	log.Debugf("raw binary image info probe succeeded: format=%s dimensions=%dx%d", info.Format, info.Width, info.Height)
	core.WriteJSONResponse(responseWriter, http.StatusOK, info)
}

func (baseHandler *BaseHandler) writeInfoError(responseWriter http.ResponseWriter, request *http.Request, sourceURL string, statusCode int, message string) {
	log.Debugf("image info error: status=%d message=%s source=%s", statusCode, message, sourceURL)
	baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent(sourceURL, InspectFailedEventData{
		SourceURL:  sourceURL,
		Reason:     message,
		StatusCode: statusCode,
	}))
	core.WriteErrorResponse(responseWriter, request, statusCode, message)
}
