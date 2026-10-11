package image

import (
	"net/http"

	"layr.sh/core"
)

// RegisterRoutes registers all Image data plane and control plane routes.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	service.registerBaseRoutes(baseRouter)
	service.registerControlPlaneRoutes(controlPlaneRouter)
}

func (service *Service) registerBaseRoutes(router *core.Router) {
	// 1. Image Transformation Endpoint
	core.GetRoute[core.Empty](router, "/v1/image/{signature}/{path...}", service.baseHandler.handleTransform,
		core.RouteTag("Transform"),
		core.RouteSummary("Transform and optimize an image dynamically"),
		core.RouteDescription("Dynamically fetches, crops, resizes, optimizes, and transforms source images according to imgproxy option specifications. Verifies HMAC-SHA256 signature if enabled, caches transformed result, and streams binary image with appropriate Content-Type header."),
		core.RouteBinaryResponse(http.StatusOK, "Transformed image binary stream"),
		core.RouteQueryParam("token", "Presigned capability HMAC token for protected storage sources"),
		core.RouteQueryParam("expires", "Unix epoch seconds expiration timestamp for presigned capability"),
		core.RouteQueryParam("op", "Presigned operation capability type ('read')"),
		core.RouteOperationID("image__transform__get"),
		core.RouteSDKGroupName("image", "transform"),
		core.RouteSDKMethodName("get"),
	)

	// 3. Inspect Image Metadata (GET)
	core.GetRoute[GetInfoResponse](router, "/v1/image/info/{signature}/{path...}", service.baseHandler.handleGetInfo,
		core.RouteTag("Info"),
		core.RouteSummary("Inspect image metadata by path"),
		core.RouteDescription("Extracts structural metadata (width, height, format, file size, colorspace, EXIF, orientation) from a remote or storage image path using cryptographic signature verification."),
		core.RouteQueryParam("token", "Presigned capability HMAC token for protected storage sources"),
		core.RouteQueryParam("expires", "Unix epoch seconds expiration timestamp for presigned capability"),
		core.RouteQueryParam("op", "Presigned operation capability type ('read')"),
		core.RouteOperationID("image__info__get"),
		core.RouteSDKGroupName("image", "info"),
		core.RouteSDKMethodName("get"),
	)

	// 4. Inspect Image Metadata (POST binary)
	core.PostRoute[GetInfoResponse, core.Empty](router, "/v1/image/info", service.baseHandler.handleProbeInfo,
		core.RouteTag("Info"),
		core.RouteSummary("Inspect image metadata from binary upload"),
		core.RouteDescription("Probes and extracts dimensions, format, file size, colorspace, and EXIF metadata from an uploaded image binary payload."),
		core.RouteOperationID("image__info__post"),
		core.RouteSDKGroupName("image", "info"),
		core.RouteSDKMethodName("post"),
	)
}

func (service *Service) registerControlPlaneRoutes(router *core.Router) {
	// 1. Runtime Configuration
	core.GetRoute[Config](router, "/v1/_/image/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Get runtime image service configuration"),
		core.RouteDescription("Returns operational image service settings including compression quality defaults, maximum resolution limits, allowed source domains, and signature enforcement policy. Requires image:config.read scope."),
		core.RouteOperationID("image__config__get"),
		core.RouteSDKGroupName("image", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/image/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Update runtime image service configuration"),
		core.RouteDescription("Modifies operational image service parameters in image.config, updating compression quality, resolution limits, and domain allowlists without restarting the service. Requires image:config.write scope."),
		core.RouteOperationID("image__config__update"),
		core.RouteSDKGroupName("image", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 2. Transformation Presets
	core.GetRoute[ListPresetsResponse](router, "/v1/_/image/presets", service.controlPlaneHandler.handleListPresets,
		core.RouteTag("Presets"),
		core.RouteSummary("List configured image transformation presets"),
		core.RouteDescription("Returns all configured transformation presets from image.presets with preset names, processing option strings, and creation timestamps. Requires image:preset.read scope."),
		core.RouteOperationID("image__presets__list"),
		core.RouteSDKGroupName("image", "presets"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[Preset, CreatePresetInput](router, "/v1/_/image/presets", service.controlPlaneHandler.handleCreatePreset,
		core.RouteTag("Presets"),
		core.RouteSummary("Create a named image transformation preset"),
		core.RouteDescription("Creates a new transformation preset in image.presets defining reusable imgproxy processing options (dimensions, crop mode, quality, format). Emits image.preset.created event. Requires image:preset.write scope."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("image__presets__create"),
		core.RouteSDKGroupName("image", "presets"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[Preset](router, "/v1/_/image/presets/{preset_id}", service.controlPlaneHandler.handleGetPreset,
		core.RouteTag("Presets"),
		core.RouteSummary("Get image transformation preset by ID"),
		core.RouteDescription("Retrieves a single transformation preset definition by ID from image.presets. Requires image:preset.read scope."),
		core.RouteOperationID("image__presets__get"),
		core.RouteSDKGroupName("image", "presets"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Preset, UpdatePresetInput](router, "/v1/_/image/presets/{preset_id}", service.controlPlaneHandler.handleUpdatePreset,
		core.RouteTag("Presets"),
		core.RouteSummary("Update an image transformation preset"),
		core.RouteDescription("Modifies the transformation options or friendly name of an existing preset in image.presets. Emits image.preset.updated event. Requires image:preset.write scope."),
		core.RouteOperationID("image__presets__update"),
		core.RouteSDKGroupName("image", "presets"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/image/presets/{preset_id}", service.controlPlaneHandler.handleDeletePreset,
		core.RouteTag("Presets"),
		core.RouteSummary("Delete an image transformation preset"),
		core.RouteDescription("Deletes a transformation preset by ID from image.presets. Emits image.preset.deleted event. Requires image:preset.write scope."),
		core.RouteNoContentResponse("Preset deleted"),
		core.RouteOperationID("image__presets__delete"),
		core.RouteSDKGroupName("image", "presets"),
		core.RouteSDKMethodName("delete"),
	)

	// 3. Cryptographic URL Signing
	core.PostRoute[SignURLResponse, SignURLInput](router, "/v1/_/image/url/sign", service.controlPlaneHandler.handleSignURL,
		core.RouteTag("URLs"),
		core.RouteSummary("Generate a cryptographically signed image URL"),
		core.RouteDescription("Generates a cryptographically signed HMAC-SHA256 signature and URL path for an image transformation request using the master image key. Requires image:sign.write scope."),
		core.RouteOperationID("image__url__sign"),
		core.RouteSDKGroupName("image", "url"),
		core.RouteSDKMethodName("sign"),
	)

	// 4. Performance & Telemetry Statistics
	core.GetRoute[GetStatsResponse](router, "/v1/_/image/stats", service.controlPlaneHandler.handleGetStats,
		core.RouteTag("Stats"),
		core.RouteSummary("Get image service telemetry statistics"),
		core.RouteDescription("Returns real-time operational statistics including image cache hit ratios, processing latency, memory usage, and transformation throughput. Requires image:stats.read scope."),
		core.RouteOperationID("image__stats__get"),
		core.RouteSDKGroupName("image", "stats"),
		core.RouteSDKMethodName("get"),
	)
}
