// Package function defines the serverless and edge function execution engine.
package function

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionFullLifecycleE2E(t *testing.T) {
	testKernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()

	// 1. Initialize Function Service with real workerd runtime
	functionService := NewService(testKernel)
	require.NoError(t, functionService.Start(testCtx))
	defer functionService.Stop()

	coreServer := core.NewServer(testKernel)
	functionService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())
	functionService.RegisterHostRoutes(coreServer)

	// 2. Create Service Account with root scopes
	serviceAccount, accountErr := testKernel.ServiceAccountManager().Create(testCtx, core.CreateServiceAccountInput{
		Name:   "Function E2E Testing Account",
		Scopes: []string{core.ScopeRoot},
	})
	require.NoError(t, accountErr)
	authHeader := "Bearer " + serviceAccount.SecretKey

	// -------------------------------------------------------------------------
	// PART A: Classic ServiceWorker Syntax (workerd README standard)
	// -------------------------------------------------------------------------
	t.Log("--- Testing ServiceWorker syntax with addEventListener ---")

	createSwPayload := `{"name":"sw-worker","description":"Classic ServiceWorker Runtime","runtime":"workerd"}`
	createSwRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader([]byte(createSwPayload)))
	createSwRequest.Header.Set("Content-Type", "application/json")
	createSwRequest.Header.Set("Authorization", authHeader)
	createSwResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createSwResponseRecorder, createSwRequest)
	require.Equal(t, http.StatusCreated, createSwResponseRecorder.Code)

	var swEndpoint Endpoint
	require.NoError(t, json.Unmarshal(createSwResponseRecorder.Body.Bytes(), &swEndpoint))

	deploySwPayload := `{
		"bundle_content": "addEventListener('fetch', event => { event.respondWith(new Response(JSON.stringify({ runtime: 'workerd', syntax: 'serviceWorkerScript', bound_env: typeof GREETING_VAR !== 'undefined' ? GREETING_VAR : 'none' }), { headers: { 'Content-Type': 'application/json', 'X-Runtime': 'workerd-v8' } })); });",
		"environment_variables": { "GREETING_VAR": "Hello from Layr Environment" }
	}`
	deploySwRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+swEndpoint.ID.String()+"/deploy", bytes.NewReader([]byte(deploySwPayload)))
	deploySwRequest.Header.Set("Content-Type", "application/json")
	deploySwRequest.Header.Set("Authorization", authHeader)
	deploySwResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deploySwResponseRecorder, deploySwRequest)
	require.Equal(t, http.StatusCreated, deploySwResponseRecorder.Code)

	publishableKey := testKernel.CryptoKeyManager().DerivePublishableKey()

	// Public function invocation without publishable key
	invokeSwRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/sw-worker/hello", nil)
	invokeSwResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(invokeSwResponseRecorder, invokeSwRequest)
	require.Equal(t, http.StatusOK, invokeSwResponseRecorder.Code)
	require.Equal(t, "workerd-v8", invokeSwResponseRecorder.Header().Get("X-Runtime"))

	var swResponseBody map[string]string
	require.NoError(t, json.Unmarshal(invokeSwResponseRecorder.Body.Bytes(), &swResponseBody))
	require.Equal(t, "workerd", swResponseBody["runtime"])
	require.Equal(t, "serviceWorkerScript", swResponseBody["syntax"])
	require.Equal(t, "Hello from Layr Environment", swResponseBody["bound_env"])
	t.Logf("ServiceWorker execution verified: %s", invokeSwResponseRecorder.Body.String())

	// -------------------------------------------------------------------------
	// PART B: Modern ES Module Syntax with Web Crypto & Dynamic Computations
	// -------------------------------------------------------------------------
	t.Log("--- Testing ES Module syntax with WebCrypto calculation ---")

	createEsmPayload := `{"name":"crypto-worker","description":"ESM Worker with WebCrypto","runtime":"workerd"}`
	createEsmRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader([]byte(createEsmPayload)))
	createEsmRequest.Header.Set("Content-Type", "application/json")
	createEsmRequest.Header.Set("Authorization", authHeader)
	createEsmResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createEsmResponseRecorder, createEsmRequest)
	require.Equal(t, http.StatusCreated, createEsmResponseRecorder.Code)

	var esmEndpoint Endpoint
	require.NoError(t, json.Unmarshal(createEsmResponseRecorder.Body.Bytes(), &esmEndpoint))

	deployEsmV1Payload := `{
		"bundle_content": "import { Buffer } from 'node:buffer'; import { EventEmitter } from 'node:events'; export default { async fetch(request, env, ctx) { const url = new URL(request.url); const name = url.searchParams.get('name') || 'world'; const data = new TextEncoder().encode('layr-' + name); const hashBuf = await crypto.subtle.digest('SHA-256', data); const hashHex = Array.from(new Uint8Array(hashBuf)).map(b => b.toString(16).padStart(2, '0')).join(''); const nodeBuffer = Buffer.from('node-compat-' + name); const nodeBase64 = nodeBuffer.toString('base64'); const eventEmitter = new EventEmitter(); let emittedMessage = ''; eventEmitter.on('test-event', msg => { emittedMessage = msg; }); eventEmitter.emit('test-event', 'event-received'); return new Response(JSON.stringify({ greeting: 'Hello ' + name, version: 1, sha256: hashHex, api_key: env.API_KEY || 'none', node_base64: nodeBase64, node_event: emittedMessage }), { headers: { 'Content-Type': 'application/json' } }); } };",
		"environment_variables": { "API_KEY": "secret-super-key-99" }
	}`
	deployEsmV1Request := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+esmEndpoint.ID.String()+"/deploy", bytes.NewReader([]byte(deployEsmV1Payload)))
	deployEsmV1Request.Header.Set("Content-Type", "application/json")
	deployEsmV1Request.Header.Set("Authorization", authHeader)
	deployEsmV1ResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deployEsmV1ResponseRecorder, deployEsmV1Request)
	require.Equal(t, http.StatusCreated, deployEsmV1ResponseRecorder.Code)

	// Public Ingress HTTP execution with URL query params
	invokeEsmRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/crypto-worker/hash?name=antigravity", nil)
	invokeEsmRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	invokeEsmResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(invokeEsmResponseRecorder, invokeEsmRequest)
	require.Equal(t, http.StatusOK, invokeEsmResponseRecorder.Code)

	var esmResponse map[string]any
	require.NoError(t, json.Unmarshal(invokeEsmResponseRecorder.Body.Bytes(), &esmResponse))
	require.Equal(t, "Hello antigravity", esmResponse["greeting"])
	require.Equal(t, float64(1), esmResponse["version"])
	require.Equal(t, "secret-super-key-99", esmResponse["api_key"])
	require.NotEmpty(t, esmResponse["sha256"])
	require.Equal(t, "bm9kZS1jb21wYXQtYW50aWdyYXZpdHk=", esmResponse["node_base64"])
	require.Equal(t, "event-received", esmResponse["node_event"])
	t.Logf("ES Module computation verified: %s", invokeEsmResponseRecorder.Body.String())

	// Transparent HTTP POST Ingress to endpoint root without publishable key
	directPayload := `{"user":"direct-caller"}`
	directRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/function/crypto-worker", bytes.NewReader([]byte(directPayload)))
	directRequest.Header.Set("Content-Type", "application/json")
	directResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(directResponseRecorder, directRequest)
	require.Equal(t, http.StatusOK, directResponseRecorder.Code)

	var directResponse map[string]any
	require.NoError(t, json.Unmarshal(directResponseRecorder.Body.Bytes(), &directResponse))
	require.Contains(t, directResponse["greeting"], "Hello world")

	// Inspect execution logs in control plane
	listExecsRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/endpoints/"+esmEndpoint.ID.String()+"/executions", nil)
	listExecsRequest.Header.Set("Authorization", authHeader)
	listExecsResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(listExecsResponseRecorder, listExecsRequest)
	require.Equal(t, http.StatusOK, listExecsResponseRecorder.Code)
	var listExecutionsResponse ListExecutionsResponse
	require.NoError(t, json.Unmarshal(listExecsResponseRecorder.Body.Bytes(), &listExecutionsResponse))
	require.GreaterOrEqual(t, listExecutionsResponse.Count, 2)
	require.Equal(t, http.StatusOK, listExecutionsResponse.Executions[0].StatusCode)

	// -------------------------------------------------------------------------
	// PART C: Live Hot-Reload to v2 & Immediate Rollback to v1
	// -------------------------------------------------------------------------
	t.Log("--- Testing zero-downtime hot reload and rollback ---")

	deployV2Payload := `{"bundle_content":"export default { fetch(req) { return new Response('Hot reload v2 active!'); } };"}`
	deployV2Request := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+esmEndpoint.ID.String()+"/deploy", bytes.NewReader([]byte(deployV2Payload)))
	deployV2Request.Header.Set("Content-Type", "application/json")
	deployV2Request.Header.Set("Authorization", authHeader)
	deployV2ResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deployV2ResponseRecorder, deployV2Request)
	require.Equal(t, http.StatusCreated, deployV2ResponseRecorder.Code)

	invokeV2Request := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/crypto-worker/status", nil)
	invokeV2Request.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	invokeV2ResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(invokeV2ResponseRecorder, invokeV2Request)
	require.Equal(t, http.StatusOK, invokeV2ResponseRecorder.Code)
	require.Equal(t, "Hot reload v2 active!", invokeV2ResponseRecorder.Body.String())
	t.Log("Hot reload v2 verified successfully!")

	// Rollback to v1
	rollbackPayload := `{"target_version":1}`
	rollbackRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+esmEndpoint.ID.String()+"/rollback", bytes.NewReader([]byte(rollbackPayload)))
	rollbackRequest.Header.Set("Content-Type", "application/json")
	rollbackRequest.Header.Set("Authorization", authHeader)
	rollbackResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(rollbackResponseRecorder, rollbackRequest)
	require.Equal(t, http.StatusOK, rollbackResponseRecorder.Code)

	invokeRollbackRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/crypto-worker/hash?name=rollback", nil)
	invokeRollbackRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	invokeRollbackResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(invokeRollbackResponseRecorder, invokeRollbackRequest)
	require.Equal(t, http.StatusOK, invokeRollbackResponseRecorder.Code)
	require.Contains(t, invokeRollbackResponseRecorder.Body.String(), "Hello rollback")
	t.Log("Rollback to v1 verified successfully!")

	// -------------------------------------------------------------------------
	// PART D: Private vs Public Endpoints & Core EventHook Triggering
	// -------------------------------------------------------------------------
	t.Log("--- Testing Private vs Public Endpoints & Core EventHook Triggering ---")

	// 1. Create a private endpoint
	createPrivPayload := `{"name":"private-worker","is_public":false,"runtime":"workerd"}`
	createPrivRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader([]byte(createPrivPayload)))
	createPrivRequest.Header.Set("Content-Type", "application/json")
	createPrivRequest.Header.Set("Authorization", authHeader)
	createPrivResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createPrivResponseRecorder, createPrivRequest)
	require.Equal(t, http.StatusCreated, createPrivResponseRecorder.Code)

	var privEndpoint Endpoint
	require.NoError(t, json.Unmarshal(createPrivResponseRecorder.Body.Bytes(), &privEndpoint))
	require.False(t, privEndpoint.IsPublic)

	deployPrivPayload := `{"bundle_content": "export default { async fetch(req, env, ctx) { const auth = ctx.auth; return new Response(JSON.stringify({ message: 'Hello from private isolate', is_authenticated: auth.isAuthenticated(), is_user: auth.isUser(), is_service_account: auth.isServiceAccount(), userId: auth.userId, serviceAccountId: auth.serviceAccountId, role: auth.role(), has_invoke_scope: auth.hasScope('function:endpoint.invoke') }), { headers: { 'Content-Type': 'application/json' } }); } };"}`
	deployPrivRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+privEndpoint.ID.String()+"/deploy", bytes.NewReader([]byte(deployPrivPayload)))
	deployPrivRequest.Header.Set("Content-Type", "application/json")
	deployPrivRequest.Header.Set("Authorization", authHeader)
	deployPrivResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deployPrivResponseRecorder, deployPrivRequest)
	require.Equal(t, http.StatusCreated, deployPrivResponseRecorder.Code)

	// 2. Unauthenticated invocation of private endpoint should fail with 401
	unauthRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/private-worker", nil)
	unauthResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(unauthResponseRecorder, unauthRequest)
	require.Equal(t, http.StatusUnauthorized, unauthResponseRecorder.Code)

	// 3. Invocation of private endpoint with Service Account key should succeed
	authPrivRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/private-worker", nil)
	authPrivRequest.Header.Set("Authorization", authHeader)
	authPrivResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(authPrivResponseRecorder, authPrivRequest)
	require.Equal(t, http.StatusOK, authPrivResponseRecorder.Code)
	var privServiceAccountResponse map[string]any
	require.NoError(t, json.Unmarshal(authPrivResponseRecorder.Body.Bytes(), &privServiceAccountResponse))
	require.Equal(t, "Hello from private isolate", privServiceAccountResponse["message"])
	require.Equal(t, true, privServiceAccountResponse["is_authenticated"])
	require.Equal(t, false, privServiceAccountResponse["is_user"])
	require.Equal(t, true, privServiceAccountResponse["is_service_account"])
	require.Equal(t, true, privServiceAccountResponse["has_invoke_scope"])
	require.Equal(t, "service_role", privServiceAccountResponse["role"])

	// 4. Invocation of private endpoint with End-User JWT should succeed with user auth context
	userToken := testKernel.JWTSigner().GenerateAccessToken(core.JWTClaims{
		Subject: "usr_e2e_tester_123",
		Role:    "user",
		Email:   "tester@layr.sh",
	})
	userPrivRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/function/private-worker", nil)
	userPrivRequest.Header.Set("Authorization", "Bearer "+userToken)
	userPrivResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(userPrivResponseRecorder, userPrivRequest)
	require.Equal(t, http.StatusOK, userPrivResponseRecorder.Code)
	var privUserResponse map[string]any
	require.NoError(t, json.Unmarshal(userPrivResponseRecorder.Body.Bytes(), &privUserResponse))
	require.Equal(t, "Hello from private isolate", privUserResponse["message"])
	require.Equal(t, true, privUserResponse["is_authenticated"])
	require.Equal(t, true, privUserResponse["is_user"])
	require.Equal(t, false, privUserResponse["is_service_account"])
	require.Equal(t, "usr_e2e_tester_123", privUserResponse["userId"])
	require.Equal(t, "user", privUserResponse["role"])

	// 4. Test Core EventHook triggering function over HTTP webhook
	eventServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		request.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
		coreServer.Handler().ServeHTTP(responseWriter, request)
	}))
	defer eventServer.Close()

	hookTargetURL := eventServer.URL + "/v1/function/sw-worker"
	_, hookErr := testKernel.EventBus().EventHookManager().Create(testCtx, core.CreateEventHookInput{
		Name:          "payment-hook-to-function",
		Driver:        core.EventHookDriverHTTP,
		HTTPTargetURL: &hookTargetURL,
		EventTypes:    []string{"payment.succeeded"},
	})
	require.NoError(t, hookErr)

	eventFiredChannel := make(chan bool, 1)
	testKernel.EventBus().Subscribe("function.execution.completed", func(_ context.Context, _ core.Event) error {
		eventFiredChannel <- true
		return nil
	})

	testKernel.EventBus().PublishSync(testCtx, core.NewEvent("payment.succeeded", map[string]string{
		"amount": "100.00",
	}).WithResourceID("pay-100"))

	select {
	case <-eventFiredChannel:
		t.Log("Function successfully triggered by Core EventHook!")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for event hook to trigger function execution")
	}

	// -------------------------------------------------------------------------
	// PART E: Runtime Health, Process PID, Stats, and Clean Deletion
	// -------------------------------------------------------------------------
	activeRunner, runnerErr := functionService.Engine().GetRunner("workerd")
	require.NoError(t, runnerErr)
	runnerHealth, healthErr := activeRunner.Health(testCtx)
	require.NoError(t, healthErr)
	require.True(t, runnerHealth.Available)
	require.Greater(t, runnerHealth.ProcessPID, 0)
	t.Logf("Workerd runtime health: PID=%d, ActiveIsolates=%d", runnerHealth.ProcessPID, runnerHealth.ActiveIsolates)

	statsRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/stats", nil)
	statsRequest.Header.Set("Authorization", authHeader)
	statsResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(statsResponseRecorder, statsRequest)
	require.Equal(t, http.StatusOK, statsResponseRecorder.Code)

	// -------------------------------------------------------------------------
	// PART F: Multi-Endpoint Custom Domain Routing under Ingress TLS
	// -------------------------------------------------------------------------
	t.Log("--- Testing multi-endpoint custom domain routing under Ingress TLS ---")

	// 1. Create a users endpoint
	createUsersPayload := `{"name":"users-microservice","is_public":true}`
	createUsersRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader([]byte(createUsersPayload)))
	createUsersRequest.Header.Set("Content-Type", "application/json")
	createUsersRequest.Header.Set("Authorization", authHeader)
	createUsersResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createUsersResponseRecorder, createUsersRequest)
	require.Equal(t, http.StatusCreated, createUsersResponseRecorder.Code)

	var usersEndpoint Endpoint
	require.NoError(t, json.Unmarshal(createUsersResponseRecorder.Body.Bytes(), &usersEndpoint))

	deployUsersPayload := `{"bundle_content":"export default { fetch(req) { return new Response(JSON.stringify({service:'users-microservice',path:new URL(req.url).pathname}),{headers:{'content-type':'application/json'}}); } };"}`
	deployUsersRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+usersEndpoint.ID.String()+"/deploy", bytes.NewReader([]byte(deployUsersPayload)))
	deployUsersRequest.Header.Set("Content-Type", "application/json")
	deployUsersRequest.Header.Set("Authorization", authHeader)
	deployUsersResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deployUsersResponseRecorder, deployUsersRequest)
	require.Equal(t, http.StatusCreated, deployUsersResponseRecorder.Code)

	// Create a root gateway endpoint
	createGatewayPayload := `{"name":"gateway-microservice","is_public":true}`
	createGatewayRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader([]byte(createGatewayPayload)))
	createGatewayRequest.Header.Set("Content-Type", "application/json")
	createGatewayRequest.Header.Set("Authorization", authHeader)
	createGatewayResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createGatewayResponseRecorder, createGatewayRequest)
	require.Equal(t, http.StatusCreated, createGatewayResponseRecorder.Code)

	var gatewayEndpoint Endpoint
	require.NoError(t, json.Unmarshal(createGatewayResponseRecorder.Body.Bytes(), &gatewayEndpoint))

	deployGatewayPayload := `{"bundle_content":"export default { fetch(req) { return new Response(JSON.stringify({service:'gateway-root',path:new URL(req.url).pathname}),{headers:{'content-type':'application/json'}}); } };"}`
	deployGatewayRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+gatewayEndpoint.ID.String()+"/deploy", bytes.NewReader([]byte(deployGatewayPayload)))
	deployGatewayRequest.Header.Set("Content-Type", "application/json")
	deployGatewayRequest.Header.Set("Authorization", authHeader)
	deployGatewayResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deployGatewayResponseRecorder, deployGatewayRequest)
	require.Equal(t, http.StatusCreated, deployGatewayResponseRecorder.Code)

	// 2. Register standalone custom domain via control plane
	createDomainPayload := `{"domain":"api.mycustomworker.io"}`
	createDomainRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/domains", bytes.NewReader([]byte(createDomainPayload)))
	createDomainRequest.Header.Set("Content-Type", "application/json")
	createDomainRequest.Header.Set("Authorization", authHeader)
	createDomainResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createDomainResponseRecorder, createDomainRequest)
	require.Equal(t, http.StatusCreated, createDomainResponseRecorder.Code)

	var customDomain CustomDomain
	require.NoError(t, json.Unmarshal(createDomainResponseRecorder.Body.Bytes(), &customDomain))
	require.Equal(t, "api.mycustomworker.io", customDomain.Domain)

	// 3. Attach gatewayEndpoint to root ("/") route on the custom domain
	rootRoutePayload := `{"custom_domain_id":"` + customDomain.ID.String() + `","path_prefix":"/"}`
	rootRouteRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+gatewayEndpoint.ID.String()+"/domains", bytes.NewReader([]byte(rootRoutePayload)))
	rootRouteRequest.Header.Set("Content-Type", "application/json")
	rootRouteRequest.Header.Set("Authorization", authHeader)
	rootRouteResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(rootRouteResponseRecorder, rootRouteRequest)
	require.Equal(t, http.StatusCreated, rootRouteResponseRecorder.Code)

	// 4. Attach usersEndpoint to "/users" route on the same custom domain
	usersRoutePayload := `{"custom_domain_id":"` + customDomain.ID.String() + `","path_prefix":"/users"}`
	usersRouteRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+usersEndpoint.ID.String()+"/domains", bytes.NewReader([]byte(usersRoutePayload)))
	usersRouteRequest.Header.Set("Content-Type", "application/json")
	usersRouteRequest.Header.Set("Authorization", authHeader)
	usersRouteResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(usersRouteResponseRecorder, usersRouteRequest)
	require.Equal(t, http.StatusCreated, usersRouteResponseRecorder.Code)

	// 5. Ingress traffic with Host: api.mycustomworker.io and X-Forwarded-Proto: https
	// Test routing to /users/profile -> matches usersEndpoint
	usersTrafficRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "https://api.mycustomworker.io/users/profile", nil)
	usersTrafficRequest.Host = "api.mycustomworker.io"
	usersTrafficRequest.Header.Set("X-Forwarded-Proto", "https")
	usersTrafficResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(usersTrafficResponseRecorder, usersTrafficRequest)
	require.Equal(t, http.StatusOK, usersTrafficResponseRecorder.Code)

	var usersTrafficResponse map[string]any
	require.NoError(t, json.Unmarshal(usersTrafficResponseRecorder.Body.Bytes(), &usersTrafficResponse))
	require.Equal(t, "users-microservice", usersTrafficResponse["service"])
	require.Equal(t, "/users/profile", usersTrafficResponse["path"])

	// Test routing to /home -> falls back to gatewayEndpoint root prefix
	gatewayTrafficRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "https://api.mycustomworker.io/home", nil)
	gatewayTrafficRequest.Host = "api.mycustomworker.io:443"
	gatewayTrafficRequest.Header.Set("X-Forwarded-Proto", "https")
	gatewayTrafficResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(gatewayTrafficResponseRecorder, gatewayTrafficRequest)
	require.Equal(t, http.StatusOK, gatewayTrafficResponseRecorder.Code)

	var gatewayTrafficResponse map[string]any
	require.NoError(t, json.Unmarshal(gatewayTrafficResponseRecorder.Body.Bytes(), &gatewayTrafficResponse))
	require.Equal(t, "gateway-root", gatewayTrafficResponse["service"])
	require.Equal(t, "/home", gatewayTrafficResponse["path"])

	// Verify telemetry executions recorded for both endpoints
	listUsersExecRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/endpoints/"+usersEndpoint.ID.String()+"/executions", nil)
	listUsersExecRequest.Header.Set("Authorization", authHeader)
	listUsersExecResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(listUsersExecResponseRecorder, listUsersExecRequest)
	require.Equal(t, http.StatusOK, listUsersExecResponseRecorder.Code)

	var usersListExecutionsResponse ListExecutionsResponse
	require.NoError(t, json.Unmarshal(listUsersExecResponseRecorder.Body.Bytes(), &usersListExecutionsResponse))
	require.Equal(t, 1, usersListExecutionsResponse.Count)

	listGatewayExecRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/endpoints/"+gatewayEndpoint.ID.String()+"/executions", nil)
	listGatewayExecRequest.Header.Set("Authorization", authHeader)
	listGatewayExecResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(listGatewayExecResponseRecorder, listGatewayExecRequest)
	require.Equal(t, http.StatusOK, listGatewayExecResponseRecorder.Code)

	var gatewayListExecutionsResponse ListExecutionsResponse
	require.NoError(t, json.Unmarshal(listGatewayExecResponseRecorder.Body.Bytes(), &gatewayListExecutionsResponse))
	require.Equal(t, 1, gatewayListExecutionsResponse.Count)

	deleteUsersEndpointRequest := httptest.NewRequestWithContext(testCtx, http.MethodDelete, "/v1/_/function/endpoints/"+usersEndpoint.ID.String(), nil)
	deleteUsersEndpointRequest.Header.Set("Authorization", authHeader)
	deleteUsersEndpointResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deleteUsersEndpointResponseRecorder, deleteUsersEndpointRequest)
	require.Equal(t, http.StatusNoContent, deleteUsersEndpointResponseRecorder.Code)

	deleteGatewayEpRequest := httptest.NewRequestWithContext(testCtx, http.MethodDelete, "/v1/_/function/endpoints/"+gatewayEndpoint.ID.String(), nil)
	deleteGatewayEpRequest.Header.Set("Authorization", authHeader)
	deleteGatewayEpResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deleteGatewayEpResponseRecorder, deleteGatewayEpRequest)
	require.Equal(t, http.StatusNoContent, deleteGatewayEpResponseRecorder.Code)
}
