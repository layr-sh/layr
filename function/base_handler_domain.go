// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"uuid"

	"github.com/jackc/pgx/v5"
)

type domainRouteCandidate struct {
	endpointID uuid.UUID
	pathPrefix string
}

func (baseHandler *BaseHandler) resolveEndpointByCustomDomain(ctx context.Context, host string, rawPath string) (*Endpoint, *Deployment, error) {
	normalizedHost, validateErr := NormalizeAndValidateDomain(host)
	if validateErr != nil {
		return nil, nil, fmt.Errorf("invalid host for custom domain lookup: %w", validateErr)
	}

	const queryDomainRoutesSQL = `
		SELECT d.id, r.endpoint_id, r.path_prefix
		FROM function.custom_domains d
		LEFT JOIN function.custom_domain_routes r ON r.custom_domain_id = d.id
		WHERE d.domain = $1 AND d.status = 'active'
		ORDER BY length(r.path_prefix) DESC NULLS LAST;
	`
	rows, queryRoutesErr := baseHandler.kernel.DB().Query(ctx, queryDomainRoutesSQL, normalizedHost)
	if queryRoutesErr != nil {
		return nil, nil, fmt.Errorf("failed to fetch domain routes: %w", queryRoutesErr)
	}
	defer rows.Close()

	var domainFound bool
	var candidates []domainRouteCandidate
	for rows.Next() {
		domainFound = true
		var domainID uuid.UUID
		var endpointID *uuid.UUID
		var pathPrefix *string
		_ = rows.Scan(&domainID, &endpointID, &pathPrefix)
		if endpointID != nil && pathPrefix != nil {
			candidates = append(candidates, domainRouteCandidate{
				endpointID: *endpointID,
				pathPrefix: *pathPrefix,
			})
		}
	}

	if !domainFound {
		return nil, nil, ErrCustomDomainNotFound
	}

	cleanedPath := strings.TrimSpace(rawPath)
	if cleanedPath == "" || !strings.HasPrefix(cleanedPath, "/") {
		cleanedPath = "/" + cleanedPath
	}
	cleanedPath = path.Clean(cleanedPath)

	var matchedEndpointID *uuid.UUID
	for _, candidate := range candidates {
		candidatePrefix := candidate.pathPrefix
		if candidatePrefix == "/" || cleanedPath == candidatePrefix || strings.HasPrefix(cleanedPath, candidatePrefix+"/") {
			matchedID := candidate.endpointID
			matchedEndpointID = &matchedID
			break
		}
	}

	if matchedEndpointID == nil {
		return nil, nil, ErrCustomDomainNotFound
	}

	const queryEndpointSQL = `
		SELECT id, name, description, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, active_deployment_id, created_at, updated_at
		FROM function.endpoints
		WHERE id = $1;
	`
	var targetEndpoint Endpoint
	endpointErr := baseHandler.kernel.DB().QueryRow(ctx, queryEndpointSQL, *matchedEndpointID).Scan(
		&targetEndpoint.ID,
		&targetEndpoint.Name,
		&targetEndpoint.Description,
		&targetEndpoint.Runtime,
		&targetEndpoint.Entrypoint,
		&targetEndpoint.MemoryLimitMB,
		&targetEndpoint.TimeoutSeconds,
		&targetEndpoint.IsPublic,
		&targetEndpoint.ActiveDeploymentID,
		&targetEndpoint.CreatedAt,
		&targetEndpoint.UpdatedAt,
	)
	if endpointErr != nil {
		if errors.Is(endpointErr, pgx.ErrNoRows) {
			return nil, nil, ErrEndpointNotFound
		}
		return nil, nil, fmt.Errorf("failed to query endpoint: %w", endpointErr)
	}

	if targetEndpoint.ActiveDeploymentID != nil {
		activeDeployment, _ := baseHandler.fetchActiveDeployment(ctx, targetEndpoint.ID)
		return &targetEndpoint, activeDeployment, nil
	}

	return &targetEndpoint, nil, nil
}

// HandleCustomDomain intercepts incoming requests with custom domain hosts and routes them to active endpoints.
// Returns true if the host matched a configured custom domain and was handled, false otherwise.
func (baseHandler *BaseHandler) HandleCustomDomain(responseWriter http.ResponseWriter, request *http.Request) bool {
	host := strings.TrimSpace(request.Host)
	if host == "" {
		return false
	}

	requestCtx := request.Context()
	matchedEndpoint, activeDeployment, resolveErr := baseHandler.resolveEndpointByCustomDomain(requestCtx, host, request.URL.Path)
	if resolveErr != nil {
		return false
	}

	baseHandler.invokeEndpoint(responseWriter, request, matchedEndpoint, activeDeployment)
	return true
}
