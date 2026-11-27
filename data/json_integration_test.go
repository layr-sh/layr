package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func TestDataExactNumericRoundTripIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	ctx := context.Background()
	service := NewService(kernel)
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer service.Stop()
	if _, err := kernel.DB().Exec(ctx, `CREATE TABLE public.review_numeric(id bigint PRIMARY KEY, amount numeric, payload jsonb, integer_values bigint[], decimal_values numeric[]); CREATE FUNCTION public.review_numeric_echo(value numeric) RETURNS numeric LANGUAGE sql AS 'SELECT value'`); err != nil {
		t.Fatal(err)
	}
	const exactInteger = "9007199254740993"
	const exactDecimal = "1234567890.123456789"
	payload := ` [ {"id":` + exactInteger + `,"amount":` + exactDecimal + `,"payload":{"integer":` + exactInteger + `,"decimal":` + exactDecimal + `},"integer_values":[` + exactInteger + `],"decimal_values":[` + exactDecimal + `]} ] `
	createRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/data/public/review_numeric", strings.NewReader(payload))
	createResponseRecorder := httptest.NewRecorder()
	service.BaseHandler().handleCreateRecord(createResponseRecorder, createRequest)
	if createResponseRecorder.Code != http.StatusCreated || !bytes.Contains(createResponseRecorder.Body.Bytes(), []byte(exactInteger)) || !bytes.Contains(createResponseRecorder.Body.Bytes(), []byte(exactDecimal)) {
		t.Fatalf("inexact insert: %d %s", createResponseRecorder.Code, createResponseRecorder.Body.String())
	}
	updateRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/data/public/review_numeric/"+exactInteger, strings.NewReader(`{"amount":`+exactDecimal+`}`))
	updateResponseRecorder := httptest.NewRecorder()
	service.BaseHandler().handleUpdateRecord(updateResponseRecorder, updateRequest)
	if updateResponseRecorder.Code != http.StatusOK || !bytes.Contains(updateResponseRecorder.Body.Bytes(), []byte(exactDecimal)) {
		t.Fatalf("inexact update: %d %s", updateResponseRecorder.Code, updateResponseRecorder.Body.String())
	}
	gqlRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/graphql", strings.NewReader(`{"query":"query($id: bigint!,$limit: Int!){ review_numeric(where:{id:{_eq:$id}},limit:$limit){ id amount payload } }","variables":{"id":`+exactInteger+`,"limit":1}}`))
	gqlResponseRecorder := httptest.NewRecorder()
	service.BaseHandler().handleExecuteGraphQL(gqlResponseRecorder, gqlRequest)
	if gqlResponseRecorder.Code != http.StatusOK || !bytes.Contains(gqlResponseRecorder.Body.Bytes(), []byte(exactInteger)) || !bytes.Contains(gqlResponseRecorder.Body.Bytes(), []byte(exactDecimal)) {
		t.Fatalf("inexact GraphQL variables/result: %d %s", gqlResponseRecorder.Code, gqlResponseRecorder.Body.String())
	}
	rpcRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/data/public/rpc/review_numeric_echo", strings.NewReader(`{"value":`+exactDecimal+`}`))
	rpcResponseRecorder := httptest.NewRecorder()
	service.BaseHandler().handleExecuteFunction(rpcResponseRecorder, rpcRequest)
	if rpcResponseRecorder.Code != http.StatusOK || !bytes.Contains(rpcResponseRecorder.Body.Bytes(), []byte(exactDecimal)) {
		t.Fatalf("inexact RPC: %d %s", rpcResponseRecorder.Code, rpcResponseRecorder.Body.String())
	}
	executeSQLResponse, err := service.DDLEngine().ExecuteSQL(ctx, `SELECT payload FROM public.review_numeric`)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(executeSQLResponse.Rows)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), exactInteger) {
		t.Fatal("scratchpad JSON lost precision")
	}
}
