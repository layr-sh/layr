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
	"strconv"
	"strings"
	"time"

	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/data/common"
	datakv "layr.sh/data/kv"
	"layr.sh/data/rest"
)

const (
	maxRequestBodyBytes int64  = 10 * 1024 * 1024
	pgTypeOIDJSON       uint32 = 114
	pgTypeOIDJSONB      uint32 = 3802
)

// HandleListRecords handles GET /api/v1/data/{schema_name}/{table_name}.
func (handler *BaseHandler) HandleListRecords(responseWriter http.ResponseWriter, request *http.Request) {
	schema, table, _, tableMetadata, authClaims, ok := handler.prepareTableContext(responseWriter, request)
	if !ok {
		return
	}

	config := handler.configManager.Get()
	queryParams, err := rest.ParseQueryParams(request.URL.Query(), config.REST.DefaultLimit, config.REST.MaxLimit)
	if err != nil {
		handler.writeError(responseWriter, request, http.StatusBadRequest, err.Error(), "LAYR_DATA_001")
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
		common.ApplyRLS(ctx, tx, authClaims)
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

// HandleGetRecord handles GET /api/v1/data/{schema_name}/{table_name}/{record_id}.
func (handler *BaseHandler) HandleGetRecord(responseWriter http.ResponseWriter, request *http.Request) {
	schema, table, recordID, tableMetadata, authClaims, ok := handler.prepareTableContext(responseWriter, request)
	if !ok {
		return
	}
	if recordID == "" {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Missing record ID in path", "LAYR_DATA_001")
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
		common.ApplyRLS(ctx, tx, authClaims)
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
		handler.writeError(responseWriter, request, http.StatusNotFound, "Record not found", "LAYR_DATA_002")
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

// HandleCreateRecords handles POST /api/v1/data/{schema_name}/{table_name}.
func (handler *BaseHandler) HandleCreateRecords(responseWriter http.ResponseWriter, request *http.Request) {
	schema, table, _, tableMetadata, authClaims, ok := handler.prepareTableContext(responseWriter, request)
	if !ok {
		return
	}

	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodyBytes)
	bodyBytes, err := io.ReadAll(request.Body)
	if err != nil || len(bodyBytes) == 0 {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body", "LAYR_DATA_001")
		return
	}

	var rawRows []map[string]any
	if bodyBytes[0] == '[' {
		if decodeErr := json.Unmarshal(bodyBytes, &rawRows); decodeErr != nil {
			handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON array", "LAYR_DATA_001")
			return
		}
	} else {
		var insertRowPayload InsertRowPayload
		if payloadErr := json.Unmarshal(bodyBytes, &insertRowPayload); payloadErr == nil && len(insertRowPayload.Data) > 0 {
			for _, rec := range insertRowPayload.Data {
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
				handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON object", "LAYR_DATA_001")
				return
			}
			rawRows = append(rawRows, singleRow)
		}
	}

	if len(rawRows) == 0 {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "No rows to insert", "LAYR_DATA_001")
		return
	}

	onConflict := request.URL.Query().Get("on_conflict")
	if onConflict != "" && !common.IsValidIdentifier(onConflict) {
		handler.writeError(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("invalid on_conflict identifier: %s", onConflict), "LAYR_DATA_001")
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
		common.ApplyRLS(ctx, tx, authClaims)
	}

	queryBuilder := rest.NewQueryBuilder(schema, table)
	primaryKey := tableMetadata.PrimaryKey
	if primaryKey == "" {
		primaryKey = "id"
	}

	sqlStatement, buildErr := queryBuilder.BuildInsert(rawRows, onConflict)
	if buildErr != nil {
		handler.writeError(responseWriter, request, http.StatusBadRequest, buildErr.Error(), "LAYR_DATA_001")
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

// HandleUpdateRecord handles PATCH/PUT /api/v1/data/{schema_name}/{table_name}/{record_id}.
func (handler *BaseHandler) HandleUpdateRecord(responseWriter http.ResponseWriter, request *http.Request) {
	schema, table, recordID, tableMetadata, authClaims, ok := handler.prepareTableContext(responseWriter, request)
	if !ok {
		return
	}
	if recordID == "" {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Missing record ID in path", "LAYR_DATA_001")
		return
	}

	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodyBytes)
	bodyBytes, err := io.ReadAll(request.Body)
	if err != nil || len(bodyBytes) == 0 {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body", "LAYR_DATA_001")
		return
	}

	var updateRowPayload UpdateRowPayload
	rowMap := make(map[string]any)
	if payloadErr := json.Unmarshal(bodyBytes, &updateRowPayload); payloadErr == nil && len(updateRowPayload.Data.Properties) > 0 {
		for k, v := range updateRowPayload.Data.Properties {
			rowMap[k] = v
		}
	} else {
		if decodeErr := json.Unmarshal(bodyBytes, &rowMap); decodeErr != nil {
			handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON object", "LAYR_DATA_001")
			return
		}
	}

	ctx := request.Context()
	tx, err := handler.db.Begin(ctx)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer func() { _ = handler.resolveTransaction(ctx, tx, err) }()

	if !handler.isRLSBypassed(request, "data:query.write") {
		common.ApplyRLS(ctx, tx, authClaims)
	}

	primaryKey := tableMetadata.PrimaryKey
	if primaryKey == "" {
		primaryKey = "id"
	}

	queryBuilder := rest.NewQueryBuilder(schema, table)
	filters := []rest.FilterOp{{Column: primaryKey, Op: "eq", Value: recordID}}
	sqlStatement, err := queryBuilder.BuildUpdate(rowMap, filters)
	if err != nil {
		handler.writeError(responseWriter, request, http.StatusBadRequest, err.Error(), "LAYR_DATA_001")
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
		handler.writeError(responseWriter, request, http.StatusNotFound, "Record not found", "LAYR_DATA_002")
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

// HandleDeleteRecord handles DELETE /api/v1/data/{schema_name}/{table_name}/{record_id}.
func (handler *BaseHandler) HandleDeleteRecord(responseWriter http.ResponseWriter, request *http.Request) {
	schema, table, recordID, tableMetadata, authClaims, ok := handler.prepareTableContext(responseWriter, request)
	if !ok {
		return
	}
	if recordID == "" {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Missing record ID in path", "LAYR_DATA_001")
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
		common.ApplyRLS(ctx, tx, authClaims)
	}

	primaryKey := tableMetadata.PrimaryKey
	if primaryKey == "" {
		primaryKey = "id"
	}

	queryBuilder := rest.NewQueryBuilder(schema, table)
	filters := []rest.FilterOp{{Column: primaryKey, Op: "eq", Value: recordID}}
	sqlStatement, err := queryBuilder.BuildDelete(filters)
	if err != nil {
		handler.writeError(responseWriter, request, http.StatusBadRequest, err.Error(), "LAYR_DATA_001")
		return
	}

	result, err := tx.Exec(ctx, sqlStatement.SQL, sqlStatement.Args...)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	if result.RowsAffected() == 0 {
		handler.writeError(responseWriter, request, http.StatusNotFound, "Record not found", "LAYR_DATA_002")
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

// HandleExecuteFunction handles POST /api/v1/data/{schema_name}/rpc/{function_name}.
func (handler *BaseHandler) HandleExecuteFunction(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.REST.Enabled {
		handler.writeError(responseWriter, request, http.StatusForbidden, "REST API is disabled", "LAYR_DATA_003")
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
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Schema and function name are required", "LAYR_DATA_001")
		return
	}

	if !common.IsValidIdentifier(schema) || !common.IsValidIdentifier(functionName) {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid schema or function name identifier", "LAYR_DATA_001")
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
		handler.writeError(responseWriter, request, http.StatusForbidden, fmt.Sprintf("Schema %q is not exposed for REST operations", schema), "LAYR_DATA_003")
		return
	}

	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodyBytes)
	var executeFunctionRequest ExecuteFunctionRequest
	if request.Body != nil {
		if decodeErr := json.NewDecoder(request.Body).Decode(&executeFunctionRequest); decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
			handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload in request body", "LAYR_DATA_001")
			return
		}
	}

	for k := range executeFunctionRequest.Args {
		if !common.IsValidIdentifier(k) {
			handler.writeError(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("invalid argument name: %s", k), "LAYR_DATA_001")
			return
		}
	}

	ctx := request.Context()
	tx, err := handler.db.Begin(ctx)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer func() { _ = handler.resolveTransaction(ctx, tx, err) }()

	isMutation := request.Header.Get("X-Layr-Mutation") == "true" || request.URL.Query().Get("mutation") == "true"
	requiredScope := "data:query.read"
	if isMutation {
		requiredScope = "data:query.write"
	}

	authClaims := common.ExtractClaims(request)
	if !handler.isRLSBypassed(request, requiredScope) {
		common.ApplyRLS(ctx, tx, authClaims)
	}

	var query string
	var args []any
	if len(executeFunctionRequest.Args) > 0 {
		argPlaceholders := make([]string, 0, len(executeFunctionRequest.Args))
		paramIndex := 1
		for k, v := range executeFunctionRequest.Args {
			argPlaceholders = append(argPlaceholders, fmt.Sprintf(`"%s" := $%d`, k, paramIndex))
			args = append(args, v)
			paramIndex++
		}
		query = fmt.Sprintf(`SELECT * FROM "%s"."%s"(%s)`, schema, functionName, strings.Join(argPlaceholders, ", "))
	} else {
		query = fmt.Sprintf(`SELECT * FROM "%s"."%s"()`, schema, functionName)
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		handler.writeDBError(responseWriter, request, err)
		return
	}
	defer rows.Close()

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

	handler.writeJSON(responseWriter, http.StatusOK, ExecuteFunctionResponse{
		Result: results,
	})
}

func (handler *BaseHandler) prepareTableContext(responseWriter http.ResponseWriter, request *http.Request) (string, string, string, rest.TableMetadata, common.AuthClaims, bool) {
	config := handler.configManager.Get()
	if !config.REST.Enabled {
		handler.writeError(responseWriter, request, http.StatusForbidden, "REST API is disabled", "LAYR_DATA_003")
		return "", "", "", rest.TableMetadata{}, common.AuthClaims{}, false
	}

	schema, table, recordID, err := handler.extractSchemaTableAndRecordID(request)
	if err != nil {
		handler.writeError(responseWriter, request, http.StatusBadRequest, err.Error(), "LAYR_DATA_001")
		return "", "", "", rest.TableMetadata{}, common.AuthClaims{}, false
	}

	if !common.IsValidIdentifier(schema) || !common.IsValidIdentifier(table) {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid schema or table name identifier", "LAYR_DATA_001")
		return "", "", "", rest.TableMetadata{}, common.AuthClaims{}, false
	}

	isSchemaAllowed := false
	for _, allowedSchema := range config.Schemas {
		if allowedSchema == schema {
			isSchemaAllowed = true
			break
		}
	}
	if !isSchemaAllowed {
		handler.writeError(responseWriter, request, http.StatusForbidden, fmt.Sprintf("Schema %q is not exposed for REST operations", schema), "LAYR_DATA_003")
		return "", "", "", rest.TableMetadata{}, common.AuthClaims{}, false
	}

	for _, excludedTable := range config.REST.ExcludedTables {
		if excludedTable == table || excludedTable == fmt.Sprintf("%s.%s", schema, table) {
			handler.writeError(responseWriter, request, http.StatusForbidden, fmt.Sprintf("Table %q is excluded from REST operations", table), "LAYR_DATA_003")
			return "", "", "", rest.TableMetadata{}, common.AuthClaims{}, false
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

	authClaims := common.ExtractClaims(request)
	return schema, table, recordID, tableMetadata, authClaims, true
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
