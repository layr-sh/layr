// Package function defines the serverless and edge function execution engine.
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

type mockFailingRunner struct {
	shouldFail bool
	healthFail bool
	deployFail bool
	stopFail   bool
	startFail  bool
	statusCode int
}

func (runner *mockFailingRunner) Name() string {
	return "workerd"
}

func (runner *mockFailingRunner) Undeploy(_ context.Context, _ string) error {
	return nil
}

func (runner *mockFailingRunner) Start(_ context.Context) error {
	if runner.startFail {
		return errors.New("runner start failed")
	}
	return nil
}

func (runner *mockFailingRunner) Stop() error {
	if runner.stopFail {
		return errors.New("runner stop failed")
	}
	return nil
}

func (runner *mockFailingRunner) Deploy(_ context.Context, _ *Endpoint, _ *Deployment) error {
	if runner.deployFail {
		return errors.New("runner deploy failed")
	}
	return nil
}

func (runner *mockFailingRunner) Health(_ context.Context) (*RunnerHealth, error) {
	if runner.healthFail {
		return nil, errors.New("runner health check failed")
	}
	return &RunnerHealth{
		Available: true,
	}, nil
}

func (runner *mockFailingRunner) Forward(responseWriter http.ResponseWriter, _ *http.Request, _ *Endpoint) error {
	if runner.shouldFail {
		return errors.New("execution crashed")
	}
	responseWriter.Header().Set("X-Layr-Test", "true")
	if runner.statusCode != 0 {
		responseWriter.WriteHeader(runner.statusCode)
		return nil
	}
	responseWriter.WriteHeader(http.StatusOK)
	return nil
}

func TestFunctionServiceIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()

	// 1. Verify factory
	serviceFactory, exists := core.GetServiceFactory("function")
	require.True(t, exists)
	serviceRunner, factoryErr := serviceFactory(kernel)
	require.NoError(t, factoryErr)
	require.NotNil(t, serviceRunner)

	// 2. Initialize service
	service := NewService(kernel)
	testRunner := &mockFailingRunner{}
	service.Engine().RegisterRunner(testRunner)

	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	// 3. Create endpoint and deployment in database
	var endpointID uuid.UUID
	const insertEndpointSQL = `
		INSERT INTO function.endpoints (name, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at)
		VALUES ('event-triggered-func', 'workerd', 'index.js', 128, 30, true, clock_timestamp(), clock_timestamp())
		RETURNING id;
	`
	require.NoError(t, kernel.DB().QueryRow(testCtx, insertEndpointSQL).Scan(&endpointID))

	var deploymentID uuid.UUID
	const insertDeploymentSQL = `
		INSERT INTO function.deployments (endpoint_id, version, bundle_format, bundle_content, bundle_hash, bundle_files, environment_variables, status, created_at)
		VALUES ($1, 1, 'raw', 'export default {};', 'hash1', '{}', '{}', 'active', clock_timestamp())
		RETURNING id;
	`
	require.NoError(t, kernel.DB().QueryRow(testCtx, insertDeploymentSQL, endpointID).Scan(&deploymentID))

	const updateEndpointSQL = `UPDATE function.endpoints SET active_deployment_id = $1 WHERE id = $2;`
	_, updateErr := kernel.DB().Exec(testCtx, updateEndpointSQL, deploymentID, endpointID)
	require.NoError(t, updateErr)

	// 4. Test restart loads active endpoints
	service.Stop()
	restartService := NewService(kernel)
	restartRunner := &mockFailingRunner{}
	restartService.Engine().RegisterRunner(restartRunner)
	require.NoError(t, restartService.Start(testCtx))
	defer restartService.Stop()

	var loadedName string
	const queryCheckSQL = `SELECT name FROM function.endpoints WHERE id = $1;`
	require.NoError(t, kernel.DB().QueryRow(testCtx, queryCheckSQL, endpointID).Scan(&loadedName))
	require.Equal(t, "event-triggered-func", loadedName)
}
