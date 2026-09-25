// Package function defines the serverless and edge function execution engine.
package function

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionControlPlaneHandlerDeploymentsUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	testRunner := &mockFailingRunner{}
	service.Engine().RegisterRunner(testRunner)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()

	readAuthContext := core.AuthContext{
		ServiceAccountID: "sa-deploy-read",
		JWT: core.JWTClaims{
			Subject:  "sa-deploy-read",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFunctionEndpointRead,
		},
	}
	readCtx := core.WithAuthContext(testCtx, readAuthContext)

	deployAuthContext := core.AuthContext{
		ServiceAccountID: "sa-deploy-write",
		JWT: core.JWTClaims{
			Subject:  "sa-deploy-write",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFunctionEndpointDeploy,
		},
	}
	deployCtx := core.WithAuthContext(testCtx, deployAuthContext)

	noScopeAuthContext := core.AuthContext{
		ServiceAccountID: "sa-no-scope",
		JWT: core.JWTClaims{
			Subject:  "sa-no-scope",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "",
		},
	}
	noScopeCtx := core.WithAuthContext(testCtx, noScopeAuthContext)

	// Create test endpoint
	var endpoint Endpoint
	createErr := kernel.DB().QueryRow(testCtx, `
		INSERT INTO function.endpoints (name, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at)
		VALUES ('deploy-unit-test-fn', 'workerd', 'index.js', 128, 30, true, clock_timestamp(), clock_timestamp())
		RETURNING id, name, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at;
	`).Scan(&endpoint.ID, &endpoint.Name, &endpoint.Runtime, &endpoint.Entrypoint, &endpoint.MemoryLimitMB, &endpoint.TimeoutSeconds, &endpoint.IsPublic, &endpoint.CreatedAt, &endpoint.UpdatedAt)
	require.NoError(t, createErr)

	randomUUID := "01923456-789a-7bc8-9def-0123456789ab"

	t.Run("Scope permissions and denials", func(t *testing.T) {
		// Deploy without scope -> 403
		deployNoScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/deploy", nil)
		deployNoScopeRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		deployNoScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(deployNoScopeResponseRecorder, deployNoScopeRequest)
		require.Equal(t, http.StatusForbidden, deployNoScopeResponseRecorder.Code)

		// List deployments without scope -> 403
		listNoScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/deployments", nil)
		listNoScopeRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		listNoScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListDeployments(listNoScopeResponseRecorder, listNoScopeRequest)
		require.Equal(t, http.StatusForbidden, listNoScopeResponseRecorder.Code)

		// Rollback without scope -> 403
		rollbackNoScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/rollback", nil)
		rollbackNoScopeRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		rollbackNoScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(rollbackNoScopeResponseRecorder, rollbackNoScopeRequest)
		require.Equal(t, http.StatusForbidden, rollbackNoScopeResponseRecorder.Code)
	})

	t.Run("Validations and edge cases", func(t *testing.T) {
		// Deploy - invalid UUID
		badIDRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/invalid-uuid/deploy", nil)
		badIDRequest.SetPathValue("endpoint_id", "invalid-uuid")
		badIDResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(badIDResponseRecorder, badIDRequest)
		require.Equal(t, http.StatusBadRequest, badIDResponseRecorder.Code)

		// Deploy - not found endpoint
		notFoundRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+randomUUID+"/deploy", bytes.NewReader([]byte("{}")))
		notFoundRequest.SetPathValue("endpoint_id", randomUUID)
		notFoundResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(notFoundResponseRecorder, notFoundRequest)
		require.Equal(t, http.StatusNotFound, notFoundResponseRecorder.Code)

		// Deploy - bad JSON
		badJSONRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/deploy", bytes.NewReader([]byte("{invalid-json")))
		badJSONRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(badJSONResponseRecorder, badJSONRequest)
		require.Equal(t, http.StatusBadRequest, badJSONResponseRecorder.Code)

		// Deploy - invalid bundle file path with ..
		invalidFilesCreateDeploymentInput := CreateDeploymentInput{
			BundleFiles: map[string]string{
				"../malicious.js": "console.log()",
			},
		}
		invalidFilesJSON, _ := json.Marshal(invalidFilesCreateDeploymentInput)
		invalidFilesRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/deploy", bytes.NewReader(invalidFilesJSON))
		invalidFilesRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		invalidFilesResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(invalidFilesResponseRecorder, invalidFilesRequest)
		require.Equal(t, http.StatusBadRequest, invalidFilesResponseRecorder.Code)

		// Deploy - bundle too large
		serviceConfig := service.ConfigManager().Get()
		serviceConfig.MaxBundleSizeBytes = 10
		require.NoError(t, service.ConfigManager().Set(testCtx, serviceConfig))

		tooLargeCreateDeploymentInput := CreateDeploymentInput{
			BundleContent: "this bundle content exceeds 10 bytes",
		}
		tooLargeJSON, _ := json.Marshal(tooLargeCreateDeploymentInput)
		tooLargeRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/deploy", bytes.NewReader(tooLargeJSON))
		tooLargeRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		tooLargeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(tooLargeResponseRecorder, tooLargeRequest)
		require.Equal(t, http.StatusRequestEntityTooLarge, tooLargeResponseRecorder.Code)

		// Reset max bundle size
		serviceConfig.MaxBundleSizeBytes = 10485760
		require.NoError(t, service.ConfigManager().Set(testCtx, serviceConfig))

		// List deployments - invalid UUID
		listBadIDRequest := httptest.NewRequestWithContext(readCtx, http.MethodGet, "/v1/_/function/endpoints/invalid-uuid/deployments", nil)
		listBadIDRequest.SetPathValue("endpoint_id", "invalid-uuid")
		listBadIDResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListDeployments(listBadIDResponseRecorder, listBadIDRequest)
		require.Equal(t, http.StatusBadRequest, listBadIDResponseRecorder.Code)

		// Rollback - invalid UUID
		rollBadIDRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/invalid-uuid/rollback", nil)
		rollBadIDRequest.SetPathValue("endpoint_id", "invalid-uuid")
		rollBadIDResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(rollBadIDResponseRecorder, rollBadIDRequest)
		require.Equal(t, http.StatusBadRequest, rollBadIDResponseRecorder.Code)

		// Rollback - bad JSON
		rollBadJSONRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/rollback", bytes.NewReader([]byte("{invalid-json")))
		rollBadJSONRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		rollBadJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(rollBadJSONResponseRecorder, rollBadJSONRequest)
		require.Equal(t, http.StatusBadRequest, rollBadJSONResponseRecorder.Code)

		// Rollback - not found version
		rollNotFoundRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/rollback", bytes.NewReader([]byte(`{"target_version":999}`)))
		rollNotFoundRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		rollNotFoundResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(rollNotFoundResponseRecorder, rollNotFoundRequest)
		require.Equal(t, http.StatusNotFound, rollNotFoundResponseRecorder.Code)

		// Deploy with WorkerdRuntimeConfig
		runtimeConfigCreateDeploymentInput := CreateDeploymentInput{
			BundleContent: "export default {};",
			WorkerdRuntimeConfig: &WorkerdRuntimeConfig{
				CompatibilityDate: "2026-08-04",
			},
		}
		runtimeConfigJSON, _ := json.Marshal(runtimeConfigCreateDeploymentInput)
		runtimeConfigRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/deploy", bytes.NewReader(runtimeConfigJSON))
		runtimeConfigRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		runtimeConfigResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(runtimeConfigResponseRecorder, runtimeConfigRequest)
		require.Equal(t, http.StatusCreated, runtimeConfigResponseRecorder.Code)

		// Deploy version 2
		versionTwoCreateDeploymentInput := CreateDeploymentInput{
			BundleContent: "export default { v: 2 };",
		}
		versionTwoJSON, _ := json.Marshal(versionTwoCreateDeploymentInput)
		versionTwoRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/deploy", bytes.NewReader(versionTwoJSON))
		versionTwoRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		versionTwoResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(versionTwoResponseRecorder, versionTwoRequest)
		require.Equal(t, http.StatusCreated, versionTwoResponseRecorder.Code)

		// List deployments with active deployments having WorkerdRuntimeConfig
		validListDeploymentsRequest := httptest.NewRequestWithContext(readCtx, http.MethodGet, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/deployments", nil)
		validListDeploymentsRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		validListDeploymentsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListDeployments(validListDeploymentsResponseRecorder, validListDeploymentsRequest)
		require.Equal(t, http.StatusOK, validListDeploymentsResponseRecorder.Code)

		// Deploy failing runner (generic error)
		testRunner.deployFail = true
		failCreateDeploymentInput := CreateDeploymentInput{
			BundleContent: "export default {};",
		}
		failDeployJSON, _ := json.Marshal(failCreateDeploymentInput)
		failDeployRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/deploy", bytes.NewReader(failDeployJSON))
		failDeployRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		failDeployResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(failDeployResponseRecorder, failDeployRequest)
		require.Equal(t, http.StatusInternalServerError, failDeployResponseRecorder.Code)
		testRunner.deployFail = false

		// Rollback with non-existent endpoint ID
		notFoundEndpointRollbackRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/01923456-789a-7bc8-9def-0123456789ab/rollback", bytes.NewReader([]byte(`{"target_version":1}`)))
		notFoundEndpointRollbackRequest.SetPathValue("endpoint_id", "01923456-789a-7bc8-9def-0123456789ab")
		notFoundEndpointRollbackResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(notFoundEndpointRollbackResponseRecorder, notFoundEndpointRollbackRequest)
		require.Equal(t, http.StatusNotFound, notFoundEndpointRollbackResponseRecorder.Code)

		// Rollback to deployment with WorkerdRuntimeConfig (version 1)
		validRollbackRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/rollback", bytes.NewReader([]byte(`{"target_version":1}`)))
		validRollbackRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		validRollbackResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(validRollbackResponseRecorder, validRollbackRequest)
		require.Equal(t, http.StatusOK, validRollbackResponseRecorder.Code)

		// Rollback with runner deploy error (reverting to previous deployment)
		testRunner.deployFail = true
		failRollbackRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/rollback", bytes.NewReader([]byte(`{"target_version":2}`)))
		failRollbackRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		failRollbackResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(failRollbackResponseRecorder, failRollbackRequest)
		require.Equal(t, http.StatusInternalServerError, failRollbackResponseRecorder.Code)
		testRunner.deployFail = false

		// Database error branches for Rollback and Deploy
		_, createTrgErr := kernel.DB().Exec(testCtx, `
			CREATE OR REPLACE FUNCTION function.trg_fail_update() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'simulated update failure';
			END;
			$$ LANGUAGE plpgsql;
			CREATE TRIGGER trg_test_fail_update BEFORE UPDATE ON function.endpoints
			FOR EACH ROW EXECUTE FUNCTION function.trg_fail_update();
		`)
		require.NoError(t, createTrgErr)

		rollbackUpdateFailRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/rollback", bytes.NewReader([]byte(`{"target_version":1}`)))
		rollbackUpdateFailRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		rollbackUpdateFailResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(rollbackUpdateFailResponseRecorder, rollbackUpdateFailRequest)
		require.Equal(t, http.StatusInternalServerError, rollbackUpdateFailResponseRecorder.Code)

		_, dropTrgErr := kernel.DB().Exec(testCtx, `DROP TRIGGER trg_test_fail_update ON function.endpoints;`)
		require.NoError(t, dropTrgErr)

		_, dropDeploymentsErr := kernel.DB().Exec(testCtx, `DROP TABLE function.deployments CASCADE;`)
		require.NoError(t, dropDeploymentsErr)

		createDepFailRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/deploy", bytes.NewReader([]byte(`{"bundle_content":"export default {};"}`)))
		createDepFailRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		createDepFailResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(createDepFailResponseRecorder, createDepFailRequest)
		require.Equal(t, http.StatusInternalServerError, createDepFailResponseRecorder.Code)

		rollbackQueryFailRequest := httptest.NewRequestWithContext(deployCtx, http.MethodPost, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/rollback", bytes.NewReader([]byte(`{"target_version":1}`)))
		rollbackQueryFailRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		rollbackQueryFailResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(rollbackQueryFailResponseRecorder, rollbackQueryFailRequest)
		require.Equal(t, http.StatusInternalServerError, rollbackQueryFailResponseRecorder.Code)
	})
}

func TestFunctionControlPlaneHandlerDeploymentsDatabaseFailureUnit(t *testing.T) {
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenService := NewService(brokenKernel)
	controlPlaneHandler := brokenService.ControlPlaneHandler()

	rootCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		ServiceAccountID: "sa-root",
		JWT: core.JWTClaims{
			Subject:  "sa-root",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "*",
		},
	})

	randomUUID := "01923456-789a-7bc8-9def-0123456789ab"

	// Create deployment DB failure -> 500
	createRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+randomUUID+"/deploy", strings.NewReader(`{"bundle_content":"export default {};"}`))
	createRequest.SetPathValue("endpoint_id", randomUUID)
	createResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateDeployment(createResponseRecorder, createRequest)
	require.Equal(t, http.StatusInternalServerError, createResponseRecorder.Code)

	// List deployments DB failure -> 500
	listRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints/"+randomUUID+"/deployments", nil)
	listRequest.SetPathValue("endpoint_id", randomUUID)
	listResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListDeployments(listResponseRecorder, listRequest)
	require.Equal(t, http.StatusInternalServerError, listResponseRecorder.Code)

	// Rollback deployment DB failure -> 500
	rollbackRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints/"+randomUUID+"/rollback", strings.NewReader(`{"target_version":1}`))
	rollbackRequest.SetPathValue("endpoint_id", randomUUID)
	rollbackResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleRollbackDeployment(rollbackResponseRecorder, rollbackRequest)
	require.Equal(t, http.StatusInternalServerError, rollbackResponseRecorder.Code)
}

type failingTarWriter struct{}

func (writer *failingTarWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type byteLimitWriter struct {
	remainingBytes int
}

func (writer *byteLimitWriter) Write(payload []byte) (int, error) {
	if writer.remainingBytes < len(payload) {
		return 0, errors.New("byte limit exceeded")
	}
	writer.remainingBytes -= len(payload)
	return len(payload), nil
}

func TestFunctionDecodeCodeBundleUnit(t *testing.T) {
	t.Parallel()

	// 1. Base64 encoded bundle
	base64Bundle := base64.StdEncoding.EncodeToString([]byte("console.log('test')"))
	decoded := decodeCodeBundle(base64Bundle)
	require.Equal(t, []byte("console.log('test')"), decoded)

	// 2. Plain script
	plain := decodeCodeBundle("console.log('raw')")
	require.Equal(t, []byte("console.log('raw')"), plain)
}

func TestFunctionPackageBundleFilesToUnit(t *testing.T) {
	t.Parallel()

	// 1. Write header error with failingTarWriter
	_, packageHeaderErr := packageBundleFilesTo(map[string]string{"index.js": "export default {};"}, &failingTarWriter{})
	require.Error(t, packageHeaderErr)

	// 2. Content write error
	_, packageContentErr := packageBundleFilesTo(map[string]string{"index.js": "export default {};"}, &byteLimitWriter{remainingBytes: 512})
	require.Error(t, packageContentErr)

	// 3. Close write error
	_, packageCloseErr := packageBundleFilesTo(map[string]string{"index.js": "export default {};"}, &byteLimitWriter{remainingBytes: 512 + len("export default {};")})
	require.Error(t, packageCloseErr)
}
