package data

import (
	"testing"

	"github.com/go-fuego/fuego"
	"layr.sh/core"
)

func TestDataRouterUnit(t *testing.T) {
	service := NewService(core.NewTestKernel(nil))

	// 1. Register routes on routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(baseRouter, controlPlaneRouter)

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
