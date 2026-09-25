// Package function defines the serverless and edge function execution engine.
package function

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionControlPlaneHandlerDeploymentsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(testCtx, core.CreateServiceAccountInput{
		Name: "deployments-admin",
		Scopes: []string{
			core.ScopeFunctionEndpointRead,
			core.ScopeFunctionEndpointWrite,
			core.ScopeFunctionEndpointDeploy,
		},
	})
	require.NoError(t, accountErr)
	authHeader := "Bearer " + serviceAccount.SecretKey

	// 1. Create target endpoint
	createEndpointInput := CreateEndpointInput{
		Name: "deploy-integration-fn",
	}
	createJSON, _ := json.Marshal(createEndpointInput)
	createRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader(createJSON))
	createRequest.Header.Set("Authorization", authHeader)
	createResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateEndpoint(createResponseRecorder, createRequest)
	require.Equal(t, http.StatusCreated, createResponseRecorder.Code)

	var targetEndpoint Endpoint
	require.NoError(t, json.Unmarshal(createResponseRecorder.Body.Bytes(), &targetEndpoint))

	t.Run("deployment lifecycle and rollbacks", func(t *testing.T) {
		// 1. Deploy version 1 (raw string)
		deployV1CreateDeploymentInput := CreateDeploymentInput{
			BundleContent: "export default { version: 1 };",
		}
		deployV1JSON, _ := json.Marshal(deployV1CreateDeploymentInput)
		deployV1Request := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/deploy", bytes.NewReader(deployV1JSON))
		deployV1Request.SetPathValue("endpoint_id", targetEndpoint.ID.String())
		deployV1Request.Header.Set("Authorization", authHeader)
		deployV1ResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(deployV1ResponseRecorder, deployV1Request)
		require.Equal(t, http.StatusCreated, deployV1ResponseRecorder.Code)

		var firstDeployment Deployment
		require.NoError(t, json.Unmarshal(deployV1ResponseRecorder.Body.Bytes(), &firstDeployment))
		require.Equal(t, 1, firstDeployment.Version)

		// 2. Deploy version 2 (multi-file bundle)
		deployV2CreateDeploymentInput := CreateDeploymentInput{
			BundleFiles: map[string]string{
				"index.js":  "import helper from './helper.js'; export default {};",
				"helper.js": "export default 'ok';",
			},
		}
		deployV2JSON, _ := json.Marshal(deployV2CreateDeploymentInput)
		deployV2Request := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/deploy", bytes.NewReader(deployV2JSON))
		deployV2Request.SetPathValue("endpoint_id", targetEndpoint.ID.String())
		deployV2Request.Header.Set("Authorization", authHeader)
		deployV2ResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(deployV2ResponseRecorder, deployV2Request)
		require.Equal(t, http.StatusCreated, deployV2ResponseRecorder.Code)

		var secondDeployment Deployment
		require.NoError(t, json.Unmarshal(deployV2ResponseRecorder.Body.Bytes(), &secondDeployment))
		require.Equal(t, 2, secondDeployment.Version)
		require.Equal(t, "tar", secondDeployment.BundleFormat)

		// 3. List deployments
		listRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/deployments", nil)
		listRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
		listRequest.Header.Set("Authorization", authHeader)
		listResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListDeployments(listResponseRecorder, listRequest)
		require.Equal(t, http.StatusOK, listResponseRecorder.Code)

		var listDeploymentsResponse ListDeploymentsResponse
		require.NoError(t, json.Unmarshal(listResponseRecorder.Body.Bytes(), &listDeploymentsResponse))
		require.Equal(t, 2, listDeploymentsResponse.Count)

		// 4. Rollback to version 1
		rollbackDeploymentInput := RollbackDeploymentInput{
			TargetVersion: 1,
		}
		rollbackJSON, _ := json.Marshal(rollbackDeploymentInput)
		rollbackRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/rollback", bytes.NewReader(rollbackJSON))
		rollbackRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
		rollbackRequest.Header.Set("Authorization", authHeader)
		rollbackResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(rollbackResponseRecorder, rollbackRequest)
		require.Equal(t, http.StatusOK, rollbackResponseRecorder.Code)

		var rolledBackDeployment Deployment
		require.NoError(t, json.Unmarshal(rollbackResponseRecorder.Body.Bytes(), &rolledBackDeployment))
		require.Equal(t, 1, rolledBackDeployment.Version)
	})
}

type failingWorkerdRunner struct{}

func (failingRunner *failingWorkerdRunner) Name() string                  { return "workerd" }
func (failingRunner *failingWorkerdRunner) Start(_ context.Context) error { return nil }
func (failingRunner *failingWorkerdRunner) Stop() error                   { return nil }
func (failingRunner *failingWorkerdRunner) Deploy(_ context.Context, _ *Endpoint, _ *Deployment) error {
	return fmt.Errorf("%w: failed to download workerd binary", ErrRunnerDownloadFailed)
}
func (failingRunner *failingWorkerdRunner) Undeploy(_ context.Context, _ string) error { return nil }
func (failingRunner *failingWorkerdRunner) Forward(_ http.ResponseWriter, _ *http.Request, _ *Endpoint) error {
	return nil
}
func (failingRunner *failingWorkerdRunner) Health(_ context.Context) (*RunnerHealth, error) {
	return &RunnerHealth{Available: false}, nil
}

func TestFunctionControlPlaneHandlerDeploymentFailurePostgresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(testCtx, core.CreateServiceAccountInput{
		Name: "deployments-failure-admin",
		Scopes: []string{
			core.ScopeFunctionEndpointRead,
			core.ScopeFunctionEndpointWrite,
			core.ScopeFunctionEndpointDeploy,
		},
	})
	require.NoError(t, accountErr)
	authHeader := "Bearer " + serviceAccount.SecretKey

	t.Run("lazy workerd download failure on first deployment", func(t *testing.T) {
		testConfigManager := NewConfigManager(nil)
		failingEngine := NewEngine()
		failingEngine.RegisterRunner(&failingWorkerdRunner{})
		failingService := &Service{
			kernel:        kernel,
			configManager: testConfigManager,
			engine:        failingEngine,
		}
		failingControlPlaneHandler := NewControlPlaneHandler(failingService)

		// Create endpoint
		createPayload := `{"name":"lazy-workerd-fn","runtime":"workerd"}`
		createRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader([]byte(createPayload)))
		createRequest.Header.Set("Authorization", authHeader)
		createResponseRecorder := httptest.NewRecorder()
		failingControlPlaneHandler.handleCreateEndpoint(createResponseRecorder, createRequest)
		require.Equal(t, http.StatusCreated, createResponseRecorder.Code)

		var createdEndpoint Endpoint
		require.NoError(t, json.Unmarshal(createResponseRecorder.Body.Bytes(), &createdEndpoint))

		// Deploy endpoint - triggers download which fails
		deployPayload := `{"bundle_content":"export default {};"}`
		deployRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+createdEndpoint.ID.String()+"/deploy", bytes.NewReader([]byte(deployPayload)))
		deployRequest.SetPathValue("endpoint_id", createdEndpoint.ID.String())
		deployRequest.Header.Set("Authorization", authHeader)
		deployResponseRecorder := httptest.NewRecorder()
		failingControlPlaneHandler.handleCreateDeployment(deployResponseRecorder, deployRequest)

		require.Equal(t, http.StatusBadGateway, deployResponseRecorder.Code)
		require.Contains(t, deployResponseRecorder.Body.String(), "failed to download runtime runner binary")

		// Verify database consistency - no active deployment and no deployments row committed
		var activeDeploymentID *uuid.UUID
		queryErr := kernel.DB().QueryRow(testCtx, "SELECT active_deployment_id FROM function.endpoints WHERE id = $1;", createdEndpoint.ID).Scan(&activeDeploymentID)
		require.NoError(t, queryErr)
		require.Nil(t, activeDeploymentID)

		var deploymentCount int
		countErr := kernel.DB().QueryRow(testCtx, "SELECT count(*) FROM function.deployments WHERE endpoint_id = $1;", createdEndpoint.ID).Scan(&deploymentCount)
		require.NoError(t, countErr)
		require.Zero(t, deploymentCount)

		// Test rollback error when runner download fails
		var dummyDepID uuid.UUID
		insertErr := kernel.DB().QueryRow(testCtx, `
			INSERT INTO function.deployments (endpoint_id, version, bundle_format, bundle_content, bundle_hash, bundle_files, environment_variables, status)
			VALUES ($1, 1, 'raw', 'code'::bytea, 'hash', '{}'::jsonb, '{}'::jsonb, 'active')
			RETURNING id;
		`, createdEndpoint.ID).Scan(&dummyDepID)
		require.NoError(t, insertErr)

		rollbackPayload := `{"target_version":1}`
		rollbackRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+createdEndpoint.ID.String()+"/rollback", bytes.NewReader([]byte(rollbackPayload)))
		rollbackRequest.SetPathValue("endpoint_id", createdEndpoint.ID.String())
		rollbackRequest.Header.Set("Authorization", authHeader)
		rollbackResponseRecorder := httptest.NewRecorder()
		failingControlPlaneHandler.handleRollbackDeployment(rollbackResponseRecorder, rollbackRequest)
		require.Equal(t, http.StatusBadGateway, rollbackResponseRecorder.Code)
		require.Contains(t, rollbackResponseRecorder.Body.String(), "failed to download runtime runner binary")
	})
}
