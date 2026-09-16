package data

import (
	"testing"

	"github.com/go-fuego/fuego"
	"layr.sh/core"
)

func TestDataRouterUnit(t *testing.T) {
	service := NewService(nil)

	// 1. Test nil safety
	service.RegisterRoutes(nil, nil)

	isolatedBaseRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(isolatedBaseRouter, nil)

	isolatedControlPlaneRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(nil, isolatedControlPlaneRouter)

	// 2. Test nil handler safety
	nilHandlerService := &Service{}
	nilHandlerBaseRouter := core.NewRouter(fuego.NewServer())
	nilHandlerControlPlaneRouter := core.NewRouter(fuego.NewServer())
	nilHandlerService.RegisterRoutes(nilHandlerBaseRouter, nilHandlerControlPlaneRouter)

	// 3. Register both public and control plane routes on fresh routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(baseRouter, controlPlaneRouter)

	// 4. Inspect generated OpenAPI specifications
	publicOpenAPISpec := baseRouter.OutputOpenAPISpec()
	if publicOpenAPISpec == nil {
		t.Fatal("expected non-nil public OpenAPI specification")
	}
	if publicOpenAPISpec.Paths.Value("/api/v1/data/{schema_name}/{table_name}") == nil {
		t.Fatal("expected /api/v1/data/{schema_name}/{table_name} route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/api/v1/data/{schema_name}/{table_name}/{record_id}") == nil {
		t.Fatal("expected /api/v1/data/{schema_name}/{table_name}/{record_id} route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/api/v1/data/kv/{key}") == nil {
		t.Fatal("expected /api/v1/data/kv/{key} route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/api/v1/data/kv/mget") == nil {
		t.Fatal("expected /api/v1/data/kv/mget route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/api/v1/data/kv/mset") == nil {
		t.Fatal("expected /api/v1/data/kv/mset route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/api/v1/data/kv/increment") == nil {
		t.Fatal("expected /api/v1/data/kv/increment route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/api/v1/graphql") == nil {
		t.Fatal("expected /api/v1/graphql route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/api/v1/realtime") == nil {
		t.Fatal("expected /api/v1/realtime route in public OpenAPI spec")
	}

	controlPlaneOpenAPISpec := controlPlaneRouter.OutputOpenAPISpec()
	if controlPlaneOpenAPISpec == nil {
		t.Fatal("expected non-nil control plane OpenAPI specification")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/api/v1/_/data/config") == nil {
		t.Fatal("expected /api/v1/_/data/config route in control plane OpenAPI spec")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/api/v1/_/data/cache/flush") == nil {
		t.Fatal("expected /api/v1/_/data/cache/flush route in control plane OpenAPI spec")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/api/v1/_/data/cache/invalidate") == nil {
		t.Fatal("expected /api/v1/_/data/cache/invalidate route in control plane OpenAPI spec")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/api/v1/_/data/tables") == nil {
		t.Fatal("expected /api/v1/_/data/tables route in control plane OpenAPI spec")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/api/v1/_/data/sql") == nil {
		t.Fatal("expected /api/v1/_/data/sql route in control plane OpenAPI spec")
	}

	// 5. Verify payload and response struct definitions
	_ = TableRecord{ID: "uuid", Properties: map[string]string{"key": "value"}}
	_ = TableRowsResponse{Data: []TableRecord{{ID: "1"}}}
	_ = TableRowResponse{Data: TableRecord{ID: "1"}}
	_ = InsertRowPayload{Data: []TableRecord{{ID: "1"}}}
	_ = UpdateRowPayload{Data: TableRecord{ID: "1"}}
	_ = KVGetResponse{Key: "k", Value: "v"}
	_ = KVSetRequest{Value: "v", TTL: 60}
	_ = KVSetResponse{Key: "k", Status: "ok", TTL: 60}
	_ = KVMGetRequest{Keys: []string{"k1", "k2"}}
	_ = KVMGetResponse{Values: map[string]string{"k1": "v1"}}
	_ = KVIncrementRequest{Key: "k", TTL: 60}
	_ = KVIncrementResponse{Key: "k", Value: 42}
	_ = ToggleTableRLSRequest{Action: "enable"}
	_ = ListTablesResponse{Tables: []TableSummary{{Name: "t", Schema: "s"}}}
	_ = ExecuteSQLRequest{Query: "SELECT 1;"}
}
