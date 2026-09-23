package image

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"layr.sh/core"
)

// handleTransform handles GET /v1/image/{signature}/{path...}.
func (baseHandler *BaseHandler) handleTransform(responseWriter http.ResponseWriter, request *http.Request) {
	signature := request.PathValue("signature")
	rawPath := request.PathValue("path")
	log.Tracef("handling image transform request: path=%s", rawPath)
	if signature == "" || rawPath == "" {
		baseHandler.writeTransformError(responseWriter, request, rawPath, http.StatusBadRequest, "Signature and image path are required")
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
		baseHandler.writeTransformError(responseWriter, request, fullPath, http.StatusForbidden, "Invalid URL signature")
		return
	}

	// 2. Parse Path & Processing Options
	optionsString, sourceURL, extensionOverride, parseErr := ParseURLPath(fullPath)
	if parseErr != nil {
		baseHandler.writeTransformError(responseWriter, request, fullPath, http.StatusBadRequest, parseErr.Error())
		return
	}

	processingOptions, optionsErr := ParseProcessingOptions(optionsString, baseHandler.presetManager.ResolveOptions)
	if optionsErr != nil {
		baseHandler.writeTransformError(responseWriter, request, sourceURL, http.StatusBadRequest, optionsErr.Error())
		return
	}

	if extensionOverride != "" {
		processingOptions.Format = extensionOverride
	}

	// 3. Cache Lookup
	cacheKey, optionsHash := ComputeCacheKey(sourceURL, processingOptions)
	cachedImage, found := baseHandler.cacheManager.Get(request.Context(), cacheKey)
	if found {
		log.Debugf("image transform cache hit for key=%s", cacheKey)
		if clientETag := request.Header.Get("If-None-Match"); clientETag != "" && clientETag == cachedImage.ETag {
			responseWriter.WriteHeader(http.StatusNotModified)
			return
		}

		baseHandler.kernel.EventBus().Publish(request.Context(), NewTransformedEvent(sourceURL, TransformedEventData{
			SourceURL: sourceURL,
			Format:    strings.TrimPrefix(cachedImage.ContentType, "image/"),
			ByteSize:  int64(len(cachedImage.Data)),
			CacheHit:  true,
		}))

		responseWriter.Header().Set("Content-Type", cachedImage.ContentType)
		responseWriter.Header().Set("Content-Length", strconv.Itoa(len(cachedImage.Data)))
		responseWriter.Header().Set("ETag", cachedImage.ETag)
		responseWriter.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", baseHandler.configManager.Get().CacheTTLSeconds))
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write(cachedImage.Data)
		return
	}

	log.Tracef("image transform cache miss for key=%s, fetching source", cacheKey)

	// 4. Source Asset Fetch
	bodyReadCloser, _, _, fetchErr := baseHandler.fetcher.Fetch(request.Context(), sourceURL, request)
	if fetchErr != nil {
		if processingOptions.FallbackImageURL != "" {
			fallbackReadCloser, _, _, fallbackErr := baseHandler.fetcher.Fetch(request.Context(), processingOptions.FallbackImageURL, request)
			if fallbackErr == nil {
				bodyReadCloser = fallbackReadCloser
				fetchErr = nil
			}
		}
	}

	if fetchErr != nil {
		if errors.Is(fetchErr, ErrStorageNotFound) {
			baseHandler.writeTransformError(responseWriter, request, sourceURL, http.StatusNotFound, "Image asset not found")
			return
		}
		if errors.Is(fetchErr, ErrStorageAccessDenied) {
			baseHandler.writeTransformError(responseWriter, request, sourceURL, http.StatusForbidden, "Access denied to storage asset")
			return
		}
		if errors.Is(fetchErr, ErrSSRFBlocked) || errors.Is(fetchErr, ErrDomainNotAllowed) {
			baseHandler.writeTransformError(responseWriter, request, sourceURL, http.StatusForbidden, fetchErr.Error())
			return
		}
		baseHandler.writeTransformError(responseWriter, request, sourceURL, http.StatusBadGateway, fetchErr.Error())
		return
	}
	defer func() { _ = bodyReadCloser.Close() }()

	// 5. Image Transformation
	outputBytes, outputContentType, transformErr := baseHandler.engine.Transform(bodyReadCloser, processingOptions)
	if transformErr != nil {
		baseHandler.writeTransformError(responseWriter, request, sourceURL, http.StatusUnprocessableEntity, transformErr.Error())
		return
	}

	// 6. Record Cache Entry
	eTag := baseHandler.cacheManager.Set(request.Context(), cacheKey, sourceURL, optionsHash, outputContentType, outputBytes)

	baseHandler.kernel.EventBus().Publish(request.Context(), NewTransformedEvent(sourceURL, TransformedEventData{
		SourceURL: sourceURL,
		Format:    strings.TrimPrefix(outputContentType, "image/"),
		ByteSize:  int64(len(outputBytes)),
		Width:     processingOptions.Width,
		Height:    processingOptions.Height,
		CacheHit:  false,
	}))

	log.Debugf("image transform completed: source=%s format=%s size=%d bytes", sourceURL, outputContentType, len(outputBytes))

	// 7. Write HTTP Streaming Response
	responseWriter.Header().Set("Content-Type", outputContentType)
	responseWriter.Header().Set("Content-Length", strconv.Itoa(len(outputBytes)))
	responseWriter.Header().Set("ETag", eTag)
	responseWriter.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", baseHandler.configManager.Get().CacheTTLSeconds))
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = responseWriter.Write(outputBytes)
}

func (baseHandler *BaseHandler) writeTransformError(responseWriter http.ResponseWriter, request *http.Request, sourceURL string, statusCode int, message string) {
	log.Debugf("image transform error: status=%d message=%s source=%s", statusCode, message, sourceURL)
	baseHandler.kernel.EventBus().Publish(request.Context(), NewTransformFailedEvent(sourceURL, TransformFailedEventData{
		SourceURL:  sourceURL,
		Reason:     message,
		StatusCode: statusCode,
	}))
	core.WriteErrorResponse(responseWriter, request, statusCode, message)
}
