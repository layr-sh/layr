package image

import (
	"errors"
	"net/http"
	"strings"

	"layr.sh/core"
)

// handleGetInfo handles GET /v1/image/info/{signature}/{path...}.
func (baseHandler *BaseHandler) handleGetInfo(responseWriter http.ResponseWriter, request *http.Request) {
	signature := request.PathValue("signature")
	rawPath := request.PathValue("path")
	log.Tracef("handling image info request: path=%s", rawPath)
	if signature == "" || rawPath == "" {
		baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent(rawPath, InspectFailedEventData{
			SourceURL:  rawPath,
			Reason:     "Signature and info path are required",
			StatusCode: http.StatusBadRequest,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Signature and info path are required")
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
		baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent(fullPath, InspectFailedEventData{
			SourceURL:  fullPath,
			Reason:     "Invalid URL signature",
			StatusCode: http.StatusForbidden,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Invalid URL signature")
		return
	}

	// 2. Parse Path & Info Options
	optionsString, sourceURL, _, parseErr := ParseURLPath(fullPath)
	if parseErr != nil {
		baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent(fullPath, InspectFailedEventData{
			SourceURL:  fullPath,
			Reason:     parseErr.Error(),
			StatusCode: http.StatusBadRequest,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, parseErr.Error())
		return
	}

	infoOptions := ParseInfoOptions(optionsString)

	// 3. Fetch Source Asset
	bodyReadCloser, _, _, fetchErr := baseHandler.fetcher.Fetch(request.Context(), sourceURL, request)
	if fetchErr != nil {
		if errors.Is(fetchErr, ErrStorageNotFound) {
			baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent(sourceURL, InspectFailedEventData{
				SourceURL:  sourceURL,
				Reason:     "Image asset not found",
				StatusCode: http.StatusNotFound,
			}))
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Image asset not found")
			return
		}
		if errors.Is(fetchErr, ErrStorageAccessDenied) {
			baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent(sourceURL, InspectFailedEventData{
				SourceURL:  sourceURL,
				Reason:     "Access denied",
				StatusCode: http.StatusForbidden,
			}))
			core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied")
			return
		}
		baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent(sourceURL, InspectFailedEventData{
			SourceURL:  sourceURL,
			Reason:     fetchErr.Error(),
			StatusCode: http.StatusBadGateway,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusBadGateway, fetchErr.Error())
		return
	}
	defer func() { _ = bodyReadCloser.Close() }()

	// 4. Introspect Metadata
	getInfoResponse, inspectErr := baseHandler.engine.Inspect(bodyReadCloser, infoOptions)
	if inspectErr != nil {
		baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent(sourceURL, InspectFailedEventData{
			SourceURL:  sourceURL,
			Reason:     inspectErr.Error(),
			StatusCode: http.StatusUnprocessableEntity,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, inspectErr.Error())
		return
	}

	baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectCompletedEvent(sourceURL, InspectCompletedEventData{
		SourceURL: sourceURL,
		Format:    getInfoResponse.Format,
		ByteSize:  getInfoResponse.Size,
	}))

	log.Debugf("image metadata inspection succeeded for %s: format=%s dimensions=%dx%d", sourceURL, getInfoResponse.Format, getInfoResponse.Width, getInfoResponse.Height)
	core.WriteJSONResponse(responseWriter, http.StatusOK, getInfoResponse)
}

// handleProbeInfo handles POST /v1/image/info inspecting uploaded binary or multipart image data.
func (baseHandler *BaseHandler) handleProbeInfo(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling image info probe request")
	infoOptions := NewDefaultInfoOptions()

	contentType := request.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		log.Trace("handling multipart image info probe request")
		const maxMultipartMemory = 32 << 20 // 32MB
		if parseMultipartErr := request.ParseMultipartForm(maxMultipartMemory); parseMultipartErr != nil {
			baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent("upload", InspectFailedEventData{
				SourceURL:  "upload",
				Reason:     "Failed to parse multipart payload",
				StatusCode: http.StatusBadRequest,
			}))
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Failed to parse multipart payload")
			return
		}

		file, _, fileErr := request.FormFile("file")
		if fileErr != nil {
			file, _, fileErr = request.FormFile("image")
		}
		if fileErr != nil {
			baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent("upload", InspectFailedEventData{
				SourceURL:  "upload",
				Reason:     "No file uploaded in multipart form",
				StatusCode: http.StatusBadRequest,
			}))
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "No file uploaded in multipart form")
			return
		}
		defer func() { _ = file.Close() }()

		getInfoResponse, inspectErr := baseHandler.engine.Inspect(file, infoOptions)
		if inspectErr != nil {
			baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent("upload", InspectFailedEventData{
				SourceURL:  "upload",
				Reason:     inspectErr.Error(),
				StatusCode: http.StatusUnprocessableEntity,
			}))
			core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, inspectErr.Error())
			return
		}

		baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectCompletedEvent("upload", InspectCompletedEventData{
			SourceURL: "upload",
			Format:    getInfoResponse.Format,
			ByteSize:  getInfoResponse.Size,
		}))

		log.Debugf("multipart image info probe succeeded: format=%s dimensions=%dx%d", getInfoResponse.Format, getInfoResponse.Width, getInfoResponse.Height)
		core.WriteJSONResponse(responseWriter, http.StatusOK, getInfoResponse)
		return
	}

	// Raw binary body inspection
	log.Trace("handling raw binary image info probe request")
	defer func() { _ = request.Body.Close() }()
	getInfoResponse, inspectErr := baseHandler.engine.Inspect(request.Body, infoOptions)
	if inspectErr != nil {
		baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectFailedEvent("upload", InspectFailedEventData{
			SourceURL:  "upload",
			Reason:     inspectErr.Error(),
			StatusCode: http.StatusUnprocessableEntity,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, inspectErr.Error())
		return
	}

	baseHandler.kernel.EventBus().Publish(request.Context(), NewInspectCompletedEvent("upload", InspectCompletedEventData{
		SourceURL: "upload",
		Format:    getInfoResponse.Format,
		ByteSize:  getInfoResponse.Size,
	}))

	log.Debugf("raw binary image info probe succeeded: format=%s dimensions=%dx%d", getInfoResponse.Format, getInfoResponse.Width, getInfoResponse.Height)
	core.WriteJSONResponse(responseWriter, http.StatusOK, getInfoResponse)
}
