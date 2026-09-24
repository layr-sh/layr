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
		core.RouteTag("Image Data Plane"),
		core.RouteSummary("Transform and optimize an image"),
		core.RouteDescription("Dynamically fetches, crops, resizes, optimizes, and transforms source images according to imgproxy option specifications."),
		core.RouteBinaryResponse(http.StatusOK, "Transformed image binary stream"),
		core.RouteOperationID("image__transform__get"),
		core.RouteSDKGroupName("image", "transform"),
		core.RouteSDKMethodName("get"),
	)

	// 3. Inspect Image Metadata (GET)
	core.GetRoute[GetInfoResponse](router, "/v1/image/info/{signature}/{path...}", service.baseHandler.handleGetInfo,
		core.RouteTag("Image Data Plane"),
		core.RouteSummary("Inspect image metadata by path"),
		core.RouteDescription("Extracts dimensions, format, colorspace, EXIF, and structural metadata from a remote or local image path."),
		core.RouteOperationID("image__info__get"),
		core.RouteSDKGroupName("image", "info"),
		core.RouteSDKMethodName("get"),
	)

	// 4. Inspect Image Metadata (POST binary)
	core.PostRoute[GetInfoResponse, core.Empty](router, "/v1/image/info", service.baseHandler.handleProbeInfo,
		core.RouteTag("Image Data Plane"),
		core.RouteSummary("Inspect image metadata from binary upload"),
		core.RouteDescription("Extracts dimensions, format, colorspace, EXIF, and structural metadata from an uploaded image payload."),
		core.RouteOperationID("image__info__post"),
		core.RouteSDKGroupName("image", "info"),
		core.RouteSDKMethodName("post"),
	)
}

func (service *Service) registerControlPlaneRoutes(router *core.Router) {
	// 1. Runtime Configuration
	core.GetRoute[Config](router, "/v1/_/image/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("Image Control Plane"),
		core.RouteSummary("Get runtime image configuration"),
		core.RouteDescription("Returns the dynamic operational configuration settings for the image service."),
		core.RouteOperationID("image__config__get"),
		core.RouteSDKGroupName("image", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/image/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Image Control Plane"),
		core.RouteSummary("Update runtime image configuration"),
		core.RouteDescription("Modifies dynamic operational configuration settings such as quality, resolution limits, and domain allowlists."),
		core.RouteOperationID("image__config__update"),
		core.RouteSDKGroupName("image", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 2. Transformation Presets
	core.GetRoute[ListPresetsResponse](router, "/v1/_/image/presets", service.controlPlaneHandler.handleListPresets,
		core.RouteTag("Image Control Plane"),
		core.RouteSummary("List image presets"),
		core.RouteDescription("Returns all configured transformation presets."),
		core.RouteOperationID("image__presets__list"),
		core.RouteSDKGroupName("image", "presets"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[Preset, CreatePresetInput](router, "/v1/_/image/presets", service.controlPlaneHandler.handleCreatePreset,
		core.RouteTag("Image Control Plane"),
		core.RouteSummary("Create an image preset"),
		core.RouteDescription("Creates a new named transformation preset with processing option definitions."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("image__presets__create"),
		core.RouteSDKGroupName("image", "presets"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[Preset](router, "/v1/_/image/presets/{id}", service.controlPlaneHandler.handleGetPreset,
		core.RouteTag("Image Control Plane"),
		core.RouteSummary("Get image preset by ID"),
		core.RouteDescription("Returns a single transformation preset definition."),
		core.RouteOperationID("image__presets__get"),
		core.RouteSDKGroupName("image", "presets"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Preset, UpdatePresetInput](router, "/v1/_/image/presets/{id}", service.controlPlaneHandler.handleUpdatePreset,
		core.RouteTag("Image Control Plane"),
		core.RouteSummary("Update an image preset"),
		core.RouteDescription("Modifies an existing transformation preset definition."),
		core.RouteOperationID("image__presets__update"),
		core.RouteSDKGroupName("image", "presets"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/image/presets/{id}", service.controlPlaneHandler.handleDeletePreset,
		core.RouteTag("Image Control Plane"),
		core.RouteSummary("Delete an image preset"),
		core.RouteDescription("Deletes a transformation preset by ID."),
		core.RouteNoContentResponse("Preset deleted"),
		core.RouteOperationID("image__presets__delete"),
		core.RouteSDKGroupName("image", "presets"),
		core.RouteSDKMethodName("delete"),
	)

	// 3. Cryptographic URL Signing
	core.PostRoute[SignURLResponse, SignURLInput](router, "/v1/_/image/sign", service.controlPlaneHandler.handleSignURL,
		core.RouteTag("Image Control Plane"),
		core.RouteSummary("Sign an image transformation path"),
		core.RouteDescription("Generates a cryptographically signed HMAC-SHA256 URL for an image transformation path."),
		core.RouteOperationID("image__sign__create"),
		core.RouteSDKGroupName("image", "sign"),
		core.RouteSDKMethodName("create"),
	)

	// 4. Performance & Telemetry Statistics
	core.GetRoute[GetStatsResponse](router, "/v1/_/image/stats", service.controlPlaneHandler.handleGetStats,
		core.RouteTag("Image Control Plane"),
		core.RouteSummary("Get image service telemetry statistics"),
		core.RouteDescription("Returns real-time cache hit ratios, throughput metrics, and memory utilization statistics."),
		core.RouteOperationID("image__stats__get"),
		core.RouteSDKGroupName("image", "stats"),
		core.RouteSDKMethodName("get"),
	)
}
