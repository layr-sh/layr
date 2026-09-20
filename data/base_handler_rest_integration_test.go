package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"layr.sh/core"
	"layr.sh/data/rest"
)

func TestDataBaseHandlerRESTLifecycleIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(db)
	eventBus := core.NewEventBus(db, nil)
	defer eventBus.Close()

	service := NewService(db)
	inMemoryKVStore := newInMemoryKVStore()
	service.SetKVStore(inMemoryKVStore)
	service.SetServiceAccountManager(serviceAccountManager)
	service.SetEventBus(eventBus)
	_ = service.Start(ctx)
	defer func() { _ = service.Stop() }()

	baseHandler := service.BaseHandler()

	// 1. Create a table using control plane handler
	createTableInput := CreateTableInput{
		Name: "items",
		Columns: []Column{
			{Name: "id", Type: "uuid", IsPrimaryKey: true},
			{Name: "name", Type: "text", IsNullable: false},
			{Name: "status", Type: "varchar(32)"},
		},
	}
	rawTableJSONBytes, _ := json.Marshal(createTableInput)
	createTableHTTPRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/data/tables/public", bytes.NewReader(rawTableJSONBytes))
	createTableHTTPRequest.SetPathValue("schema_name", "public")
	createTableResponseRecorder := httptest.NewRecorder()
	service.GetControlPlaneHandler().handleCreateTable(createTableResponseRecorder, createTableHTTPRequest)
	assert.Equal(t, http.StatusCreated, createTableResponseRecorder.Code)

	// 2. Insert single record with representation
	insertPayload := map[string]any{"name": "Item 1", "status": "active"}
	insertBytes, _ := json.Marshal(insertPayload)
	postRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/items", bytes.NewReader(insertBytes))
	postRequest.SetPathValue("schema_name", "public")
	postRequest.SetPathValue("table_name", "items")
	postRequest.Header.Set("Prefer", "return=representation")
	postResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(postResponseRecorder, postRequest)
	assert.Equal(t, http.StatusCreated, postResponseRecorder.Code)

	var createdItem map[string]any
	_ = json.NewDecoder(postResponseRecorder.Body).Decode(&createdItem)
	itemID, ok := createdItem["id"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, itemID)

	// 3. Insert bulk records with minimal
	bulkPayload := []map[string]any{
		{"name": "Item 2", "status": "pending"},
		{"name": "Item 3", "status": "active"},
	}
	bulkBytes, _ := json.Marshal(bulkPayload)
	bulkPostRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/items", bytes.NewReader(bulkBytes))
	bulkPostRequest.SetPathValue("schema_name", "public")
	bulkPostRequest.SetPathValue("table_name", "items")
	bulkPostRequest.Header.Set("Prefer", "return=minimal")
	bulkPostResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(bulkPostResponseRecorder, bulkPostRequest)
	assert.Equal(t, http.StatusNoContent, bulkPostResponseRecorder.Code)

	// 4. List records with exact count and caching
	listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items?count=exact&order=name.asc&cache_ttl=60", nil)
	listRequest.SetPathValue("schema_name", "public")
	listRequest.SetPathValue("table_name", "items")
	listResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListRecords(listResponseRecorder, listRequest)
	assert.Equal(t, http.StatusOK, listResponseRecorder.Code)
	assert.Equal(t, "3", listResponseRecorder.Header().Get("X-Total-Count"))
	assert.Equal(t, "MISS", listResponseRecorder.Header().Get("X-Layr-Cache"))

	// List again -> HIT
	secondListResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListRecords(secondListResponseRecorder, listRequest)
	assert.Equal(t, http.StatusOK, secondListResponseRecorder.Code)
	assert.Equal(t, "HIT", secondListResponseRecorder.Header().Get("X-Layr-Cache"))

	// List when MaxCachedQueries exceeded skips caching
	tableConfig := service.GetConfigManager().Get()
	tableConfig.Cache.MaxCachedQueries = 2
	service.GetConfigManager().SetMemoryConfig(tableConfig)
	_ = inMemoryKVStore.Set(ctx, "cache:query_count", "5", 0)
	capListRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items?limit=2", nil)
	capListRequest.SetPathValue("schema_name", "public")
	capListRequest.SetPathValue("table_name", "items")
	capListRequest.Header.Set("X-Layr-Cache-TTL", "60")
	capListResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListRecords(capListResponseRecorder, capListRequest)
	assert.Equal(t, http.StatusOK, capListResponseRecorder.Code)
	tableConfig.Cache.MaxCachedQueries = 10000
	service.GetConfigManager().SetMemoryConfig(tableConfig)

	// 5. Get record by ID
	getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items/"+itemID, nil)
	getRequest.SetPathValue("schema_name", "public")
	getRequest.SetPathValue("table_name", "items")
	getRequest.SetPathValue("record_id", itemID)
	getResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetRecord(getResponseRecorder, getRequest)
	assert.Equal(t, http.StatusOK, getResponseRecorder.Code)

	// Get record with minimal
	getMinimalRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items/"+itemID, nil)
	getMinimalRequest.SetPathValue("schema_name", "public")
	getMinimalRequest.SetPathValue("table_name", "items")
	getMinimalRequest.SetPathValue("record_id", itemID)
	getMinimalRequest.Header.Set("Prefer", "return=minimal")
	getMinimalResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetRecord(getMinimalResponseRecorder, getMinimalRequest)
	assert.Equal(t, http.StatusNoContent, getMinimalResponseRecorder.Code)

	// Get non-existent -> 404
	getBadRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items/018f0000-0000-7000-8000-000000000000", nil)
	getBadRequest.SetPathValue("schema_name", "public")
	getBadRequest.SetPathValue("table_name", "items")
	getBadRequest.SetPathValue("record_id", "018f0000-0000-7000-8000-000000000000")
	getBadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetRecord(getBadResponseRecorder, getBadRequest)
	assert.Equal(t, http.StatusNotFound, getBadResponseRecorder.Code)

	// 6. Update record with representation
	patchPayload := map[string]any{"status": "archived"}
	patchBytes, _ := json.Marshal(patchPayload)
	patchRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/data/public/items/"+itemID, bytes.NewReader(patchBytes))
	patchRequest.SetPathValue("schema_name", "public")
	patchRequest.SetPathValue("table_name", "items")
	patchRequest.SetPathValue("record_id", itemID)
	patchResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateRecord(patchResponseRecorder, patchRequest)
	assert.Equal(t, http.StatusOK, patchResponseRecorder.Code)
	assert.Contains(t, patchResponseRecorder.Body.String(), "archived")

	// Update record with minimal
	patchMinimalRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/data/public/items/"+itemID, bytes.NewReader(patchBytes))
	patchMinimalRequest.SetPathValue("schema_name", "public")
	patchMinimalRequest.SetPathValue("table_name", "items")
	patchMinimalRequest.SetPathValue("record_id", itemID)
	patchMinimalRequest.Header.Set("Prefer", "return=minimal")
	patchMinimalResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateRecord(patchMinimalResponseRecorder, patchMinimalRequest)
	assert.Equal(t, http.StatusNoContent, patchMinimalResponseRecorder.Code)

	// Update non-existent -> 404
	patchBadRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/data/public/items/018f0000-0000-7000-8000-000000000000", bytes.NewReader(patchBytes))
	patchBadRequest.SetPathValue("schema_name", "public")
	patchBadRequest.SetPathValue("table_name", "items")
	patchBadRequest.SetPathValue("record_id", "018f0000-0000-7000-8000-000000000000")
	patchBadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateRecord(patchBadResponseRecorder, patchBadRequest)
	assert.Equal(t, http.StatusNotFound, patchBadResponseRecorder.Code)

	// 7. Delete record
	deleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/data/public/items/"+itemID, nil)
	deleteRequest.SetPathValue("schema_name", "public")
	deleteRequest.SetPathValue("table_name", "items")
	deleteRequest.SetPathValue("record_id", itemID)
	deleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteRecord(deleteResponseRecorder, deleteRequest)
	assert.Equal(t, http.StatusNoContent, deleteResponseRecorder.Code)

	// Delete non-existent -> 404
	deleteBadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteRecord(deleteBadResponseRecorder, deleteRequest)
	assert.Equal(t, http.StatusNotFound, deleteBadResponseRecorder.Code)

	// 10. Bulk insert with return=representation (covers line 315)
	bulkRepPayload := []map[string]any{
		{"name": "Bulk Rep 1", "status": "active"},
		{"name": "Bulk Rep 2", "status": "active"},
	}
	bulkRepBytes, _ := json.Marshal(bulkRepPayload)
	bulkRepRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/items", bytes.NewReader(bulkRepBytes))
	bulkRepRequest.SetPathValue("schema_name", "public")
	bulkRepRequest.SetPathValue("table_name", "items")
	bulkRepRequest.Header.Set("Prefer", "return=representation")
	bulkRepResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(bulkRepResponseRecorder, bulkRepRequest)
	assert.Equal(t, http.StatusCreated, bulkRepResponseRecorder.Code)
	assert.Contains(t, bulkRepResponseRecorder.Body.String(), "Bulk Rep 1")

	// 11. CreateRecordInput format insert
	createRecordInput := CreateRecordInput{
		Data: []Record{
			{
				ID:         "018f0000-0000-7000-8000-000000000099",
				Properties: map[string]string{"name": "Payload Item", "status": "active"},
			},
		},
	}
	payloadBytes, _ := json.Marshal(createRecordInput)
	insertPayloadRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/items", bytes.NewReader(payloadBytes))
	insertPayloadRequest.SetPathValue("schema_name", "public")
	insertPayloadRequest.SetPathValue("table_name", "items")
	insertPayloadRequest.Header.Set("Prefer", "return=representation")
	insertPayloadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(insertPayloadResponseRecorder, insertPayloadRequest)
	assert.Equal(t, http.StatusCreated, insertPayloadResponseRecorder.Code)

	// 12. UpdateRecordInput format update
	updateRecordInput := UpdateRecordInput{
		Data: Record{
			Properties: map[string]string{"status": "payload_updated"},
		},
	}
	updatePayloadBytes, _ := json.Marshal(updateRecordInput)
	updatePayloadRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/data/public/items/018f0000-0000-7000-8000-000000000099", bytes.NewReader(updatePayloadBytes))
	updatePayloadRequest.SetPathValue("schema_name", "public")
	updatePayloadRequest.SetPathValue("table_name", "items")
	updatePayloadRequest.SetPathValue("record_id", "018f0000-0000-7000-8000-000000000099")
	updatePayloadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateRecord(updatePayloadResponseRecorder, updatePayloadRequest)
	assert.Equal(t, http.StatusOK, updatePayloadResponseRecorder.Code)

	// 13. Cache TTL via header
	listHeaderRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items", nil)
	listHeaderRequest.SetPathValue("schema_name", "public")
	listHeaderRequest.SetPathValue("table_name", "items")
	listHeaderRequest.Header.Set("X-Layr-Cache-TTL", "60")
	listHeaderRequest.Header.Set("X-Layr-Cache-Key-Suffix", "hdr-suf")
	listHeaderResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListRecords(listHeaderResponseRecorder, listHeaderRequest)
	assert.Equal(t, http.StatusOK, listHeaderResponseRecorder.Code)

	// 14. Service account RLS bypass
	serviceAccount, _ := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Bypass SA",
		Scopes: []string{"data:query.read", "data:query.write"},
	})
	serviceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items", nil)
	serviceAccountRequest.SetPathValue("schema_name", "public")
	serviceAccountRequest.SetPathValue("table_name", "items")
	serviceAccountRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	assert.True(t, baseHandler.isRLSBypassed(serviceAccountRequest, "data:query.read"))

	// SA without scope
	noScopeServiceAccount, _ := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "No Read Scope",
		Scopes: []string{"auth:users.read"},
	})
	serviceAccountNoScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items", nil)
	serviceAccountNoScopeRequest.Header.Set("Authorization", "Bearer "+noScopeServiceAccount.SecretKey)
	assert.False(t, baseHandler.isRLSBypassed(serviceAccountNoScopeRequest, "data:query.read"))

	// SA invalid key
	serviceAccountBadRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items", nil)
	serviceAccountBadRequest.Header.Set("Authorization", "Bearer invalid_secret_key_12345678901234567890")
	assert.False(t, baseHandler.isRLSBypassed(serviceAccountBadRequest, "data:query.read"))

	// 15. resolveTransaction rollback
	tx, err := db.Begin(ctx)
	assert.NoError(t, err)
	assert.Error(t, baseHandler.resolveTransaction(ctx, tx, errors.New("rollback test")))

	// 16. Canceled context on Begin(ctx) across operations
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	canceledListRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "/api/v1/data/public/items", nil)
	canceledListRequest.SetPathValue("schema_name", "public")
	canceledListRequest.SetPathValue("table_name", "items")
	canceledListResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListRecords(canceledListResponseRecorder, canceledListRequest)
	assert.Equal(t, http.StatusInternalServerError, canceledListResponseRecorder.Code)

	canceledGetRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "/api/v1/data/public/items/123", nil)
	canceledGetRequest.SetPathValue("schema_name", "public")
	canceledGetRequest.SetPathValue("table_name", "items")
	canceledGetRequest.SetPathValue("record_id", "123")
	canceledGetResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetRecord(canceledGetResponseRecorder, canceledGetRequest)
	assert.Equal(t, http.StatusInternalServerError, canceledGetResponseRecorder.Code)

	canceledCreateRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPost, "/api/v1/data/public/items", bytes.NewReader([]byte(`{"name":"test"}`)))
	canceledCreateRequest.SetPathValue("schema_name", "public")
	canceledCreateRequest.SetPathValue("table_name", "items")
	canceledCreateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(canceledCreateResponseRecorder, canceledCreateRequest)
	assert.Equal(t, http.StatusInternalServerError, canceledCreateResponseRecorder.Code)

	canceledUpdateRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPatch, "/api/v1/data/public/items/123", bytes.NewReader([]byte(`{"name":"test"}`)))
	canceledUpdateRequest.SetPathValue("schema_name", "public")
	canceledUpdateRequest.SetPathValue("table_name", "items")
	canceledUpdateRequest.SetPathValue("record_id", "123")
	canceledUpdateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateRecord(canceledUpdateResponseRecorder, canceledUpdateRequest)
	assert.Equal(t, http.StatusInternalServerError, canceledUpdateResponseRecorder.Code)

	canceledDeleteRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodDelete, "/api/v1/data/public/items/123", nil)
	canceledDeleteRequest.SetPathValue("schema_name", "public")
	canceledDeleteRequest.SetPathValue("table_name", "items")
	canceledDeleteRequest.SetPathValue("record_id", "123")
	canceledDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteRecord(canceledDeleteResponseRecorder, canceledDeleteRequest)
	assert.Equal(t, http.StatusInternalServerError, canceledDeleteResponseRecorder.Code)

	// 17. Empty primary key fallback ("" -> "id") in table metadata
	_, _ = db.Exec(ctx, `CREATE TABLE public.no_pk_items (id uuid primary key default uuidv7(), name text);`)
	baseHandler.SetTableMetadata(rest.TableMetadata{
		Schema:     "public",
		Table:      "no_pk_items",
		PrimaryKey: "",
		Columns:    []string{"id", "name"},
	})

	noPKCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/no_pk_items", bytes.NewReader([]byte(`{"name":"No PK Item"}`)))
	noPKCreateRequest.SetPathValue("schema_name", "public")
	noPKCreateRequest.SetPathValue("table_name", "no_pk_items")
	noPKCreateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(noPKCreateResponseRecorder, noPKCreateRequest)
	assert.Equal(t, http.StatusCreated, noPKCreateResponseRecorder.Code)

	var createdNoPK map[string]any
	_ = json.NewDecoder(noPKCreateResponseRecorder.Body).Decode(&createdNoPK)
	noPKID := createdNoPK["id"].(string)

	noPKGetRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/no_pk_items/"+noPKID, nil)
	noPKGetRequest.SetPathValue("schema_name", "public")
	noPKGetRequest.SetPathValue("table_name", "no_pk_items")
	noPKGetRequest.SetPathValue("record_id", noPKID)
	noPKGetResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetRecord(noPKGetResponseRecorder, noPKGetRequest)
	assert.Equal(t, http.StatusOK, noPKGetResponseRecorder.Code)

	noPKUpdateRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/data/public/no_pk_items/"+noPKID, bytes.NewReader([]byte(`{"name":"Updated No PK"}`)))
	noPKUpdateRequest.SetPathValue("schema_name", "public")
	noPKUpdateRequest.SetPathValue("table_name", "no_pk_items")
	noPKUpdateRequest.SetPathValue("record_id", noPKID)
	noPKUpdateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateRecord(noPKUpdateResponseRecorder, noPKUpdateRequest)
	assert.Equal(t, http.StatusOK, noPKUpdateResponseRecorder.Code)

	noPKDeleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/data/public/no_pk_items/"+noPKID, nil)
	noPKDeleteRequest.SetPathValue("schema_name", "public")
	noPKDeleteRequest.SetPathValue("table_name", "no_pk_items")
	noPKDeleteRequest.SetPathValue("record_id", noPKID)
	noPKDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteRecord(noPKDeleteResponseRecorder, noPKDeleteRequest)
	assert.Equal(t, http.StatusNoContent, noPKDeleteResponseRecorder.Code)

	// 18. Database execution errors on invalid columns / functions
	badSelectRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items?select=non_existent_column", nil)
	badSelectRequest.SetPathValue("schema_name", "public")
	badSelectRequest.SetPathValue("table_name", "items")
	badSelectResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListRecords(badSelectResponseRecorder, badSelectRequest)
	assert.Equal(t, http.StatusBadRequest, badSelectResponseRecorder.Code)

	badGetColumnRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items/123?select=non_existent_column", nil)
	badGetColumnRequest.SetPathValue("schema_name", "public")
	badGetColumnRequest.SetPathValue("table_name", "items")
	badGetColumnRequest.SetPathValue("record_id", "123")
	badGetColumnResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetRecord(badGetColumnResponseRecorder, badGetColumnRequest)
	assert.Equal(t, http.StatusBadRequest, badGetColumnResponseRecorder.Code)

	badInsertColumnRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/items", bytes.NewReader([]byte(`{"non_existent_column":"dummy"}`)))
	badInsertColumnRequest.SetPathValue("schema_name", "public")
	badInsertColumnRequest.SetPathValue("table_name", "items")
	badInsertColumnResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(badInsertColumnResponseRecorder, badInsertColumnRequest)
	assert.Equal(t, http.StatusBadRequest, badInsertColumnResponseRecorder.Code)

	badUpdateColumnRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/data/public/items/123", bytes.NewReader([]byte(`{"non_existent_column":"dummy"}`)))
	badUpdateColumnRequest.SetPathValue("schema_name", "public")
	badUpdateColumnRequest.SetPathValue("table_name", "items")
	badUpdateColumnRequest.SetPathValue("record_id", "123")
	badUpdateColumnResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateRecord(badUpdateColumnResponseRecorder, badUpdateColumnRequest)
	assert.Equal(t, http.StatusBadRequest, badUpdateColumnResponseRecorder.Code)

	// 19. Builder errors: invalid column identifier in update & insert payloads
	builderBadInsertRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/items", bytes.NewReader([]byte(`{"bad-identifier!":"dummy"}`)))
	builderBadInsertRequest.SetPathValue("schema_name", "public")
	builderBadInsertRequest.SetPathValue("table_name", "items")
	builderBadInsertResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(builderBadInsertResponseRecorder, builderBadInsertRequest)
	assert.Equal(t, http.StatusBadRequest, builderBadInsertResponseRecorder.Code)

	builderBadUpdateRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/data/public/items/123", bytes.NewReader([]byte(`{"bad-identifier!":"dummy"}`)))
	builderBadUpdateRequest.SetPathValue("schema_name", "public")
	builderBadUpdateRequest.SetPathValue("table_name", "items")
	builderBadUpdateRequest.SetPathValue("record_id", "123")
	builderBadUpdateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateRecord(builderBadUpdateResponseRecorder, builderBadUpdateRequest)
	assert.Equal(t, http.StatusBadRequest, builderBadUpdateResponseRecorder.Code)

	// Builder error: bad primary key identifier in delete
	baseHandler.SetTableMetadata(rest.TableMetadata{
		Schema:     "public",
		Table:      "items_bad_pk",
		PrimaryKey: "bad-pk-identifier!",
	})
	badPKDeleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/data/public/items_bad_pk/123", nil)
	badPKDeleteRequest.SetPathValue("schema_name", "public")
	badPKDeleteRequest.SetPathValue("table_name", "items_bad_pk")
	badPKDeleteRequest.SetPathValue("record_id", "123")
	badPKDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteRecord(badPKDeleteResponseRecorder, badPKDeleteRequest)
	assert.Equal(t, http.StatusBadRequest, badPKDeleteResponseRecorder.Code)

	// 20. Database execution error on delete: foreign key restrict violation
	_, _ = db.Exec(ctx, `
		CREATE TABLE public.parent_items (id uuid primary key default uuidv7(), name text);
		CREATE TABLE public.child_items (id uuid primary key default uuidv7(), parent_id uuid references public.parent_items(id) on delete restrict);
		INSERT INTO public.parent_items (id, name) VALUES ('018f0000-0000-7000-8000-000000000001', 'Parent');
		INSERT INTO public.child_items (parent_id) VALUES ('018f0000-0000-7000-8000-000000000001');
	`)
	deleteForeignKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/data/public/parent_items/018f0000-0000-7000-8000-000000000001", nil)
	deleteForeignKeyRequest.SetPathValue("schema_name", "public")
	deleteForeignKeyRequest.SetPathValue("table_name", "parent_items")
	deleteForeignKeyRequest.SetPathValue("record_id", "018f0000-0000-7000-8000-000000000001")
	deleteForeignKeyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteRecord(deleteForeignKeyResponseRecorder, deleteForeignKeyRequest)
	assert.Equal(t, http.StatusConflict, deleteForeignKeyResponseRecorder.Code)

	// 21. Count exact on zero results (end < offset)
	emptyCountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/items?count=exact&status=eq.non_existent_status_12345", nil)
	emptyCountRequest.SetPathValue("schema_name", "public")
	emptyCountRequest.SetPathValue("table_name", "items")
	emptyCountResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListRecords(emptyCountResponseRecorder, emptyCountRequest)
	assert.Equal(t, http.StatusOK, emptyCountResponseRecorder.Code)
	assert.Equal(t, "0", emptyCountResponseRecorder.Header().Get("X-Total-Count"))

	// 22. JSONB and bytea columns in scanRowsToJSONMaps
	_, err = db.Exec(ctx, `
		CREATE TABLE public.typed_data (
			id uuid primary key default uuidv7(),
			payload jsonb,
			raw_data bytea
		);
	`)
	assert.NoError(t, err)
	typedInsertPayload := map[string]any{
		"payload":  map[string]any{"key": "value"},
		"raw_data": "\\xdeadbeef",
	}
	typedInsertBytes, _ := json.Marshal(typedInsertPayload)
	typedInsertRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/typed_data", bytes.NewReader(typedInsertBytes))
	typedInsertRequest.SetPathValue("schema_name", "public")
	typedInsertRequest.SetPathValue("table_name", "typed_data")
	typedInsertResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(typedInsertResponseRecorder, typedInsertRequest)
	assert.Equal(t, http.StatusCreated, typedInsertResponseRecorder.Code)

	var createdTyped map[string]any
	_ = json.NewDecoder(typedInsertResponseRecorder.Body).Decode(&createdTyped)
	typedID := createdTyped["id"].(string)

	typedGetRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/typed_data/"+typedID, nil)
	typedGetRequest.SetPathValue("schema_name", "public")
	typedGetRequest.SetPathValue("table_name", "typed_data")
	typedGetRequest.SetPathValue("record_id", typedID)
	typedGetResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetRecord(typedGetResponseRecorder, typedGetRequest)
	assert.Equal(t, http.StatusOK, typedGetResponseRecorder.Code)

	// 23. OnConflict upsert support
	_, err = db.Exec(ctx, `
		CREATE TABLE public.inventory (
			id uuid primary key default uuidv7(),
			sku text unique,
			qty int
		);
	`)
	assert.NoError(t, err)
	invItem := map[string]any{"sku": "SKU001", "qty": 10}
	invBytes, _ := json.Marshal(invItem)
	invRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/inventory", bytes.NewReader(invBytes))
	invRequest.SetPathValue("schema_name", "public")
	invRequest.SetPathValue("table_name", "inventory")
	invResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(invResponseRecorder, invRequest)
	assert.Equal(t, http.StatusCreated, invResponseRecorder.Code)

	// Upsert with on_conflict
	invUpsertItem := map[string]any{"sku": "SKU001", "qty": 20}
	invUpsertBytes, _ := json.Marshal(invUpsertItem)
	invUpsertRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/inventory?on_conflict=sku", bytes.NewReader(invUpsertBytes))
	invUpsertRequest.SetPathValue("schema_name", "public")
	invUpsertRequest.SetPathValue("table_name", "inventory")
	invUpsertResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(invUpsertResponseRecorder, invUpsertRequest)
	assert.Equal(t, http.StatusCreated, invUpsertResponseRecorder.Code)

	// 24. Deferred constraint commit failures in Create, Update, Delete
	_, err = db.Exec(ctx, `
		CREATE TABLE public.defer_items (
			id uuid primary key default uuidv7(),
			code text,
			constraint defer_code_uniq unique (code) deferrable initially deferred
		);
		CREATE TABLE public.defer_parent (id uuid primary key default uuidv7());
		CREATE TABLE public.defer_child (
			id uuid primary key default uuidv7(),
			parent_id uuid references public.defer_parent(id) deferrable initially deferred
		);
	`)
	assert.NoError(t, err)

	// 25a. CreateRecords with deferred constraint violation fails at commit
	deferCreatePayload := []map[string]any{
		{"code": "DUP_CODE_BATCH"},
		{"code": "DUP_CODE_BATCH"},
	}
	deferCreateBytes, _ := json.Marshal(deferCreatePayload)
	deferCreateRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/defer_items", bytes.NewReader(deferCreateBytes))
	deferCreateRequest.SetPathValue("schema_name", "public")
	deferCreateRequest.SetPathValue("table_name", "defer_items")
	deferCreateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleCreateRecord(deferCreateResponseRecorder, deferCreateRequest)
	assert.Equal(t, http.StatusConflict, deferCreateResponseRecorder.Code)

	// Insert distinct rows for update test
	_, err = db.Exec(ctx, `
		INSERT INTO public.defer_items (id, code) VALUES
			('018f0000-0000-7000-8000-000000000010', 'CODE_A'),
			('018f0000-0000-7000-8000-000000000020', 'CODE_B');
	`)
	assert.NoError(t, err)

	// 25b. UpdateRecord deferred constraint collision fails at commit
	deferUpdateRequest := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/v1/data/public/defer_items/018f0000-0000-7000-8000-000000000020", bytes.NewReader([]byte(`{"code":"CODE_A"}`)))
	deferUpdateRequest.SetPathValue("schema_name", "public")
	deferUpdateRequest.SetPathValue("table_name", "defer_items")
	deferUpdateRequest.SetPathValue("record_id", "018f0000-0000-7000-8000-000000000020")
	deferUpdateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateRecord(deferUpdateResponseRecorder, deferUpdateRequest)
	assert.Equal(t, http.StatusConflict, deferUpdateResponseRecorder.Code)

	// 25c. DeleteRecord deferred foreign key violation fails at commit
	_, err = db.Exec(ctx, `
		INSERT INTO public.defer_parent (id) VALUES ('018f0000-0000-7000-8000-000000000030');
		INSERT INTO public.defer_child (parent_id) VALUES ('018f0000-0000-7000-8000-000000000030');
	`)
	assert.NoError(t, err)
	deferDeleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/data/public/defer_parent/018f0000-0000-7000-8000-000000000030", nil)
	deferDeleteRequest.SetPathValue("schema_name", "public")
	deferDeleteRequest.SetPathValue("table_name", "defer_parent")
	deferDeleteRequest.SetPathValue("record_id", "018f0000-0000-7000-8000-000000000030")
	deferDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteRecord(deferDeleteResponseRecorder, deferDeleteRequest)
	assert.Equal(t, http.StatusConflict, deferDeleteResponseRecorder.Code)
}

func TestDataBaseHandlerRESTRPCIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(db)
	eventBus := core.NewEventBus(db, nil)
	defer eventBus.Close()

	service := NewService(db)
	inMemoryKVStore := newInMemoryKVStore()
	service.SetKVStore(inMemoryKVStore)
	service.SetServiceAccountManager(serviceAccountManager)
	service.SetEventBus(eventBus)
	_ = service.Start(ctx)
	defer func() { _ = service.Stop() }()

	baseHandler := service.BaseHandler()

	// 1. Scalar echo function via POST (flat JSON payload)
	_, _ = db.Exec(ctx, `CREATE OR REPLACE FUNCTION public.echo_test(msg text) RETURNS text LANGUAGE sql AS $$ SELECT msg $$;`)
	rpcBodyReader := bytes.NewReader([]byte(`{"msg":"hello world"}`))
	rpcRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/rpc/echo_test", rpcBodyReader)
	rpcRequest.Header.Set("X-Layr-Invalidate-Tables", "users, public.products")
	rpcRequest.SetPathValue("schema_name", "public")
	rpcRequest.SetPathValue("function_name", "echo_test")
	rpcResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExecuteFunction(rpcResponseRecorder, rpcRequest)
	assert.Equal(t, http.StatusOK, rpcResponseRecorder.Code)
	assert.Equal(t, "\"hello world\"\n", rpcResponseRecorder.Body.String())

	// 2. Scalar echo function via GET (query parameter)
	rpcGetRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/rpc/echo_test?msg=hello+get", nil)
	rpcGetRequest.SetPathValue("schema_name", "public")
	rpcGetRequest.SetPathValue("function_name", "echo_test")
	rpcGetResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExecuteFunction(rpcGetResponseRecorder, rpcGetRequest)
	assert.Equal(t, http.StatusOK, rpcGetResponseRecorder.Code)
	assert.Equal(t, "\"hello get\"\n", rpcGetResponseRecorder.Body.String())

	// 3. Table-valued RPC returning array of objects
	_, _ = db.Exec(ctx, `CREATE OR REPLACE FUNCTION public.test_table_rpc() RETURNS TABLE(id int, name text) LANGUAGE sql AS $$ SELECT 1, 'first' UNION ALL SELECT 2, 'second' $$;`)
	rpcTableRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/rpc/test_table_rpc", nil)
	rpcTableRequest.SetPathValue("schema_name", "public")
	rpcTableRequest.SetPathValue("function_name", "test_table_rpc")
	rpcTableResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExecuteFunction(rpcTableResponseRecorder, rpcTableRequest)
	assert.Equal(t, http.StatusOK, rpcTableResponseRecorder.Code)
	assert.Contains(t, rpcTableResponseRecorder.Body.String(), `"id":1`)
	assert.Contains(t, rpcTableResponseRecorder.Body.String(), `"name":"first"`)

	// 4. Multi-row scalar function returning array of scalars
	_, _ = db.Exec(ctx, `CREATE OR REPLACE FUNCTION public.test_scalar_series() RETURNS SETOF int LANGUAGE sql AS $$ SELECT generate_series(1, 2) $$;`)
	rpcSeriesRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/rpc/test_scalar_series", nil)
	rpcSeriesRequest.SetPathValue("schema_name", "public")
	rpcSeriesRequest.SetPathValue("function_name", "test_scalar_series")
	rpcSeriesResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExecuteFunction(rpcSeriesResponseRecorder, rpcSeriesRequest)
	assert.Equal(t, http.StatusOK, rpcSeriesResponseRecorder.Code)
	assert.Equal(t, "[1,2]\n", rpcSeriesResponseRecorder.Body.String())

	// 5. Zero-arg RPC function with invalidate_tables query parameter
	_, _ = db.Exec(ctx, `CREATE OR REPLACE FUNCTION public.zero_args_test() RETURNS text LANGUAGE sql AS $$ SELECT 'zero_ok' $$;`)
	rpcZeroRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/public/rpc/zero_args_test?invalidate_tables=items", nil)
	rpcZeroRequest.SetPathValue("schema_name", "public")
	rpcZeroRequest.SetPathValue("function_name", "zero_args_test")
	rpcZeroResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExecuteFunction(rpcZeroResponseRecorder, rpcZeroRequest)
	assert.Equal(t, http.StatusOK, rpcZeroResponseRecorder.Code)
	assert.Equal(t, "\"zero_ok\"\n", rpcZeroResponseRecorder.Body.String())

	// 6. Void procedure returning 204 No Content
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.rpc_demo_items (id serial primary key, status text);
		CREATE OR REPLACE FUNCTION public.rpc_update_status(item_status text) RETURNS void LANGUAGE plpgsql AS $$
		BEGIN
			UPDATE public.rpc_demo_items SET status = item_status;
		END;
		$$;
	`)
	assert.NoError(t, err)
	rpcMutationRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/rpc/rpc_update_status", bytes.NewReader([]byte(`{"item_status":"updated"}`)))
	rpcMutationRequest.SetPathValue("schema_name", "public")
	rpcMutationRequest.SetPathValue("function_name", "rpc_update_status")
	rpcMutationRequest.Header.Set("X-Layr-Invalidate-Tables", "rpc_demo_items")
	rpcMutationResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExecuteFunction(rpcMutationResponseRecorder, rpcMutationRequest)
	assert.Equal(t, http.StatusNoContent, rpcMutationResponseRecorder.Code)

	// 7. Context canceled -> 500 error
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	canceledRPCRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPost, "/api/v1/data/public/rpc/zero_args_test", nil)
	canceledRPCRequest.SetPathValue("schema_name", "public")
	canceledRPCRequest.SetPathValue("function_name", "zero_args_test")
	canceledRPCResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExecuteFunction(canceledRPCResponseRecorder, canceledRPCRequest)
	assert.Equal(t, http.StatusInternalServerError, canceledRPCResponseRecorder.Code)

	// 8. Non-existent function -> 404 error
	badRPCFunctionRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/rpc/non_existent_func", nil)
	badRPCFunctionRequest.SetPathValue("schema_name", "public")
	badRPCFunctionRequest.SetPathValue("function_name", "non_existent_func")
	badRPCFunctionResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExecuteFunction(badRPCFunctionResponseRecorder, badRPCFunctionRequest)
	assert.Equal(t, http.StatusNotFound, badRPCFunctionResponseRecorder.Code)

	// 9. Deferred constraint commit failure -> 409 Conflict
	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.rpc_defer_items (
			id uuid primary key default uuidv7(),
			code text,
			constraint rpc_defer_code_uniq unique (code) deferrable initially deferred
		);
		CREATE OR REPLACE FUNCTION public.rpc_fail_defer() RETURNS void LANGUAGE plpgsql AS $$
		BEGIN
			INSERT INTO public.rpc_defer_items (code) VALUES ('DEFER_COLLIDE');
			INSERT INTO public.rpc_defer_items (code) VALUES ('DEFER_COLLIDE');
		END;
		$$;
	`)
	assert.NoError(t, err)
	deferRPCRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/public/rpc/rpc_fail_defer", nil)
	deferRPCRequest.SetPathValue("schema_name", "public")
	deferRPCRequest.SetPathValue("function_name", "rpc_fail_defer")
	deferRPCResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExecuteFunction(deferRPCResponseRecorder, deferRPCRequest)
	assert.Equal(t, http.StatusConflict, deferRPCResponseRecorder.Code)
}
