// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"

	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"layr.sh/core"
)

const (
	maxDomainLength      = 253
	maxDomainLabelLength = 63
	maxPathPrefixLength  = 255
)

var domainLabelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// NormalizeAndValidateDomain normalizes and validates a domain name.
func NormalizeAndValidateDomain(rawDomain string) (string, error) {
	trimmedDomain := strings.TrimSpace(rawDomain)
	if trimmedDomain == "" {
		return "", errors.New("domain name cannot be empty")
	}

	// Strip port if present
	if strings.Contains(trimmedDomain, ":") {
		host, port, splitErr := net.SplitHostPort(trimmedDomain)
		if splitErr != nil {
			return "", fmt.Errorf("invalid domain host port format: %w", splitErr)
		}
		if _, parsePortErr := strconv.Atoi(port); parsePortErr != nil {
			return "", fmt.Errorf("invalid domain port number: %w", parsePortErr)
		}
		trimmedDomain = host
	}

	normalized := strings.ToLower(trimmedDomain)
	if len(normalized) > maxDomainLength {
		return "", errors.New("domain name exceeds maximum length of 253 characters")
	}

	labels := strings.Split(normalized, ".")
	if len(labels) < 2 {
		return "", errors.New("domain must contain at least one dot separating subdomains")
	}

	for _, label := range labels {
		if label == "" {
			return "", errors.New("domain label cannot be empty")
		}
		if len(label) > maxDomainLabelLength {
			return "", errors.New("domain label exceeds maximum length of 63 characters")
		}
		if !domainLabelPattern.MatchString(label) {
			return "", fmt.Errorf("domain label %q contains invalid characters or malformed hyphens", label)
		}
	}

	return normalized, nil
}

// NormalizeAndValidatePathPrefix normalizes and validates a custom domain routing path prefix.
func NormalizeAndValidatePathPrefix(rawPrefix string) (string, error) {
	trimmed := strings.TrimSpace(rawPrefix)
	if trimmed == "" || trimmed == "/" {
		return "/", nil
	}

	if strings.Contains(trimmed, "?") || strings.Contains(trimmed, "#") {
		return "", errors.New("path prefix cannot contain query parameters or fragment identifiers")
	}

	if strings.ContainsAny(trimmed, " \t\r\n") {
		return "", errors.New("path prefix cannot contain whitespace characters")
	}

	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}

	cleanPath := path.Clean(trimmed)
	if len(cleanPath) > maxPathPrefixLength {
		return "", errors.New("path prefix exceeds maximum length of 255 characters")
	}

	return cleanPath, nil
}

// fetchCustomDomainByID retrieves a custom domain by its UUID primary key.
func (controlPlaneHandler *ControlPlaneHandler) fetchCustomDomainByID(ctx context.Context, domainID uuid.UUID) (*CustomDomain, error) {
	const querySQL = `
		SELECT id, domain, status, created_at, updated_at
		FROM function.custom_domains
		WHERE id = $1;
	`

	var customDomain CustomDomain
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(ctx, querySQL, domainID).Scan(
		&customDomain.ID,
		&customDomain.Domain,
		&customDomain.Status,
		&customDomain.CreatedAt,
		&customDomain.UpdatedAt,
	)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return nil, ErrCustomDomainNotFound
		}
		return nil, fmt.Errorf("failed to get custom domain: %w", scanErr)
	}

	return &customDomain, nil
}

// handleCreateCustomDomain handles POST /v1/_/function/domains registering a standalone custom domain.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateCustomDomain(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create custom domain request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	var createCustomDomainInput CreateCustomDomainInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createCustomDomainInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	normalizedDomain, validateErr := NormalizeAndValidateDomain(createCustomDomainInput.Domain)
	if validateErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("domain validation failed: %s", validateErr.Error()))
		return
	}

	const insertSQL = `
		INSERT INTO function.custom_domains (id, domain, status, created_at, updated_at)
		VALUES (uuidv7(), $1, 'active', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, created_at, updated_at;
	`

	var customDomain CustomDomain
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), insertSQL, normalizedDomain).Scan(
		&customDomain.ID,
		&customDomain.Domain,
		&customDomain.Status,
		&customDomain.CreatedAt,
		&customDomain.UpdatedAt,
	)
	if scanErr != nil {
		var pgError *pgconn.PgError
		if errors.As(scanErr, &pgError) && pgError.Code == "23505" {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, ErrCustomDomainConflict.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewCustomDomainCreatedEvent(customDomain.ID.String(), CustomDomainCreatedEventData(customDomain)),
	)

	log.Debugf("custom domain %s successfully created", customDomain.ID)
	core.WriteJSONResponse(responseWriter, http.StatusCreated, customDomain)
}

// handleListCustomDomains handles GET /v1/_/function/domains listing all registered custom domains.
func (controlPlaneHandler *ControlPlaneHandler) handleListCustomDomains(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list custom domains request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointRead) {
		return
	}

	const querySQL = `
		SELECT id, domain, status, created_at, updated_at
		FROM function.custom_domains
		ORDER BY created_at ASC;
	`

	rows, queryErr := controlPlaneHandler.kernel.DB().Query(request.Context(), querySQL)
	if queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}
	defer rows.Close()

	customDomains := make([]CustomDomain, 0)
	for rows.Next() {
		var customDomain CustomDomain
		_ = rows.Scan(
			&customDomain.ID,
			&customDomain.Domain,
			&customDomain.Status,
			&customDomain.CreatedAt,
			&customDomain.UpdatedAt,
		)
		customDomains = append(customDomains, customDomain)
	}

	log.Debugf("retrieved %d custom domain(s)", len(customDomains))
	core.WriteJSONResponse(responseWriter, http.StatusOK, ListCustomDomainsResponse{
		Data:  customDomains,
		Total: len(customDomains),
	})
}

// handleDeleteCustomDomain handles DELETE /v1/_/function/domains/{domain_id} removing a standalone custom domain.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteCustomDomain(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling delete custom domain request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	domainIDString := request.PathValue("domain_id")
	domainID, parseDomainErr := uuid.Parse(domainIDString)
	if parseDomainErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid custom domain ID")
		return
	}

	var customDomain CustomDomain
	const deleteSQL = `
		DELETE FROM function.custom_domains
		WHERE id = $1
		RETURNING id, domain, status, created_at, updated_at;
	`
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), deleteSQL, domainID).Scan(
		&customDomain.ID,
		&customDomain.Domain,
		&customDomain.Status,
		&customDomain.CreatedAt,
		&customDomain.UpdatedAt,
	)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, ErrCustomDomainNotFound.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewCustomDomainDeletedEvent(domainID.String(), CustomDomainDeletedEventData(customDomain)),
	)

	log.Debugf("custom domain %s successfully deleted", domainID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

// handleCreateCustomDomainRoute handles POST /v1/_/function/endpoints/{endpoint_id}/domains attaching an endpoint to a domain route.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateCustomDomainRoute(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create endpoint domain route request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	endpointIDString := request.PathValue("endpoint_id")
	endpointID, parseEndpointErr := uuid.Parse(endpointIDString)
	if parseEndpointErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	var createCustomDomainRouteInput CreateCustomDomainRouteInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createCustomDomainRouteInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	if createCustomDomainRouteInput.CustomDomainID == uuid.Nil() {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "custom_domain_id is required")
		return
	}

	if _, fetchEndpointErr := controlPlaneHandler.fetchEndpointByID(request.Context(), endpointID); fetchEndpointErr != nil {
		if errors.Is(fetchEndpointErr, ErrEndpointNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fetchEndpointErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchEndpointErr.Error())
		return
	}

	customDomain, fetchDomainErr := controlPlaneHandler.fetchCustomDomainByID(request.Context(), createCustomDomainRouteInput.CustomDomainID)
	if fetchDomainErr != nil {
		if errors.Is(fetchDomainErr, ErrCustomDomainNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fetchDomainErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchDomainErr.Error())
		return
	}

	rawPrefix := "/"
	if createCustomDomainRouteInput.PathPrefix != nil {
		rawPrefix = *createCustomDomainRouteInput.PathPrefix
	}

	normalizedPrefix, validatePrefixErr := NormalizeAndValidatePathPrefix(rawPrefix)
	if validatePrefixErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("path prefix validation failed: %s", validatePrefixErr.Error()))
		return
	}

	const insertSQL = `
		INSERT INTO function.custom_domain_routes (id, custom_domain_id, endpoint_id, path_prefix, created_at, updated_at)
		VALUES (uuidv7(), $1, $2, $3, clock_timestamp(), clock_timestamp())
		RETURNING id, custom_domain_id, endpoint_id, path_prefix, created_at, updated_at;
	`

	var customDomainRoute CustomDomainRoute
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(
		request.Context(),
		insertSQL,
		createCustomDomainRouteInput.CustomDomainID,
		endpointID,
		normalizedPrefix,
	).Scan(
		&customDomainRoute.ID,
		&customDomainRoute.CustomDomainID,
		&customDomainRoute.EndpointID,
		&customDomainRoute.PathPrefix,
		&customDomainRoute.CreatedAt,
		&customDomainRoute.UpdatedAt,
	)
	if scanErr != nil {
		var pgError *pgconn.PgError
		if errors.As(scanErr, &pgError) && pgError.Code == "23505" {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, ErrCustomDomainRouteConflict.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	customDomainRoute.Domain = customDomain.Domain

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewCustomDomainRouteCreatedEvent(customDomainRoute.ID.String(), CustomDomainRouteCreatedEventData(customDomainRoute)),
	)

	log.Debugf("custom domain route %s successfully created", customDomainRoute.ID)
	core.WriteJSONResponse(responseWriter, http.StatusCreated, customDomainRoute)
}

// handleListCustomDomainRoutes handles GET /v1/_/function/endpoints/{endpoint_id}/domains listing all route mappings for an endpoint.
func (controlPlaneHandler *ControlPlaneHandler) handleListCustomDomainRoutes(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list endpoint domain routes request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointRead) {
		return
	}

	endpointIDString := request.PathValue("endpoint_id")
	endpointID, parseEndpointErr := uuid.Parse(endpointIDString)
	if parseEndpointErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	const querySQL = `
		SELECT r.id, r.custom_domain_id, d.domain, r.endpoint_id, r.path_prefix, r.created_at, r.updated_at
		FROM function.custom_domain_routes r
		JOIN function.custom_domains d ON d.id = r.custom_domain_id
		WHERE r.endpoint_id = $1
		ORDER BY r.created_at ASC;
	`

	rows, queryErr := controlPlaneHandler.kernel.DB().Query(request.Context(), querySQL, endpointID)
	if queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}
	defer rows.Close()

	routes := make([]CustomDomainRoute, 0)
	for rows.Next() {
		var customDomainRoute CustomDomainRoute
		_ = rows.Scan(
			&customDomainRoute.ID,
			&customDomainRoute.CustomDomainID,
			&customDomainRoute.Domain,
			&customDomainRoute.EndpointID,
			&customDomainRoute.PathPrefix,
			&customDomainRoute.CreatedAt,
			&customDomainRoute.UpdatedAt,
		)
		routes = append(routes, customDomainRoute)
	}

	log.Debugf("retrieved %d custom domain route(s)", len(routes))
	core.WriteJSONResponse(responseWriter, http.StatusOK, ListCustomDomainRoutesResponse{
		Data:  routes,
		Total: len(routes),
	})
}

// handleDeleteCustomDomainRoute handles DELETE /v1/_/function/endpoints/{endpoint_id}/domains/{route_id} unbinding an endpoint from a domain route.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteCustomDomainRoute(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling delete endpoint domain route request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	endpointIDString := request.PathValue("endpoint_id")
	endpointID, parseEndpointErr := uuid.Parse(endpointIDString)
	if parseEndpointErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	routeIDString := request.PathValue("route_id")
	routeID, parseRouteErr := uuid.Parse(routeIDString)
	if parseRouteErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid custom domain route ID")
		return
	}

	var customDomainRoute CustomDomainRoute
	const deleteSQL = `
		DELETE FROM function.custom_domain_routes
		WHERE id = $1 AND endpoint_id = $2
		RETURNING id, custom_domain_id, endpoint_id, path_prefix, created_at, updated_at;
	`
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), deleteSQL, routeID, endpointID).Scan(
		&customDomainRoute.ID,
		&customDomainRoute.CustomDomainID,
		&customDomainRoute.EndpointID,
		&customDomainRoute.PathPrefix,
		&customDomainRoute.CreatedAt,
		&customDomainRoute.UpdatedAt,
	)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, ErrCustomDomainRouteNotFound.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewCustomDomainRouteDeletedEvent(routeID.String(), CustomDomainRouteDeletedEventData(customDomainRoute)),
	)

	log.Debugf("custom domain route %s successfully deleted", routeID)
	responseWriter.WriteHeader(http.StatusNoContent)
}
