package function

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
	"uuid"
)

func insertTestEndpoint(ctx context.Context, t *testing.T, kernel *core.Kernel, name string, isPublic bool) Endpoint {
	t.Helper()
	const querySQL = `
		INSERT INTO function.endpoints (name, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at)
		VALUES ($1, 'workerd', 'index.js', 128, 30, $2, clock_timestamp(), clock_timestamp())
		RETURNING id, name, description, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, active_deployment_id, created_at, updated_at;
	`
	var storedEndpoint Endpoint
	err := kernel.DB().QueryRow(ctx, querySQL, name, isPublic).Scan(
		&storedEndpoint.ID,
		&storedEndpoint.Name,
		&storedEndpoint.Description,
		&storedEndpoint.Runtime,
		&storedEndpoint.Entrypoint,
		&storedEndpoint.MemoryLimitMB,
		&storedEndpoint.TimeoutSeconds,
		&storedEndpoint.IsPublic,
		&storedEndpoint.ActiveDeploymentID,
		&storedEndpoint.CreatedAt,
		&storedEndpoint.UpdatedAt,
	)
	require.NoError(t, err)
	return storedEndpoint
}

func deployTestEndpoint(ctx context.Context, t *testing.T, service *Service, endpoint *Endpoint, bundleContent string) {
	t.Helper()
	const insertSQL = `
		INSERT INTO function.deployments (endpoint_id, version, bundle_format, bundle_content, bundle_hash, bundle_files, environment_variables, workerd_runtime_config, status, created_at)
		VALUES ($1, 1, 'raw', $2, 'hash1', '{"index.js":"export default {};"}', '{"KEY":"VAL"}', '{"compatibility_date":"2026-08-04"}', 'active', clock_timestamp())
		RETURNING id, endpoint_id, version, bundle_format, bundle_hash, bundle_files, environment_variables, status, created_at;
	`
	var deployment Deployment
	var rawFilesJSON []byte
	var rawEnvironmentJSON []byte
	err := service.Kernel().DB().QueryRow(ctx, insertSQL, endpoint.ID, bundleContent).Scan(
		&deployment.ID,
		&deployment.EndpointID,
		&deployment.Version,
		&deployment.BundleFormat,
		&deployment.BundleHash,
		&rawFilesJSON,
		&rawEnvironmentJSON,
		&deployment.Status,
		&deployment.CreatedAt,
	)
	require.NoError(t, err)
	deployment.BundleContent = []byte(bundleContent)

	const updateEndpointSQL = `UPDATE function.endpoints SET active_deployment_id = $1 WHERE id = $2;`
	_, updateErr := service.Kernel().DB().Exec(ctx, updateEndpointSQL, deployment.ID, endpoint.ID)
	require.NoError(t, updateErr)

	endpoint.ActiveDeploymentID = &deployment.ID
	require.NoError(t, service.Engine().Deploy(ctx, endpoint, &deployment))
}

func insertTestCustomDomain(ctx context.Context, t *testing.T, kernel *core.Kernel, domain string) CustomDomain {
	t.Helper()
	const insertSQL = `
		INSERT INTO function.custom_domains (domain, status, created_at, updated_at)
		VALUES ($1, 'active', clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, created_at, updated_at;
	`
	var customDomain CustomDomain
	err := kernel.DB().QueryRow(ctx, insertSQL, domain).Scan(
		&customDomain.ID,
		&customDomain.Domain,
		&customDomain.Status,
		&customDomain.CreatedAt,
		&customDomain.UpdatedAt,
	)
	require.NoError(t, err)
	return customDomain
}

func bindTestCustomDomainRoute(ctx context.Context, t *testing.T, kernel *core.Kernel, endpointID uuid.UUID, domainID uuid.UUID, pathPrefix string) {
	t.Helper()
	const insertSQL = `
		INSERT INTO function.custom_domain_routes (custom_domain_id, endpoint_id, path_prefix, created_at, updated_at)
		VALUES ($1, $2, $3, clock_timestamp(), clock_timestamp());
	`
	_, err := kernel.DB().Exec(ctx, insertSQL, domainID, endpointID, pathPrefix)
	require.NoError(t, err)
}

type failingResponseWriter struct{}

func (writer *failingResponseWriter) Header() http.Header {
	return make(http.Header)
}

func (writer *failingResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (writer *failingResponseWriter) WriteHeader(int) {}
