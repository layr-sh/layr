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
	if publicOpenAPISpec.Paths.Value("/v1/data/{schema_name}/{table_name}") == nil {
		t.Fatal("expected /v1/data/{schema_name}/{table_name} route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/v1/data/{schema_name}/{table_name}/{record_id}") == nil {
		t.Fatal("expected /v1/data/{schema_name}/{table_name}/{record_id} route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/v1/data/kv/{key}") == nil {
		t.Fatal("expected /v1/data/kv/{key} route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/v1/data/kv/mget") == nil {
		t.Fatal("expected /v1/data/kv/mget route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/v1/data/kv/mset") == nil {
		t.Fatal("expected /v1/data/kv/mset route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/v1/data/kv/increment") == nil {
		t.Fatal("expected /v1/data/kv/increment route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/v1/graphql") == nil {
		t.Fatal("expected /v1/graphql route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/v1/realtime") == nil {
		t.Fatal("expected /v1/realtime route in public OpenAPI spec")
	}

	controlPlaneOpenAPISpec := controlPlaneRouter.OutputOpenAPISpec()
	if controlPlaneOpenAPISpec == nil {
		t.Fatal("expected non-nil control plane OpenAPI specification")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/v1/_/data/config") == nil {
		t.Fatal("expected /v1/_/data/config route in control plane OpenAPI spec")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/v1/_/data/cache/flush") == nil {
		t.Fatal("expected /v1/_/data/cache/flush route in control plane OpenAPI spec")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/v1/_/data/cache/invalidate") == nil {
		t.Fatal("expected /v1/_/data/cache/invalidate route in control plane OpenAPI spec")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/v1/_/data/tables") == nil {
		t.Fatal("expected /v1/_/data/tables route in control plane OpenAPI spec")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/v1/_/data/sql") == nil {
		t.Fatal("expected /v1/_/data/sql route in control plane OpenAPI spec")
	}

	// 5. Verify payload and response struct definitions
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
	_ = ListTablesResponse{Tables: []Table{{Name: "t", Schema: "s"}}}
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
