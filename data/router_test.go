package data

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

func TestDataRouterUnit(t *testing.T) {
	service := NewService(core.NewTestKernel(nil))

	// 1. Register routes on routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	gatewayServeMux := http.NewServeMux()
	service.RegisterRoutes(baseRouter, controlPlaneRouter, gatewayServeMux)
	gatewayServeMux.Handle("/v1/_/", controlPlaneRouter.Mux())
	gatewayServeMux.Handle("/v1/", baseRouter.Mux())

	for prefix, router := range map[string]*core.Router{
		"/v1/data/":   baseRouter,
		"/v1/_/data/": controlPlaneRouter,
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

	// 6. Verify payload and response struct definitions
	_ = Record{ID: "uuid", Properties: map[string]string{"key": "value"}}
	_ = ListRecordsResponse{Data: []Record{{ID: "1"}}}
	_ = GetRecordResponse{Data: Record{ID: "1"}}
	_ = CreateRecordInput{Data: []Record{{ID: "1"}}}
	_ = UpdateRecordInput{Data: Record{ID: "1"}}
	_ = CreateRecordResponse{Data: []Record{{ID: "1"}}}
	_ = UpdateRecordResponse{Data: Record{ID: "1"}}
	_ = GetKVResponse{Key: "k", Value: "v"}
	_ = SetKVInput{Value: "v", TTL: 60}
	_ = SetKVResponse{Key: "k", Status: "ok", TTL: 60}
	_ = GetMultipleKVInput{Keys: []string{"k1", "k2"}}
	_ = GetMultipleKVResponse{Values: map[string]string{"k1": "v1"}}
	_ = IncrementKVInput{Key: "k", TTL: 60}
	_ = IncrementKVResponse{Key: "k", Value: 42}
	_ = TouchKVInput{TTL: 60}
	_ = TouchKVResponse{Key: "k", Status: "ok", TTL: 60}
	_ = ExecuteGraphQLInput{Query: "{ test }"}
	_ = ExecuteGraphQLResponse{Data: "test"}
	_ = ToggleRLSInput{Action: "enable"}
	_ = ToggleRLSResponse{Status: "enabled"}
	_ = ListTablesResponse{Data: []Table{{Name: "t", Schema: "s"}}}
	_ = Table{Name: "t", Schema: "s"}
	_ = Column{Name: "c", Type: "text"}
	_ = Index{IndexName: "idx"}
	_ = Policy{Name: "p"}
	_ = ExecuteSQLInput{Query: "SELECT 1;"}
	_ = ExecuteSQLResponse{}
	_ = CreateTableInput{Name: "t"}
	_ = CreateTableResponse{Status: "created"}
	_ = DeleteTableResponse{Status: "deleted"}
	_ = CreateColumnResponse{Status: "created"}
	_ = UpdateColumnInput{}
	_ = UpdateColumnResponse{Status: "updated"}
	_ = DeleteColumnResponse{Status: "deleted"}
	_ = CreateIndexInput{}
	_ = CreateIndexResponse{Status: "created"}
	_ = DeleteIndexResponse{Status: "deleted"}
	_ = CreatePolicyInput{}
	_ = CreatePolicyResponse{Status: "created"}
	_ = DeletePolicyResponse{Status: "deleted"}
	_ = InvalidateCacheInput{}
	_ = InvalidateCacheResponse{}
}
