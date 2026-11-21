package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"layr.sh/core"
)

func TestAuthRouterUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)

	// 1. Register routes on routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	gatewayServeMux := http.NewServeMux()
	service.RegisterRoutes(baseRouter, controlPlaneRouter, gatewayServeMux)
	gatewayServeMux.Handle("/v1/_/", controlPlaneRouter.Mux())
	gatewayServeMux.Handle("/v1/", baseRouter.Mux())

	for prefix, router := range map[string]*core.Router{
		"/v1/auth/":   baseRouter,
		"/v1/_/auth/": controlPlaneRouter,
	} {
		for extension, contentType := range map[string]string{"json": "application/json", "yaml": "application/yaml"} {
			t.Run(prefix+extension, func(t *testing.T) {
				responseRecorder := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, prefix+"spec."+extension, nil)
				gatewayServeMux.ServeHTTP(responseRecorder, request)
				assert.Equal(t, http.StatusOK, responseRecorder.Code)
				assert.Equal(t, contentType, responseRecorder.Header().Get("Content-Type"))
				var expectedPayload []byte
				var err error
				if extension == "json" {
					expectedPayload, err = json.Marshal(router.OutputOpenAPISpec())
					expectedPayload = append(expectedPayload, '\n')
				} else {
					expectedPayload, err = yaml.Marshal(router.OutputOpenAPISpec())
				}
				require.NoError(t, err)
				assert.Equal(t, string(expectedPayload), responseRecorder.Body.String())
				assert.Nil(t, router.OutputOpenAPISpec().Paths.Find(prefix+"spec."+extension))
			})
		}
	}

	require.NotNil(t, controlPlaneRouter.OutputOpenAPISpec().Paths.Find("/v1/_/auth/stats"))
	config := DefaultConfig()
	config.OAuthProviders = map[string]OAuthProviderConfig{"google": {Enabled: false}}
	service.configManager.SetMemoryConfig(config)
	for _, path := range []string{"/v1/oauth/authorize", "/v1/oauth/token", "/v1/oauth/userinfo", "/v1/oauth/sign-out", "/v1/oauth/{provider}/authorize", "/v1/oauth/{provider}/callback"} {
		t.Run(path, func(t *testing.T) {
			requestPath := strings.ReplaceAll(path, "{provider}", "google")
			responseRecorder := httptest.NewRecorder()
			baseRouter.Mux().ServeHTTP(responseRecorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, requestPath, nil))
			require.NotEqual(t, http.StatusNotFound, responseRecorder.Code)
			oldResponseRecorder := httptest.NewRecorder()
			baseRouter.Mux().ServeHTTP(oldResponseRecorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, strings.Replace(requestPath, "/v1/oauth", "/v1/auth/oauth", 1), nil))
			require.Equal(t, http.StatusNotFound, oldResponseRecorder.Code)
		})
	}

	// 2. Verify SDK routes naming compliance (snake_case, operation ID namespacing)
	core.AssertRouterRoutesCompliance(t, baseRouter)
	core.AssertRouterRoutesCompliance(t, controlPlaneRouter)
}
