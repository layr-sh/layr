package image

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"layr.sh/core"
)

func TestImageRouterUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)

	// 1. Register both public and control plane routes on fresh routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	gatewayServeMux := http.NewServeMux()
	service.RegisterRoutes(baseRouter, controlPlaneRouter, gatewayServeMux)
	gatewayServeMux.Handle("/v1/_/", controlPlaneRouter.Mux())
	gatewayServeMux.Handle("/v1/", baseRouter.Mux())

	for prefix, router := range map[string]*core.Router{
		"/v1/image/":   baseRouter,
		"/v1/_/image/": controlPlaneRouter,
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

	// 2. Verify SDK routes naming compliance (snake_case, operation ID namespacing)
	core.AssertRouterRoutesCompliance(t, baseRouter)
	core.AssertRouterRoutesCompliance(t, controlPlaneRouter)
}
