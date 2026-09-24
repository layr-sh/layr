package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"layr.sh/core"
	"layr.sh/data/common"
	"layr.sh/data/graphql"
	datakv "layr.sh/data/kv"
)

const (
	defaultGraphQLMaxDepth = 8
	maxGraphQLComplexity   = 500
)

// handleExecuteGraphQL processes POST /v1/graphql requests.
func (handler *BaseHandler) handleExecuteGraphQL(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling execute GraphQL query request")

	if request.Method != http.MethodPost {
		handler.writeGraphQLError(responseWriter, http.StatusMethodNotAllowed, "GraphQL endpoint only supports POST requests")
		return
	}

	config := handler.configManager.Get()
	if !config.GraphQL.Enabled {
		handler.writeGraphQLError(responseWriter, http.StatusForbidden, "GraphQL API is disabled")
		return
	}

	var executeGraphQLInput ExecuteGraphQLInput
	if err := json.NewDecoder(request.Body).Decode(&executeGraphQLInput); err != nil {
		handler.writeGraphQLError(responseWriter, http.StatusBadRequest, "Invalid JSON request payload")
		return
	}

	if strings.TrimSpace(executeGraphQLInput.Query) == "" {
		handler.writeGraphQLError(responseWriter, http.StatusBadRequest, "GraphQL query string cannot be empty")
		return
	}

	operationNode, err := graphql.ParseGraphQLWithOperation(executeGraphQLInput.Query, executeGraphQLInput.OperationName, executeGraphQLInput.Variables)
	if err != nil {
		handler.writeGraphQLError(responseWriter, http.StatusBadRequest, fmt.Sprintf("GraphQL syntax error: %v", err))
		return
	}

	maxDepth := config.GraphQL.MaxDepth
	if maxDepth <= 0 {
		maxDepth = defaultGraphQLMaxDepth
	}
	actualDepth := operationNode.CalculateDepth()
	if actualDepth > maxDepth {
		handler.writeGraphQLError(responseWriter, http.StatusUnprocessableEntity, fmt.Sprintf("Query depth %d exceeds maximum allowed depth %d", actualDepth, maxDepth))
		return
	}

	maxComplexity := config.GraphQL.MaxComplexity
	if maxComplexity <= 0 {
		maxComplexity = maxGraphQLComplexity
	}
	complexity := operationNode.CalculateComplexity()
	if complexity > maxComplexity {
		handler.writeGraphQLError(responseWriter, http.StatusUnprocessableEntity, fmt.Sprintf("Query complexity %d exceeds limit of %d", complexity, maxComplexity))
		return
	}

	// Handle standard GraphQL introspection query (__schema or __type)
	if operationNode.Type == graphql.QueryOperationType && handler.isIntrospectionQuery(operationNode) {
		if !config.GraphQL.IntrospectionEnabled {
			handler.writeGraphQLError(responseWriter, http.StatusForbidden, "GraphQL introspection is disabled")
			return
		}
		introspectionData := handler.handleIntrospection(operationNode)
		log.Debug("executed GraphQL introspection query")
		handler.writeGraphQLSuccess(responseWriter, introspectionData)
		return
	}

	// Enforce default & max limits on root queries
	defaultLimit := config.GraphQL.DefaultLimit
	if defaultLimit <= 0 {
		defaultLimit = config.REST.DefaultLimit
	}
	maxLimit := config.REST.MaxLimit
	if maxLimit <= 0 {
		maxLimit = 1000
	}
	for fieldIndex := range operationNode.SelectionSet {
		if _, hasLimit := operationNode.SelectionSet[fieldIndex].Arguments["limit"]; !hasLimit && defaultLimit > 0 {
			operationNode.SelectionSet[fieldIndex].Arguments["limit"] = defaultLimit
		} else if limitArgument, ok := operationNode.SelectionSet[fieldIndex].Arguments["limit"]; ok {
			var numericLimit int
			switch v := limitArgument.(type) {
			case int64:
				numericLimit = int(v)
			case float64:
				numericLimit = int(v)
			}
			if numericLimit > maxLimit && maxLimit > 0 {
				operationNode.SelectionSet[fieldIndex].Arguments["limit"] = maxLimit
			}
		}
	}

	var cacheTTL int
	if operationNode.CacheTTL > 0 {
		cacheTTL = operationNode.CacheTTL
	} else if ttlHeader := request.Header.Get("X-Layr-Cache-TTL"); ttlHeader != "" {
		cacheTTL, _ = strconv.Atoi(ttlHeader)
	} else if config.Cache.Enabled && config.Cache.QueryTTLSeconds > 0 {
		cacheTTL = config.Cache.QueryTTLSeconds
	}

	keySuffix := operationNode.CacheKeySuffix
	if keySuffix == "" {
		keySuffix = request.Header.Get("X-Layr-Cache-Key-Suffix")
	}
	if keySuffix == "" {
		keySuffix = request.Header.Get("X-Layr-Cache-Key")
	}

	ctx := request.Context()
	jwtClaims := core.GetAuthContext(request.Context()).JWT

	var userVisibleKey string
	var internalCacheKey string

	isCacheActive := config.Cache.Enabled

	if isCacheActive && cacheTTL > 0 && operationNode.Type == graphql.QueryOperationType {
		referencedTables := handler.resolveGraphQLTables(operationNode)
		tableVersion := handler.getGraphQLTableCacheVersion(ctx, referencedTables)
		cacheAuthContext := datakv.ExtractAuthContext(request, handler.saltSecret)
		userVisibleKey = datakv.BuildGraphQLQueryKeyWithVersion(executeGraphQLInput.Query, executeGraphQLInput.Variables, keySuffix, tableVersion)
		internalCacheKey = datakv.BuildInternalKey(cacheAuthContext, userVisibleKey)
		responseWriter.Header().Set("X-Layr-Cache-Key", userVisibleKey)

		if cached, cacheGetErr := handler.kernel.KVStore().Get(ctx, internalCacheKey); cacheGetErr == nil && cached != "" {
			log.Debug("cache hit for GraphQL query")
			responseWriter.Header().Set("Content-Type", "application/json")
			responseWriter.Header().Set("X-Layr-Cache", "HIT")
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte(cached))
			return
		}
	}

	// Update compiler configuration for schema exposures and exclusions
	handler.graphqlCompiler.SetAllowedSchemas(config.Schemas)
	handler.graphqlCompiler.SetExcludedTables(config.REST.ExcludedTables)

	sqlQuery, err := handler.graphqlCompiler.Compile(operationNode)
	if err != nil {
		handler.writeGraphQLError(responseWriter, http.StatusBadRequest, fmt.Sprintf("GraphQL compilation error: %v", err))
		return
	}

	tx, err := handler.kernel.DB().Begin(ctx)
	if err != nil {
		handler.writeGraphQLDBError(responseWriter, err)
		return
	}
	defer func() { _ = handler.resolveTransaction(ctx, tx, err) }()

	requiredScope := core.ScopeDataQueryRead
	if operationNode.Type == graphql.MutationOperationType {
		requiredScope = core.ScopeDataQueryWrite
	}
	if !handler.isRLSBypassed(request, requiredScope) {
		common.ApplyRLS(ctx, tx, jwtClaims)
	}

	var rawJSON []byte
	err = tx.QueryRow(ctx, sqlQuery.SQL, sqlQuery.Args...).Scan(&rawJSON)
	if err != nil {
		handler.writeGraphQLDBError(responseWriter, err)
		return
	}

	// Invalidate cache and publish events on successful mutations
	if operationNode.Type == graphql.MutationOperationType {
		handler.handleMutationSideEffects(ctx, operationNode)
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	if isCacheActive && cacheTTL > 0 && operationNode.Type == graphql.QueryOperationType {
		responseWriter.Header().Set("X-Layr-Cache", "MISS")
		if userVisibleKey != "" {
			responseWriter.Header().Set("X-Layr-Cache-Key", userVisibleKey)
		}
	}

	var dataValue any
	if len(rawJSON) > 0 {
		dataValue = json.RawMessage(rawJSON)
	}

	executeGraphQLResponse := ExecuteGraphQLResponse{
		Data: dataValue,
	}
	responseBytes, _ := json.Marshal(executeGraphQLResponse)

	if isCacheActive && cacheTTL > 0 && operationNode.Type == graphql.QueryOperationType && internalCacheKey != "" {
		maxQueries := config.Cache.MaxCachedQueries
		shouldCache := true
		if maxQueries > 0 {
			if currentCountString, countErr := handler.kernel.KVStore().Get(ctx, "cache:query_count"); countErr == nil && currentCountString != "" {
				if currentCount, parseCountErr := strconv.ParseInt(currentCountString, 10, 64); parseCountErr == nil && currentCount >= int64(maxQueries) {
					shouldCache = false
				}
			}
		}
		if shouldCache {
			_ = handler.kernel.KVStore().Set(ctx, internalCacheKey, string(responseBytes), time.Duration(cacheTTL)*time.Second)
			counterExpiry := time.Duration(config.Cache.QueryTTLSeconds*2) * time.Second
			if counterExpiry < 5*time.Minute {
				counterExpiry = 5 * time.Minute
			}
			_, _ = handler.kernel.KVStore().Increment(ctx, "cache:query_count", counterExpiry)
		}
	}

	log.Debug("executed GraphQL operation successfully")
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = responseWriter.Write(responseBytes)
}

func (handler *BaseHandler) isIntrospectionQuery(operationNode *graphql.OperationNode) bool {
	for _, field := range operationNode.SelectionSet {
		if field.Name == "__schema" || field.Name == "__type" {
			return true
		}
	}
	return false
}

func (handler *BaseHandler) handleIntrospection(operationNode *graphql.OperationNode) map[string]any {
	result := make(map[string]any)
	tables := handler.schemaIntrospector.Tables()

	var types []map[string]any
	for _, table := range tables {
		var fields []map[string]any
		for columnName, columnInfo := range table.Columns {
			fields = append(fields, map[string]any{
				"name": columnName,
				"type": map[string]any{
					"name": columnInfo.DataType,
					"kind": "SCALAR",
				},
			})
		}
		for relationName, relationInfo := range table.ForeignKeys {
			fields = append(fields, map[string]any{
				"name": relationName,
				"type": map[string]any{
					"name": relationInfo.ForeignTable,
					"kind": "OBJECT",
				},
			})
		}
		types = append(types, map[string]any{
			"kind":   "OBJECT",
			"name":   table.Name,
			"fields": fields,
		})
	}

	schemaMap := map[string]any{
		"queryType": map[string]any{
			"name": "Query",
		},
		"mutationType": map[string]any{
			"name": "Mutation",
		},
		"types": types,
	}

	for _, field := range operationNode.SelectionSet {
		if field.Name == "__schema" {
			result["__schema"] = schemaMap
		} else if field.Name == "__type" {
			typeName, _ := field.Arguments["name"].(string)
			for _, t := range types {
				if t["name"] == typeName {
					result["__type"] = t
					break
				}
			}
			if _, found := result["__type"]; !found {
				result["__type"] = nil
			}
		}
	}

	return result
}

func (handler *BaseHandler) handleMutationSideEffects(ctx context.Context, operationNode *graphql.OperationNode) {
	for _, field := range operationNode.SelectionSet {
		name := field.Name
		tableName := ""
		action := ""
		if strings.HasPrefix(name, "insert_") {
			tableName = strings.TrimPrefix(name, "insert_")
			action = "insert"
		} else if strings.HasPrefix(name, "update_") {
			tableName = strings.TrimPrefix(name, "update_")
			action = "update"
		} else if strings.HasPrefix(name, "delete_") {
			tableName = strings.TrimPrefix(name, "delete_")
			action = "delete"
		}
		if tableName == "" {
			continue
		}

		schema := "public"
		if strings.Contains(tableName, "_") {
			for i := 1; i < len(tableName); i++ {
				if tableName[i] == '_' {
					s := tableName[:i]
					t := tableName[i+1:]
					if _, ok := handler.schemaIntrospector.GetTable(s, t); ok {
						schema = s
						tableName = t
						break
					}
				}
			}
		}

		// Invalidate cache
		handler.InvalidateTableCache(ctx, schema, tableName)

		// Publish domain event
		eventResourceID := fmt.Sprintf("%s.%s", schema, tableName)
		switch action {
		case "insert":
			handler.kernel.EventBus().Publish(ctx, NewRowCreatedEvent(eventResourceID, RowCreatedEventData{
				Schema:     schema,
				Table:      tableName,
				Properties: map[string]string{"mutation": name},
			}))
		case "update":
			handler.kernel.EventBus().Publish(ctx, NewRowUpdatedEvent(eventResourceID, RowUpdatedEventData{
				Schema:     schema,
				Table:      tableName,
				Properties: map[string]string{"mutation": name},
			}))
		case "delete":
			handler.kernel.EventBus().Publish(ctx, NewRowDeletedEvent(eventResourceID, RowDeletedEventData{
				Schema: schema,
				Table:  tableName,
			}))
		}
	}
}

func (handler *BaseHandler) writeGraphQLSuccess(responseWriter http.ResponseWriter, data any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(ExecuteGraphQLResponse{
		Data: data,
	})
}

func (handler *BaseHandler) writeGraphQLDBError(responseWriter http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := err.Error()

	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "42501": // insufficient_privilege
			status = http.StatusForbidden
			message = "Permission denied by database security policy"
		case "23505": // unique_violation
			status = http.StatusConflict
			message = fmt.Sprintf("Unique constraint violation: %s", pgError.Detail)
		case "23503", "23001": // foreign_key_violation or restrict_violation
			status = http.StatusConflict
			message = fmt.Sprintf("Foreign key violation: %s", pgError.Detail)
		case "42P01", "42883": // undefined_table or undefined_function
			status = http.StatusNotFound
			message = "Relation does not exist"
		case "42703": // undefined_column
			status = http.StatusBadRequest
			message = fmt.Sprintf("Column does not exist: %s", pgError.Message)
		}
	}

	handler.writeGraphQLError(responseWriter, status, message)
}

func (handler *BaseHandler) writeGraphQLError(responseWriter http.ResponseWriter, status int, detail string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(ExecuteGraphQLResponse{
		Errors: []GraphQLError{
			{
				Message: detail,
				Extensions: map[string]any{
					"status":    status,
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				},
			},
		},
	})
}

type graphQLTableRef struct {
	schema string
	table  string
}

func (handler *BaseHandler) resolveGraphQLTables(operationNode *graphql.OperationNode) []graphQLTableRef {
	var tableReferences []graphQLTableRef
	if operationNode == nil {
		return tableReferences
	}
	for _, fieldNode := range operationNode.SelectionSet {
		targetSchema := "public"
		targetTable := fieldNode.Name
		if strings.Contains(targetTable, "_") {
			for underscoreIndex := 1; underscoreIndex < len(targetTable); underscoreIndex++ {
				if targetTable[underscoreIndex] == '_' {
					potentialSchema := targetTable[:underscoreIndex]
					potentialTable := targetTable[underscoreIndex+1:]
					if _, exists := handler.schemaIntrospector.GetTable(potentialSchema, potentialTable); exists {
						targetSchema = potentialSchema
						targetTable = potentialTable
						break
					}
				}
			}
		}
		tableReferences = append(tableReferences, graphQLTableRef{schema: targetSchema, table: targetTable})
		if len(fieldNode.SelectionSet) > 0 {
			tableReferences = append(tableReferences, handler.resolveNestedGraphQLTables(targetSchema, fieldNode.SelectionSet)...)
		}
	}
	return tableReferences
}

func (handler *BaseHandler) resolveNestedGraphQLTables(schema string, subFields []graphql.FieldNode) []graphQLTableRef {
	var tableReferences []graphQLTableRef
	for _, subField := range subFields {
		if len(subField.SelectionSet) == 0 {
			continue
		}
		relationTable := subField.Name
		if _, exists := handler.schemaIntrospector.GetTable(schema, relationTable); exists {
			tableReferences = append(tableReferences, graphQLTableRef{schema: schema, table: relationTable})
		} else if strings.HasSuffix(relationTable, "s") {
			singular := strings.TrimSuffix(relationTable, "s")
			if _, exists := handler.schemaIntrospector.GetTable(schema, singular); exists {
				tableReferences = append(tableReferences, graphQLTableRef{schema: schema, table: singular})
			}
		} else {
			plural := relationTable + "s"
			if _, exists := handler.schemaIntrospector.GetTable(schema, plural); exists {
				tableReferences = append(tableReferences, graphQLTableRef{schema: schema, table: plural})
			}
		}
		tableReferences = append(tableReferences, handler.resolveNestedGraphQLTables(schema, subField.SelectionSet)...)
	}
	return tableReferences
}

func (handler *BaseHandler) getGraphQLTableCacheVersion(ctx context.Context, tableReferences []graphQLTableRef) int64 {
	if len(tableReferences) == 0 {
		return 0
	}
	if len(tableReferences) == 1 {
		return handler.getTableCacheVersion(ctx, tableReferences[0].schema, tableReferences[0].table)
	}
	fnvHash64 := fnv.New64a()
	for _, tableRef := range tableReferences {
		version := handler.getTableCacheVersion(ctx, tableRef.schema, tableRef.table)
		_, _ = fmt.Fprintf(fnvHash64, "%s.%s:%d;", tableRef.schema, tableRef.table, version)
	}
	return int64(fnvHash64.Sum64() & uint64(math.MaxInt64))
}
