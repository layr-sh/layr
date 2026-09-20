package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
	"layr.sh/data/common"
	datakv "layr.sh/data/kv"
	"layr.sh/data/rest"
)

const (
	maxRequestBodyBytes int64  = 10 * 1024 * 1024
	pgTypeOIDJSON       uint32 = 114
	pgTypeOIDJSONB      uint32 = 3802
	pgTypeOIDVoid       uint32 = 2278
)

// handleListRecords handles GET /api/v1/data/{schema_name}/{table_name}.
func (handler *BaseHandler) handleListRecords(responseWriter http.ResponseWriter, request *http.Request) {
	schema, table, _, tableMetadata, jwtClaims, ok := handler.prepareTableContext(responseWriter, request)
	if !ok {
		return
	}

	config := handler.configManager.Get()
	queryParams, err := rest.ParseQueryParams(request.URL.Query(), config.REST.DefaultLimit, config.REST.MaxLimit)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid query parameters", fmt.Sprintf("query records rejected for table %s.%s: %v", schema, table, err))
		return
	}

	cacheTTL := 0
	if ttlRaw := request.URL.Query().Get("cache_ttl"); ttlRaw != "" {
		if parsedTTL, parseErr := strconv.Atoi(ttlRaw); parseErr == nil && parsedTTL > 0 {
			cacheTTL = parsedTTL
		}
	} else if ttlHeader := request.Header.Get("X-Layr-Cache-TTL"); ttlHeader != "" {
		if parsedTTL, parseErr := strconv.Atoi(ttlHeader); parseErr == nil && parsedTTL > 0 {
			cacheTTL = parsedTTL
		}
	} else if handler.configManager.Get().Cache.Enabled {
		if defaultQueryTTL := handler.configManager.Get().Cache.QueryTTLSeconds; defaultQueryTTL > 0 {
			cacheTTL = defaultQueryTTL
		}
	}

	keySuffix := request.URL.Query().Get("cache_key_suffix")
	if keySuffix == "" {
		keySuffix = request.URL.Query().Get("cache_key")
	}
	if keySuffix == "" {
		keySuffix = request.Header.Get("X-Layr-Cache-Key-Suffix")
	}
	if keySuffix == "" {
		keySuffix = request.Header.Get("X-Layr-Cache-Key")
	}

	ctx := request.Context()
	var userVisibleCacheKey string
	var internalCacheKey string

	embeddedRelations := collectEmbeddedRelations(queryParams.Embedded)
	tableVersion := handler.getCompositeTableCacheVersion(ctx, schema, table, embeddedRelations)
	if cacheTTL > 0 && handler.kvStore != nil {
		cacheAuthContext := datakv.ExtractAuthContext(request, handler.saltSecret)
		userVisibleCacheKey = rest.GenerateUserVisibleRESTKeyWithVersion(schema, table, queryParams, keySuffix, tableVersion)
		internalCacheKey = datakv.BuildInternalKey(cacheAuthContext, userVisibleCacheKey)
		responseWriter.Header().Set("X-Layr-Cache-Key", userVisibleCacheKey)

		if cached, cacheGetErr := handler.kvStore.Get(ctx, internalCacheKey); cacheGetErr == nil && cached != "" {
			responseWriter.Header().Set("Content-Type", "application/json")
			responseWriter.Header().Set("X-Layr-Cache", "HIT")
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte(cached))
			return
		}
	}

	tx, err := handler.db.Begin(ctx)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer func() { _ = handler.resolveTransaction(ctx, tx, err) }()

	if !handler.isRLSBypassed(request, "data:query.read") {
		common.ApplyRLS(ctx, tx, jwtClaims)
	}

	queryBuilder := rest.NewQueryBuilder(schema, table)
	sqlStatement, _ := queryBuilder.BuildSelect(queryParams, tableMetadata)

	rows, err := tx.Query(ctx, sqlStatement.SQL, sqlStatement.Args...)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer rows.Close()

	results := handler.scanRowsToJSONMaps(rows)

	if queryParams.CountExact {
		countSQLStatement, countErr := queryBuilder.BuildCount(queryParams)
		if countErr == nil {
			var totalCount int64
			if scanErr := tx.QueryRow(ctx, countSQLStatement.SQL, countSQLStatement.Args...).Scan(&totalCount); scanErr == nil {
				responseWriter.Header().Set("X-Total-Count", fmt.Sprintf("%d", totalCount))
				end := queryParams.Offset + len(results) - 1
				if end < queryParams.Offset {
					end = queryParams.Offset
				}
				responseWriter.Header().Set("Content-Range", fmt.Sprintf("items %d-%d/%d", queryParams.Offset, end, totalCount))
				responseWriter.Header().Set("Range-Unit", "items")
			}
		}
	}

	responseJSON, _ := json.Marshal(results)

	if cacheTTL > 0 && handler.kvStore != nil && internalCacheKey != "" {
		maxQueries := handler.configManager.Get().Cache.MaxCachedQueries
		shouldCache := true
		if maxQueries > 0 {
			if currentCountString, countErr := handler.kvStore.Get(ctx, "cache:query_count"); countErr == nil && currentCountString != "" {
				if currentCount, parseCountErr := strconv.ParseInt(currentCountString, 10, 64); parseCountErr == nil && currentCount >= int64(maxQueries) {
					shouldCache = false
				}
			}
		}
		if shouldCache {
			_ = handler.kvStore.Set(ctx, internalCacheKey, string(responseJSON), time.Duration(cacheTTL)*time.Second)
			counterExpiry := time.Duration(handler.configManager.Get().Cache.QueryTTLSeconds*2) * time.Second
			if counterExpiry < 5*time.Minute {
				counterExpiry = 5 * time.Minute
			}
			_, _ = handler.kvStore.Increment(ctx, "cache:query_count", counterExpiry)
			responseWriter.Header().Set("X-Layr-Cache", "MISS")
		}
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = responseWriter.Write(responseJSON)
}

// handleGetRecord handles GET /api/v1/data/{schema_name}/{table_name}/{record_id}.
func (handler *BaseHandler) handleGetRecord(responseWriter http.ResponseWriter, request *http.Request) {
	schema, table, recordID, tableMetadata, jwtClaims, ok := handler.prepareTableContext(responseWriter, request)
	if !ok {
		return
	}
	if recordID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Missing record ID in path")
		return
	}

	ctx := request.Context()
	tx, err := handler.db.Begin(ctx)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer func() { _ = handler.resolveTransaction(ctx, tx, err) }()

	if !handler.isRLSBypassed(request, "data:query.read") {
		common.ApplyRLS(ctx, tx, jwtClaims)
	}

	primaryKey := tableMetadata.PrimaryKey
	if primaryKey == "" {
		primaryKey = "id"
	}

	selectColumns := handler.buildSelectColumns(request, tableMetadata)
	query := fmt.Sprintf(`SELECT %s FROM "%s"."%s" WHERE "%s" = $1 LIMIT 1`, selectColumns, schema, table, primaryKey)

	rows, err := tx.Query(ctx, query, recordID)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer rows.Close()

	results := handler.scanRowsToJSONMaps(rows)
	if len(results) == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Record not found")
		return
	}

	if isReturnMinimal(request) {
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(results[0])
}

// handleCreateRecord handles POST /api/v1/data/{schema_name}/{table_name}.
func (handler *BaseHandler) handleCreateRecord(responseWriter http.ResponseWriter, request *http.Request) {
	schema, table, _, tableMetadata, jwtClaims, ok := handler.prepareTableContext(responseWriter, request)
	if !ok {
		return
	}

	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodyBytes)
	bodyBytes, err := io.ReadAll(request.Body)
	if err != nil || len(bodyBytes) == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body")
		return
	}

	var rawRows []map[string]any
	if bodyBytes[0] == '[' {
		if decodeErr := json.Unmarshal(bodyBytes, &rawRows); decodeErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request payload")
			return
		}
	} else {
		var createRecordInput CreateRecordInput
		if payloadErr := json.Unmarshal(bodyBytes, &createRecordInput); payloadErr == nil && len(createRecordInput.Data) > 0 {
			for _, rec := range createRecordInput.Data {
				rowMap := make(map[string]any)
				if rec.ID != "" {
					rowMap["id"] = rec.ID
				}
				for k, v := range rec.Properties {
					rowMap[k] = v
				}
				rawRows = append(rawRows, rowMap)
			}
		} else {
			var singleRow map[string]any
			if decodeErr := json.Unmarshal(bodyBytes, &singleRow); decodeErr != nil {
				core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request payload")
				return
			}
			rawRows = append(rawRows, singleRow)
		}
	}

	if len(rawRows) == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "No records to insert")
		return
	}

	onConflict := request.URL.Query().Get("on_conflict")
	if onConflict != "" && !common.IsValidIdentifier(onConflict) {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid query parameter")
		return
	}

	ctx := request.Context()
	tx, err := handler.db.Begin(ctx)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer func() { _ = handler.resolveTransaction(ctx, tx, err) }()

	if !handler.isRLSBypassed(request, "data:query.write") {
		common.ApplyRLS(ctx, tx, jwtClaims)
	}

	queryBuilder := rest.NewQueryBuilder(schema, table)
	primaryKey := tableMetadata.PrimaryKey
	if primaryKey == "" {
		primaryKey = "id"
	}

	sqlStatement, buildErr := queryBuilder.BuildInsert(rawRows, onConflict)
	if buildErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request data", buildErr.Error())
		return
	}

	var rows pgx.Rows
	rows, err = tx.Query(ctx, sqlStatement.SQL, sqlStatement.Args...)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer rows.Close()

	insertedRows := handler.scanRowsToJSONMaps(rows)

	if commitErr := tx.Commit(ctx); commitErr != nil {
		handler.writeDBError(responseWriter, request, commitErr)
		return
	}
	tx = nil

	handler.invalidateTableCache(ctx, schema, table)

	for _, rawRow := range insertedRows {
		rowID := ""
		if rawID, exists := rawRow[primaryKey]; exists && rawID != nil {
			rowID = fmt.Sprintf("%v", rawID)
		}
		props := make(map[string]string)
		for k, v := range rawRow {
			if v != nil {
				props[k] = fmt.Sprintf("%v", v)
			}
		}

		if handler.eventBus != nil {
			eventResourceID := fmt.Sprintf("%s.%s:%s", schema, table, rowID)
			handler.eventBus.Publish(ctx, NewRowCreatedEvent(eventResourceID, RowCreatedEventData{
				Schema:     schema,
				Table:      table,
				ID:         rowID,
				Properties: props,
			}))
		}
	}

	if isReturnMinimal(request) {
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusCreated)

	if len(rawRows) == 1 && len(insertedRows) == 1 {
		rowID := ""
		if rawID, exists := insertedRows[0][primaryKey]; exists && rawID != nil {
			rowID = fmt.Sprintf("%v", rawID)
			responseWriter.Header().Set("Location", fmt.Sprintf("/api/v1/data/%s/%s/%s", schema, table, rowID))
		}
		_ = json.NewEncoder(responseWriter).Encode(insertedRows[0])
	} else {
		_ = json.NewEncoder(responseWriter).Encode(insertedRows)
	}
}

// handleUpdateRecord handles PATCH/PUT /api/v1/data/{schema_name}/{table_name}/{record_id}.
func (handler *BaseHandler) handleUpdateRecord(responseWriter http.ResponseWriter, request *http.Request) {
	schema, table, recordID, tableMetadata, jwtClaims, ok := handler.prepareTableContext(responseWriter, request)
	if !ok {
		return
	}
	if recordID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Missing record ID in path")
		return
	}

	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodyBytes)
	bodyBytes, err := io.ReadAll(request.Body)
	if err != nil || len(bodyBytes) == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body")
		return
	}

	var updateRecordInput UpdateRecordInput
	rowMap := make(map[string]any)
	if payloadErr := json.Unmarshal(bodyBytes, &updateRecordInput); payloadErr == nil && len(updateRecordInput.Data.Properties) > 0 {
		for k, v := range updateRecordInput.Data.Properties {
			rowMap[k] = v
		}
	} else {
		var rowData map[string]any
		if decodeErr := json.Unmarshal(bodyBytes, &rowData); decodeErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request payload")
			return
		}
		rowMap = rowData
	}

	ctx := request.Context()
	tx, err := handler.db.Begin(ctx)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer func() { _ = handler.resolveTransaction(ctx, tx, err) }()

	if !handler.isRLSBypassed(request, "data:query.write") {
		common.ApplyRLS(ctx, tx, jwtClaims)
	}

	primaryKey := tableMetadata.PrimaryKey
	if primaryKey == "" {
		primaryKey = "id"
	}

	queryBuilder := rest.NewQueryBuilder(schema, table)
	filters := []rest.FilterOp{{Column: primaryKey, Op: "eq", Value: recordID}}
	sqlStatement, err := queryBuilder.BuildUpdate(rowMap, filters)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request data", err.Error())
		return
	}

	rows, err := tx.Query(ctx, sqlStatement.SQL, sqlStatement.Args...)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer rows.Close()

	updatedRows := handler.scanRowsToJSONMaps(rows)
	if len(updatedRows) == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Record not found")
		return
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		handler.writeDBError(responseWriter, request, commitErr)
		return
	}
	tx = nil

	updatedRow := updatedRows[0]
	handler.invalidateTableCache(ctx, schema, table)

	props := make(map[string]string)
	for k, v := range updatedRow {
		if v != nil {
			props[k] = fmt.Sprintf("%v", v)
		}
	}

	if handler.eventBus != nil {
		eventResourceID := fmt.Sprintf("%s.%s:%s", schema, table, recordID)
		handler.eventBus.Publish(ctx, NewRowUpdatedEvent(eventResourceID, RowUpdatedEventData{
			Schema:     schema,
			Table:      table,
			ID:         recordID,
			Properties: props,
		}))
	}

	if isReturnMinimal(request) {
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(updatedRow)
}

// handleDeleteRecord handles DELETE /api/v1/data/{schema_name}/{table_name}/{record_id}.
func (handler *BaseHandler) handleDeleteRecord(responseWriter http.ResponseWriter, request *http.Request) {
	schema, table, recordID, tableMetadata, jwtClaims, ok := handler.prepareTableContext(responseWriter, request)
	if !ok {
		return
	}
	if recordID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Missing record ID in path")
		return
	}

	ctx := request.Context()
	tx, err := handler.db.Begin(ctx)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer func() { _ = handler.resolveTransaction(ctx, tx, err) }()

	if !handler.isRLSBypassed(request, "data:query.write") {
		common.ApplyRLS(ctx, tx, jwtClaims)
	}

	primaryKey := tableMetadata.PrimaryKey
	if primaryKey == "" {
		primaryKey = "id"
	}

	queryBuilder := rest.NewQueryBuilder(schema, table)
	filters := []rest.FilterOp{{Column: primaryKey, Op: "eq", Value: recordID}}
	sqlStatement, err := queryBuilder.BuildDelete(filters)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request data", err.Error())
		return
	}

	result, err := tx.Exec(ctx, sqlStatement.SQL, sqlStatement.Args...)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	if result.RowsAffected() == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Record not found")
		return
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		handler.writeDBError(responseWriter, request, commitErr)
		return
	}
	tx = nil

	handler.invalidateTableCache(ctx, schema, table)

	if handler.eventBus != nil {
		eventResourceID := fmt.Sprintf("%s.%s:%s", schema, table, recordID)
		handler.eventBus.Publish(ctx, NewRowDeletedEvent(eventResourceID, RowDeletedEventData{
			Schema: schema,
			Table:  table,
			ID:     recordID,
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

// handleExecuteFunction handles GET and POST /api/v1/data/{schema_name}/rpc/{function_name}.
// It invokes PostgreSQL stored functions/procedures with the caller's RLS session claims.
// - GET: Read operation with arguments provided via URL query params.
// - POST: Mutation operation with arguments provided via flat JSON body.
// Note: End users execute under Postgres RLS. Service accounts can bypass RLS if granted data:query.read (for GET) or data:query.write (for POST).
// Responses are unwrapped: raw scalar, array of objects, array of scalars, or 204 No Content for void/empty.
func (handler *BaseHandler) handleExecuteFunction(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		core.WriteErrorResponse(responseWriter, request, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	config := handler.configManager.Get()
	if !config.REST.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "execute function rejected: REST API is disabled in configuration")
		return
	}

	schema := request.PathValue("schema_name")
	functionName := request.PathValue("function_name")
	if schema == "" || functionName == "" {
		trimmed := strings.TrimPrefix(request.URL.Path, "/api/v1/data/")
		parts := strings.Split(trimmed, "/")
		if len(parts) >= 3 && parts[1] == "rpc" {
			schema = parts[0]
			functionName = parts[2]
		}
	}

	if schema == "" || functionName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Function name is required")
		return
	}

	if !common.IsValidIdentifier(schema) || !common.IsValidIdentifier(functionName) {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request")
		return
	}

	isSchemaAllowed := false
	for _, allowedSchema := range config.Schemas {
		if allowedSchema == schema {
			isSchemaAllowed = true
			break
		}
	}
	if !isSchemaAllowed {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", fmt.Sprintf("Schema %q is not exposed for REST operations", schema))
		return
	}

	// Service accounts can bypass RLS if granted data:query.read (on GET) or data:query.write (on POST).
	// Regular end-users do not have service account scopes and are always evaluated under PostgreSQL RLS.
	rlsBypassScope := "data:query.read"
	if request.Method == http.MethodPost {
		rlsBypassScope = "data:query.write"
	}

	args := make(map[string]any)
	if request.Method == http.MethodGet {
		for k, values := range request.URL.Query() {
			if k == "invalidate_tables" {
				continue
			}
			if len(values) > 0 {
				args[k] = parseQueryArgValue(values[0])
			}
		}
	} else if request.Method == http.MethodPost {
		if request.Body != nil {
			request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodyBytes)
			var rawPayload map[string]any
			decodeErr := json.NewDecoder(request.Body).Decode(&rawPayload)
			if decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
				core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request payload")
				return
			}
			if rawPayload != nil {
				args = rawPayload
			}
		}
	}

	for k := range args {
		if !common.IsValidIdentifier(k) {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid argument")
			return
		}
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "execute function rejected: database pool unavailable")
		return
	}

	ctx := request.Context()
	tx, err := handler.db.Begin(ctx)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer func() { _ = handler.resolveTransaction(ctx, tx, err) }()

	jwtClaims := core.GetAuthContext(request.Context()).JWT
	if !handler.isRLSBypassed(request, rlsBypassScope) {
		common.ApplyRLS(ctx, tx, jwtClaims)
	}

	var query string
	var queryArgs []any
	if len(args) > 0 {
		keys := make([]string, 0, len(args))
		for k := range args {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		argPlaceholders := make([]string, 0, len(keys))
		queryArgs = make([]any, 0, len(keys))
		for i, k := range keys {
			argPlaceholders = append(argPlaceholders, fmt.Sprintf(`"%s" := $%d`, k, i+1))
			queryArgs = append(queryArgs, args[k])
		}
		query = fmt.Sprintf(`SELECT * FROM "%s"."%s"(%s)`, schema, functionName, strings.Join(argPlaceholders, ", "))
	} else {
		query = fmt.Sprintf(`SELECT * FROM "%s"."%s"()`, schema, functionName)
	}

	rows, err := tx.Query(ctx, query, queryArgs...)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	isVoid := len(fieldDescriptions) == 1 && fieldDescriptions[0].DataTypeOID == pgTypeOIDVoid

	results := handler.scanRowsToJSONMaps(rows)

	if commitErr := tx.Commit(ctx); commitErr != nil {
		handler.writeDBError(responseWriter, request, commitErr)
		return
	}
	tx = nil

	invalidateHeader := request.Header.Get("X-Layr-Invalidate-Tables")
	if invalidateHeader == "" {
		invalidateHeader = request.URL.Query().Get("invalidate_tables")
	}
	if invalidateHeader != "" {
		for _, tableName := range strings.Split(invalidateHeader, ",") {
			tableName = strings.TrimSpace(tableName)
			if tableName != "" {
				targetSchema := schema
				targetTable := tableName
				if strings.Contains(tableName, ".") {
					parts := strings.SplitN(tableName, ".", 2)
					targetSchema = parts[0]
					targetTable = parts[1]
				}
				handler.invalidateTableCache(ctx, targetSchema, targetTable)
			}
		}
	}

	if isVoid || len(results) == 0 {
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")

	// Scalar function check: single column whose name matches functionName
	if len(fieldDescriptions) == 1 && fieldDescriptions[0].Name == functionName {
		if len(results) == 1 {
			responseWriter.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(responseWriter).Encode(results[0][functionName])
			return
		}
		scalars := make([]any, len(results))
		for i, r := range results {
			scalars[i] = r[functionName]
		}
		responseWriter.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(responseWriter).Encode(scalars)
		return
	}

	// Table-valued or multiple columns: return raw array of objects
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(results)
}

// parseQueryArgValue parses URL query parameter strings into typed primitives (int, float, bool, JSON) or fallback string.
func parseQueryArgValue(rawText string) any {
	if integerResult, err := strconv.ParseInt(rawText, 10, 64); err == nil {
		return integerResult
	}
	if floatResult, err := strconv.ParseFloat(rawText, 64); err == nil {
		return floatResult
	}
	if booleanResult, err := strconv.ParseBool(rawText); err == nil {
		return booleanResult
	}
	if strings.HasPrefix(rawText, "{") || strings.HasPrefix(rawText, "[") {
		var decodedJSON any
		if err := json.Unmarshal([]byte(rawText), &decodedJSON); err == nil {
			return decodedJSON
		}
	}
	return rawText
}

func (handler *BaseHandler) prepareTableContext(responseWriter http.ResponseWriter, request *http.Request) (string, string, string, rest.TableMetadata, core.JWTClaims, bool) {
	config := handler.configManager.Get()
	if !config.REST.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "table request rejected: REST API is disabled in configuration")
		return "", "", "", rest.TableMetadata{}, core.JWTClaims{}, false
	}

	schema, table, recordID, err := handler.extractSchemaTableAndRecordID(request)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request path")
		return "", "", "", rest.TableMetadata{}, core.JWTClaims{}, false
	}

	if !common.IsValidIdentifier(schema) || !common.IsValidIdentifier(table) {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request path")
		return "", "", "", rest.TableMetadata{}, core.JWTClaims{}, false
	}

	isSchemaAllowed := false
	for _, allowedSchema := range config.Schemas {
		if allowedSchema == schema {
			isSchemaAllowed = true
			break
		}
	}
	if !isSchemaAllowed {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", fmt.Sprintf("Schema %q is not exposed for REST operations", schema))
		return "", "", "", rest.TableMetadata{}, core.JWTClaims{}, false
	}

	for _, excludedTable := range config.REST.ExcludedTables {
		if excludedTable == table || excludedTable == fmt.Sprintf("%s.%s", schema, table) {
			core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", fmt.Sprintf("Table %q is excluded from REST operations", table))
			return "", "", "", rest.TableMetadata{}, core.JWTClaims{}, false
		}
	}

	tableKey := fmt.Sprintf("%s.%s", schema, table)
	handler.tablesRWMutex.RLock()
	tableMetadata, exists := handler.tables[tableKey]
	handler.tablesRWMutex.RUnlock()
	if !exists {
		tableMetadata = rest.TableMetadata{
			Schema:      schema,
			Table:       table,
			PrimaryKey:  "id",
			Columns:     nil,
			ForeignKeys: make(map[string]rest.RelationForeignKey),
		}
	}

	jwtClaims := core.GetAuthContext(request.Context()).JWT
	return schema, table, recordID, tableMetadata, jwtClaims, true
}

func (handler *BaseHandler) extractSchemaTableAndRecordID(request *http.Request) (string, string, string, error) {
	schema := request.PathValue("schema_name")
	table := request.PathValue("table_name")
	recordID := request.PathValue("record_id")
	if schema != "" && table != "" {
		return schema, table, recordID, nil
	}
	return handler.parsePath(request.URL.Path)
}

func (handler *BaseHandler) buildSelectColumns(request *http.Request, tableMetadata rest.TableMetadata) string {
	selectParam := request.URL.Query().Get("select")
	if selectParam != "" && selectParam != "*" {
		columns := strings.Split(selectParam, ",")
		sanitizedColumns := make([]string, 0, len(columns))
		for _, columnName := range columns {
			columnName = strings.TrimSpace(columnName)
			if common.IsValidIdentifier(columnName) {
				sanitizedColumns = append(sanitizedColumns, fmt.Sprintf(`"%s"`, columnName))
			}
		}
		if len(sanitizedColumns) > 0 {
			return strings.Join(sanitizedColumns, ", ")
		}
	}
	if len(tableMetadata.Columns) > 0 {
		formattedColumns := make([]string, len(tableMetadata.Columns))
		for index, columnName := range tableMetadata.Columns {
			formattedColumns[index] = fmt.Sprintf(`"%s"`, columnName)
		}
		return strings.Join(formattedColumns, ", ")
	}
	return "*"
}

func (handler *BaseHandler) scanRowsToJSONMaps(rows pgx.Rows) []map[string]any {
	fieldDescriptions := rows.FieldDescriptions()
	results := make([]map[string]any, 0)

	for rows.Next() {
		values, _ := rows.Values()
		rowMap := make(map[string]any, len(fieldDescriptions))
		for index, field := range fieldDescriptions {
			rawValue := values[index]
			switch typedValue := rawValue.(type) {
			case [16]byte:
				rowMap[field.Name] = uuid.UUID(typedValue).String()
			case []byte:
				switch field.DataTypeOID {
				case pgTypeOIDJSON, pgTypeOIDJSONB:
					rowMap[field.Name] = json.RawMessage(typedValue)
				default:
					rowMap[field.Name] = rawValue
				}
			default:
				rowMap[field.Name] = rawValue
			}
		}
		results = append(results, rowMap)
	}

	return results
}

func collectEmbeddedRelations(embedded []rest.EmbeddedField) []string {
	var relations []string
	for _, ef := range embedded {
		relations = append(relations, ef.Relation)
		if len(ef.Children) > 0 {
			relations = append(relations, collectEmbeddedRelations(ef.Children)...)
		}
	}
	return relations
}

func (handler *BaseHandler) getCompositeTableCacheVersion(ctx context.Context, schema string, rootTable string, embeddedRelations []string) int64 {
	rootVersion := handler.getTableCacheVersion(ctx, schema, rootTable)
	if len(embeddedRelations) == 0 {
		return rootVersion
	}
	fnvHash64 := fnv.New64a()
	_, _ = fmt.Fprintf(fnvHash64, "%s:%s:%d", schema, rootTable, rootVersion)
	for _, rel := range embeddedRelations {
		relVersion := handler.getTableCacheVersion(ctx, schema, rel)
		_, _ = fmt.Fprintf(fnvHash64, ";%s:%d", rel, relVersion)
	}
	return int64(fnvHash64.Sum64() & uint64(math.MaxInt64))
}

func (handler *BaseHandler) getTableCacheVersion(ctx context.Context, schema, table string) int64 {
	if handler.kvStore == nil {
		return 0
	}
	versionString, getErr := handler.kvStore.Get(ctx, fmt.Sprintf("cache:v:%s:%s", schema, table))
	if getErr != nil || versionString == "" {
		return 0
	}
	versionNumber, parseErr := strconv.ParseInt(versionString, 10, 64)
	if parseErr != nil {
		return 0
	}
	return versionNumber
}

func (handler *BaseHandler) invalidateTableCache(ctx context.Context, schema, table string) {
	if handler.kvStore == nil {
		return
	}
	cacheConfig := handler.configManager.Get().Cache
	if !cacheConfig.Enabled || !cacheConfig.InvalidateOnMutation {
		return
	}
	versionKey := fmt.Sprintf("cache:v:%s:%s", schema, table)
	newVersion, err := handler.kvStore.Increment(ctx, versionKey, 0)
	const maxSafeVersion int64 = 9_000_000_000_000_000
	if err != nil || newVersion >= maxSafeVersion {
		_ = handler.kvStore.Set(ctx, versionKey, "1", 0)
	}
}

// InvalidateTableCache invalidates cached query results for a specific table.
func (handler *BaseHandler) InvalidateTableCache(ctx context.Context, schema, table string) {
	handler.invalidateTableCache(ctx, schema, table)
}

// ResetTableCacheVersion explicitly resets the table cache version back to zero.
func (handler *BaseHandler) ResetTableCacheVersion(ctx context.Context, schema, table string) {
	if handler.kvStore == nil {
		return
	}
	versionKey := fmt.Sprintf("cache:v:%s:%s", schema, table)
	_ = handler.kvStore.Delete(ctx, versionKey)
}

func isReturnMinimal(request *http.Request) bool {
	prefer := request.Header.Get("Prefer")
	return strings.Contains(prefer, "return=minimal")
}
