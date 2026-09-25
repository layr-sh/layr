package function

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionBaseHandlerInvokeUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	testRunner := &mockFailingRunner{}
	service.Engine().RegisterRunner(testRunner)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	baseHandler := service.BaseHandler()

	t.Run("handleInvokeEndpoint validations", func(t *testing.T) {
		// Missing endpoint name
		missingNameRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/", nil)
		missingNameResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(missingNameResponseRecorder, missingNameRequest)
		require.Equal(t, http.StatusBadRequest, missingNameResponseRecorder.Code)

		// Endpoint not found
		notFoundRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/nonexistent", nil)
		notFoundRequest.SetPathValue("endpoint_name", "nonexistent")
		notFoundResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(notFoundResponseRecorder, notFoundRequest)
		require.Equal(t, http.StatusNotFound, notFoundResponseRecorder.Code)

		// Endpoint created but no deployment
		createdEndpoint := insertTestEndpoint(testCtx, t, kernel, "no-deploy-fn", true)

		noDeployRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/"+createdEndpoint.Name, nil)
		noDeployRequest.SetPathValue("endpoint_name", createdEndpoint.Name)
		noDeployResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(noDeployResponseRecorder, noDeployRequest)
		require.Equal(t, http.StatusServiceUnavailable, noDeployResponseRecorder.Code)

		// Deploy endpoint
		deployTestEndpoint(testCtx, t, service, &createdEndpoint, "export default {};")

		// Successful invoke
		successRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/"+createdEndpoint.Name+"/test", nil)
		successRequest.SetPathValue("endpoint_name", createdEndpoint.Name)
		successResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(successResponseRecorder, successRequest)
		require.Equal(t, http.StatusOK, successResponseRecorder.Code)

		// Failing forward
		testRunner.shouldFail = true
		failRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/"+createdEndpoint.Name+"/test", nil)
		failRequest.SetPathValue("endpoint_name", createdEndpoint.Name)
		failResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(failResponseRecorder, failRequest)
		testRunner.shouldFail = false
	})

	t.Run("handleInvokeEndpoint execution tracking and logs", func(t *testing.T) {
		// Test capturingResponseWriter with implicit WriteHeader
		capturingResponseRecorder := httptest.NewRecorder()
		capturingResponseRecorder.Header().Set("X-Layr-Stdout", "test%20stdout")
		capturingResponseRecorder.Header().Set("X-Layr-Stderr", "test%20stderr")
		recordedResponseWriter := &capturingResponseWriter{ResponseWriter: capturingResponseRecorder}
		_, writeErr := recordedResponseWriter.Write([]byte("raw worker body"))
		require.NoError(t, writeErr)
		require.Equal(t, http.StatusOK, recordedResponseWriter.statusCode)
		require.Equal(t, "test stdout", recordedResponseWriter.stdout)
		require.Equal(t, "test stderr", recordedResponseWriter.stderr)
		require.Empty(t, capturingResponseRecorder.Header().Get("X-Layr-Stdout"))
		require.Empty(t, capturingResponseRecorder.Header().Get("X-Layr-Stderr"))

		// Test capturingResponseWriter with underlying write error
		failingCapturingResponseWriter := &capturingResponseWriter{ResponseWriter: &failingResponseWriter{}}
		_, failedWriteErr := failingCapturingResponseWriter.Write([]byte("test"))
		require.Error(t, failedWriteErr)

		// Create and deploy endpoint for tracking
		trackingEndpoint := insertTestEndpoint(testCtx, t, kernel, "tracking-ep", true)
		deployTestEndpoint(testCtx, t, service, &trackingEndpoint, "export default { v: 2 };")

		// Successful invoke records execution
		trackInvokeRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/"+trackingEndpoint.Name+"/test", nil)
		trackInvokeRequest.SetPathValue("endpoint_name", trackingEndpoint.Name)
		trackInvokeResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(trackInvokeResponseRecorder, trackInvokeRequest)
		require.Equal(t, http.StatusOK, trackInvokeResponseRecorder.Code)

		// Table alteration causing GetActiveDeployment to return internal server error
		_, _ = kernel.DB().Exec(testCtx, "ALTER TABLE function.deployments RENAME TO deployments_backup")
		brokenInvokeRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/"+trackingEndpoint.Name, nil)
		brokenInvokeRequest.SetPathValue("endpoint_name", trackingEndpoint.Name)
		brokenInvokeResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(brokenInvokeResponseRecorder, brokenInvokeRequest)
		require.Equal(t, http.StatusInternalServerError, brokenInvokeResponseRecorder.Code)

		// Table alteration causing GetEndpointByName to return internal server error
		_, _ = kernel.DB().Exec(testCtx, "ALTER TABLE function.endpoints RENAME TO endpoints_backup")
		brokenEndpointInvokeRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/"+trackingEndpoint.Name, nil)
		brokenEndpointInvokeRequest.SetPathValue("endpoint_name", trackingEndpoint.Name)
		brokenEndpointInvokeResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(brokenEndpointInvokeResponseRecorder, brokenEndpointInvokeRequest)
		require.Equal(t, http.StatusInternalServerError, brokenEndpointInvokeResponseRecorder.Code)

		// Restore tables
		_, _ = kernel.DB().Exec(testCtx, "ALTER TABLE function.endpoints_backup RENAME TO endpoints")
		_, _ = kernel.DB().Exec(testCtx, "ALTER TABLE function.deployments_backup RENAME TO deployments")
	})

	t.Run("handleInvokeEndpoint auth validations", func(t *testing.T) {
		// 1. Create private endpoint
		privateEndpoint := insertTestEndpoint(testCtx, t, kernel, "secure-internal-fn", false)
		require.False(t, privateEndpoint.IsPublic)

		deployTestEndpoint(testCtx, t, service, &privateEndpoint, "export default { private: true };")

		// 2. Invoke without any credentials -> 401
		unauthRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/"+privateEndpoint.Name, nil)
		unauthRequest.SetPathValue("endpoint_name", privateEndpoint.Name)
		unauthResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(unauthResponseRecorder, unauthRequest)
		require.Equal(t, http.StatusUnauthorized, unauthResponseRecorder.Code)

		// 3. Create service account without invoke scope -> 403
		noScopeServiceAccount, noScopeErr := kernel.ServiceAccountManager().Create(testCtx, core.CreateServiceAccountInput{
			Name:   "no-scope-sa",
			Scopes: []string{core.ScopeFunctionEndpointRead},
		})
		require.NoError(t, noScopeErr)

		noScopeCtx := core.WithAuthContext(testCtx, core.AuthContext{
			ServiceAccountID: noScopeServiceAccount.ID,
			JWT: core.JWTClaims{
				Role:  "service_role",
				Scope: core.ScopeFunctionEndpointRead,
			},
		})

		forbiddenRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/function/"+privateEndpoint.Name, nil)
		forbiddenRequest.SetPathValue("endpoint_name", privateEndpoint.Name)
		forbiddenResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(forbiddenResponseRecorder, forbiddenRequest)
		require.Equal(t, http.StatusForbidden, forbiddenResponseRecorder.Code)

		// 4. Create service account with invoke scope -> 200
		validServiceAccount, validErr := kernel.ServiceAccountManager().Create(testCtx, core.CreateServiceAccountInput{
			Name:   "invoker-sa",
			Scopes: []string{core.ScopeFunctionEndpointInvoke},
		})
		require.NoError(t, validErr)

		authedCtx := core.WithAuthContext(testCtx, core.AuthContext{
			ServiceAccountID: validServiceAccount.ID,
			JWT: core.JWTClaims{
				Role:  "service_role",
				Scope: core.ScopeFunctionEndpointInvoke,
			},
		})

		validRequest := httptest.NewRequestWithContext(authedCtx, http.MethodGet, "/v1/function/"+privateEndpoint.Name, nil)
		validRequest.SetPathValue("endpoint_name", privateEndpoint.Name)
		validResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(validResponseRecorder, validRequest)
		require.Equal(t, http.StatusOK, validResponseRecorder.Code)

		// 5. Invoke with End-User AuthContext -> 200
		userCtx := core.WithAuthContext(testCtx, core.AuthContext{
			UserID: "usr_019245abcdef789",
			JWT: core.JWTClaims{
				Subject: "usr_019245abcdef789",
				Role:    "user",
				Email:   "user@layr.sh",
			},
		})
		userRequest := httptest.NewRequestWithContext(userCtx, http.MethodGet, "/v1/function/"+privateEndpoint.Name, nil)
		userRequest.SetPathValue("endpoint_name", privateEndpoint.Name)
		userResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInvokeEndpoint(userResponseRecorder, userRequest)
		require.Equal(t, http.StatusOK, userResponseRecorder.Code)
	})
}

func TestFunctionInjectAuthHeadersUnit(t *testing.T) {
	t.Parallel()

	// 1. Unauthenticated caller
	unauthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	injectAuthHeaders(unauthRequest)
	require.NotEmpty(t, unauthRequest.Header.Get("X-Layr-Auth-Context"))

	// 2. End-User authenticated caller
	userCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		UserID: "usr_100",
		JWT: core.JWTClaims{
			Subject: "usr_100",
			Role:    "user",
			Email:   "dev@layr.sh",
		},
	})
	userRequest := httptest.NewRequestWithContext(userCtx, http.MethodGet, "/test", nil)
	injectAuthHeaders(userRequest)
	require.Contains(t, userRequest.Header.Get("X-Layr-Auth-Context"), "usr_100")

	// 3. Service Account authenticated caller
	serviceAccountCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		ServiceAccountID: "sa_200",
		JWT: core.JWTClaims{
			Subject: "sa_200",
			Role:    "service_role",
			Scope:   "function:endpoint.invoke core:event.read",
		},
	})
	serviceAccountRequest := httptest.NewRequestWithContext(serviceAccountCtx, http.MethodGet, "/test", nil)
	injectAuthHeaders(serviceAccountRequest)
	require.Contains(t, serviceAccountRequest.Header.Get("X-Layr-Auth-Context"), "sa_200")
}
