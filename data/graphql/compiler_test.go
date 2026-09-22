package graphql

import (
	"strings"
	"testing"

	"layr.sh/core"
)

func TestGraphqlCompilerQueryAndMutationUnit(t *testing.T) {
	schemaIntrospector := NewSchemaIntrospector(core.NewTestKernel(nil))
	schemaIntrospector.tables["public.users"] = &TableInfo{
		Schema:     "public",
		Name:       "users",
		PrimaryKey: "id",
		Columns: map[string]ColumnInfo{
			"id":    {Name: "id", IsPrimaryKey: true},
			"name":  {Name: "name"},
			"email": {Name: "email"},
		},
	}
	schemaIntrospector.tables["tenant_a.orders"] = &TableInfo{
		Schema:     "tenant_a",
		Name:       "orders",
		PrimaryKey: "id",
		Columns: map[string]ColumnInfo{
			"id":    {Name: "id", IsPrimaryKey: true},
			"total": {Name: "total"},
		},
	}

	// 1. Test NewCompiler with empty defaultSchema
	defaultCompiler := NewCompiler(schemaIntrospector, "")
	if defaultCompiler.defaultSchema != "public" {
		t.Fatalf("expected public default schema, got %s", defaultCompiler.defaultSchema)
	}

	compiler := NewCompiler(schemaIntrospector, "public")

	// 2. Compile Query with aliases, nested relations (2 levels with aliases and empty selection sets), where clauses testing ALL operators and empty where map
	query := `
		query {
			my_users: users(where: {
				name: "Alice",
				email: {
					_eq: "test@layr.sh",
					_neq: "locked@layr.sh",
					_like: "%@layr.sh",
					_ilike: "%@LAYR.SH",
					_gt: "a",
					_gte: "a",
					_lt: "z",
					_lte: "z",
					_in: ["a@layr.sh", "b@layr.sh"],
					_is_null: false
				},
				status: { _is_null: true }
			}, limit: 10, offset: 5) {
				my_id: id
				name
				my_posts: posts(where: { title: { _neq: "Draft" } }) {
					id
					title
					my_comments: comments(where: {}) {
						id
						body
					}
					empty_rel: tags
				}
			}
			tenant_a_orders {
				id
				total
			}
		}
	`
	operationNode, parseErr := ParseGraphQL(query, nil)
	if parseErr != nil {
		t.Fatalf("parse error: %v", parseErr)
	}

	sqlQuery, compileErr := compiler.Compile(operationNode)
	if compileErr != nil {
		t.Fatalf("compiler error: %v", compileErr)
	}

	if !strings.Contains(sqlQuery.SQL, "json_build_object") || !strings.Contains(sqlQuery.SQL, "json_agg") {
		t.Fatalf("expected json_build_object and json_agg in compiled SQL: %s", sqlQuery.SQL)
	}

	// 3. Compile Mutations
	mutationDocument := `
		mutation {
			insert_users(objects: [{ name: "Bob", email: "bob@layr.sh" }]) {
				id
			}
			update_users(where: { id: "123" }, _set: { name: "Bob New" }) {
				id
			}
			delete_users(where: { id: "123" }) {
				id
			}
		}
	`
	mutationOperationNode, mutationParseErr := ParseGraphQL(mutationDocument, nil)
	if mutationParseErr != nil {
		t.Fatalf("parse mutation error: %v", mutationParseErr)
	}

	mutationSQLQuery, mutationCompileErr := compiler.Compile(mutationOperationNode)
	if mutationCompileErr != nil {
		t.Fatalf("compile mutation error: %v", mutationCompileErr)
	}

	if !strings.Contains(mutationSQLQuery.SQL, "WITH m0 AS (") || !strings.Contains(mutationSQLQuery.SQL, "INSERT INTO \"public\".\"users\"") {
		t.Fatalf("expected CTE mutation SQL: %s", mutationSQLQuery.SQL)
	}

	// 4. Compile error cases
	emptyOperationNode := &OperationNode{}
	if _, err := compiler.Compile(emptyOperationNode); err == nil {
		t.Fatal("expected error on empty selection set")
	}

	unsupportedMutationOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "unsupported_mutation"},
		},
	}
	if _, err := compiler.Compile(unsupportedMutationOperationNode); err == nil {
		t.Fatal("expected error on unsupported mutation")
	}

	invalidInsertOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "insert_users", Arguments: map[string]any{}},
		},
	}
	if _, err := compiler.Compile(invalidInsertOperationNode); err == nil {
		t.Fatal("expected error on insert without objects")
	}

	invalidInsertNonMapOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "insert_users", Arguments: map[string]any{"objects": []any{"string_not_map"}}},
		},
	}
	if _, err := compiler.Compile(invalidInsertNonMapOperationNode); err == nil {
		t.Fatal("expected error on insert non-map objects")
	}

	invalidUpdateOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "update_users", Arguments: map[string]any{}},
		},
	}
	if _, err := compiler.Compile(invalidUpdateOperationNode); err == nil {
		t.Fatal("expected error on update without _set")
	}

	invalidUpdateSetNonMapOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "update_users", Arguments: map[string]any{"_set": "not_a_map"}},
		},
	}
	if _, err := compiler.Compile(invalidUpdateSetNonMapOperationNode); err == nil {
		t.Fatal("expected error on update non-map _set")
	}

	invalidUpdateWhereOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "update_users", Arguments: map[string]any{"_set": map[string]any{"a": 1}, "where": "bad_where"}},
		},
	}
	if _, err := compiler.Compile(invalidUpdateWhereOperationNode); err == nil {
		t.Fatal("expected error on update bad where")
	}

	invalidDeleteWhereOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "delete_users", Arguments: map[string]any{"where": "bad_where"}},
		},
	}
	if _, err := compiler.Compile(invalidDeleteWhereOperationNode); err == nil {
		t.Fatal("expected error on delete bad where")
	}

	invalidRootWhereOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", Arguments: map[string]any{"where": "bad_where"}},
		},
	}
	if _, err := compiler.Compile(invalidRootWhereOperationNode); err == nil {
		t.Fatal("expected error on query bad where")
	}

	invalidNestedWhereOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "users",
				SelectionSet: []FieldNode{
					{Name: "posts", SelectionSet: []FieldNode{{Name: "id"}}, Arguments: map[string]any{"where": "bad_where"}},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidNestedWhereOperationNode); err == nil {
		t.Fatal("expected error on nested bad where")
	}

	invalidDeepNestedWhereOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "users",
				SelectionSet: []FieldNode{
					{
						Name: "posts",
						SelectionSet: []FieldNode{
							{Name: "comments", SelectionSet: []FieldNode{{Name: "id"}}, Arguments: map[string]any{"where": "bad_where"}},
						},
					},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidDeepNestedWhereOperationNode); err == nil {
		t.Fatal("expected error on deep nested bad where")
	}

	invalidRootNestedWhereOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "users",
				SelectionSet: []FieldNode{
					{
						Name:         "posts",
						SelectionSet: []FieldNode{{Name: "id"}},
						Arguments:    map[string]any{"where": "bad_where"},
					},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidRootNestedWhereOperationNode); err == nil {
		t.Fatal("expected error on root nested bad where")
	}

	emptyRootSelectionOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users"},
		},
	}
	if _, err := compiler.Compile(emptyRootSelectionOperationNode); err != nil {
		t.Fatalf("expected successful compile on empty root selection: %v", err)
	}

	schemaQualOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "public_users", SelectionSet: []FieldNode{{Name: "id"}}},
		},
	}
	if _, err := compiler.Compile(schemaQualOperationNode); err != nil {
		t.Fatalf("expected successful compile on schema-qualified root query: %v", err)
	}

	// 5. Test validation error branches in compiler
	invalidIdentifierOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "bad-table!"},
		},
	}
	if _, err := compiler.Compile(invalidIdentifierOperationNode); err == nil {
		t.Fatal("expected error on bad table identifier")
	}

	invalidAliasOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", Alias: "bad-alias!"},
		},
	}
	if _, err := compiler.Compile(invalidAliasOperationNode); err == nil {
		t.Fatal("expected error on bad alias identifier")
	}

	invalidSubAliasOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "users",
				SelectionSet: []FieldNode{
					{Name: "id", Alias: "bad-sub-alias!"},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidSubAliasOperationNode); err == nil {
		t.Fatal("expected error on bad sub alias")
	}

	invalidSubColumnOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "users",
				SelectionSet: []FieldNode{
					{Name: "bad-column!", Alias: "valid_alias"},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidSubColumnOperationNode); err == nil {
		t.Fatal("expected error on bad sub column")
	}

	invalidLimitOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", Arguments: map[string]any{"limit": "non_int"}},
		},
	}
	if _, err := compiler.Compile(invalidLimitOperationNode); err == nil {
		t.Fatal("expected error on non-int limit")
	}

	invalidOffsetOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", Arguments: map[string]any{"offset": "non_int"}},
		},
	}
	if _, err := compiler.Compile(invalidOffsetOperationNode); err == nil {
		t.Fatal("expected error on non-int offset")
	}

	floatLimitOffsetOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name:      "users",
				Arguments: map[string]any{"limit": float64(15), "offset": float64(5)},
				SelectionSet: []FieldNode{
					{Name: "id"},
				},
			},
		},
	}
	if _, err := compiler.Compile(floatLimitOffsetOperationNode); err != nil {
		t.Fatalf("expected success with float64 limit/offset: %v", err)
	}

	int64LimitOffsetOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name:      "users",
				Arguments: map[string]any{"limit": int64(15), "offset": int64(5)},
				SelectionSet: []FieldNode{
					{Name: "id"},
				},
			},
		},
	}
	if _, err := compiler.Compile(int64LimitOffsetOperationNode); err != nil {
		t.Fatalf("expected success with int64 limit/offset: %v", err)
	}

	intLimitOffsetOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name:      "users",
				Arguments: map[string]any{"limit": int(15), "offset": int(5)},
				SelectionSet: []FieldNode{
					{Name: "id"},
				},
			},
		},
	}
	if _, err := compiler.Compile(intLimitOffsetOperationNode); err != nil {
		t.Fatalf("expected success with int limit/offset: %v", err)
	}

	invalidRelationTableOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "users",
				SelectionSet: []FieldNode{
					{
						Name:         "bad-rel!",
						Alias:        "valid_alias",
						SelectionSet: []FieldNode{{Name: "id"}},
					},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidRelationTableOperationNode); err == nil {
		t.Fatal("expected error on bad rel table")
	}

	invalidNestedRelationTableOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "users",
				SelectionSet: []FieldNode{
					{
						Name: "posts",
						SelectionSet: []FieldNode{
							{
								Name:         "bad-table!",
								SelectionSet: []FieldNode{{Name: "id"}},
							},
						},
					},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidNestedRelationTableOperationNode); err == nil {
		t.Fatal("expected error on bad nested rel table")
	}

	invalidRelationSubAliasOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "users",
				SelectionSet: []FieldNode{
					{
						Name: "posts",
						SelectionSet: []FieldNode{
							{Name: "id", Alias: "bad-rel-alias!"},
						},
					},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidRelationSubAliasOperationNode); err == nil {
		t.Fatal("expected error on bad rel sub alias")
	}

	invalidRelationColumnOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "users",
				SelectionSet: []FieldNode{
					{
						Name: "posts",
						SelectionSet: []FieldNode{
							{Name: "bad-column!", Alias: "valid_alias"},
						},
					},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidRelationColumnOperationNode); err == nil {
		t.Fatal("expected error on bad relation column")
	}

	invalidWhereColumnOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name:      "users",
				Arguments: map[string]any{"where": map[string]any{"bad-column!": "value"}},
			},
		},
	}
	if _, err := compiler.Compile(invalidWhereColumnOperationNode); err == nil {
		t.Fatal("expected error on bad where column")
	}

	// Introspected FK join test
	schemaIntrospector.tables["public.posts"] = &TableInfo{
		Schema:     "public",
		Name:       "posts",
		PrimaryKey: "id",
		Columns: map[string]ColumnInfo{
			"id":      {Name: "id", IsPrimaryKey: true},
			"user_id": {Name: "user_id"},
		},
		ForeignKeys: map[string]RelationInfo{
			"users": {
				RelationName: "users",
				ForeignTable: "users",
				LocalColumn:  "user_id",
				TargetColumn: "id",
			},
		},
	}
	foreignKeyOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "users",
				SelectionSet: []FieldNode{
					{
						Name:         "posts",
						SelectionSet: []FieldNode{{Name: "id"}},
					},
				},
			},
		},
	}
	if _, err := compiler.Compile(foreignKeyOperationNode); err != nil {
		t.Fatalf("expected success with introspected FK relation: %v", err)
	}

	// 8. Test Mutation Identifiers Validation
	invalidInsertTableOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "insert_bad-table!",
				Arguments: map[string]any{
					"objects": []any{map[string]any{"id": "1"}},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidInsertTableOperationNode); err == nil {
		t.Fatal("expected error for bad table in insert mutation")
	}

	invalidInsertColumnOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "insert_users",
				Arguments: map[string]any{
					"objects": []any{map[string]any{"bad-column!": "1"}},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidInsertColumnOperationNode); err == nil {
		t.Fatal("expected error for bad column in insert mutation")
	}

	invalidUpdateTableOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "update_bad-table!",
				Arguments: map[string]any{
					"_set": map[string]any{"name": "test"},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidUpdateTableOperationNode); err == nil {
		t.Fatal("expected error for bad table in update mutation")
	}

	invalidUpdateColumnOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "update_users",
				Arguments: map[string]any{
					"_set": map[string]any{"bad-column!": "test"},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidUpdateColumnOperationNode); err == nil {
		t.Fatal("expected error for bad column in update mutation")
	}

	invalidDeleteTableOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "delete_bad-table!",
				Arguments: map[string]any{
					"where": map[string]any{"id": map[string]any{"_eq": "1"}},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidDeleteTableOperationNode); err == nil {
		t.Fatal("expected error for bad table in delete mutation")
	}

	invalidUpdateWhereMutationOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "update_users",
				Arguments: map[string]any{
					"_set":  map[string]any{"name": "test"},
					"where": map[string]any{"bad-where!": map[string]any{"_eq": "1"}},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidUpdateWhereMutationOperationNode); err == nil {
		t.Fatal("expected error for bad where in update mutation")
	}

	invalidDeleteWhereMutationOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "delete_users",
				Arguments: map[string]any{
					"where": map[string]any{"bad-where!": map[string]any{"_eq": "1"}},
				},
			},
		},
	}
	if _, err := compiler.Compile(invalidDeleteWhereMutationOperationNode); err == nil {
		t.Fatal("expected error for bad where in delete mutation")
	}

	// 9. Unsafe mutation without where clause rejected
	updateNoWhereOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "update_users", Arguments: map[string]any{"_set": map[string]any{"name": "Alice"}}},
		},
	}
	if _, err := compiler.Compile(updateNoWhereOperationNode); err == nil {
		t.Fatal("expected error for update without where")
	}

	deleteNoWhereOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "delete_users", Arguments: map[string]any{}},
		},
	}
	if _, err := compiler.Compile(deleteNoWhereOperationNode); err == nil {
		t.Fatal("expected error for delete without where")
	}

	// 10. Empty _in evaluates to FALSE
	emptyInQuery := `query { users(where: { id: { _in: [] } }) { id } }`
	emptyInOperationNode, parseErr := ParseGraphQL(emptyInQuery, nil)
	if parseErr != nil {
		t.Fatalf("parse error: %v", parseErr)
	}
	emptyInSQLQuery, compileErr := compiler.Compile(emptyInOperationNode)
	if compileErr != nil || !strings.Contains(emptyInSQLQuery.SQL, "FALSE") {
		t.Fatalf("expected FALSE in empty _in SQL: %s", emptyInSQLQuery.SQL)
	}

	// 11. order_by in root query and nested relation
	orderByQuery := `query { users(order_by: { name: "desc", id: "asc" }) { id posts(order_by: { title: "desc" }, limit: 5, offset: 2) { id } } }`
	orderByOperationNode, parseErr := ParseGraphQL(orderByQuery, nil)
	if parseErr != nil {
		t.Fatalf("parse error: %v", parseErr)
	}
	orderBySQLQuery, compileErr := compiler.Compile(orderByOperationNode)
	if compileErr != nil || !strings.Contains(orderBySQLQuery.SQL, "ORDER BY") || !strings.Contains(orderBySQLQuery.SQL, "LIMIT 5 OFFSET 2") {
		t.Fatalf("expected ORDER BY and nested limit/offset: %s", orderBySQLQuery.SQL)
	}

	// 12. Invalid order_by
	invalidOrderOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", Arguments: map[string]any{"order_by": "not_an_object"}},
		},
	}
	if _, err := compiler.Compile(invalidOrderOperationNode); err == nil {
		t.Fatal("expected error on non-object order_by")
	}

	invalidOrderColumnOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", Arguments: map[string]any{"order_by": map[string]any{"bad-col!": "asc"}}},
		},
	}
	if _, err := compiler.Compile(invalidOrderColumnOperationNode); err == nil {
		t.Fatal("expected error on bad column in order_by")
	}

	// Nested invalid order_by
	invalidNestedOrderOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", SelectionSet: []FieldNode{
				{Name: "posts", SelectionSet: []FieldNode{{Name: "id"}}, Arguments: map[string]any{"order_by": "bad"}},
			}},
		},
	}
	if _, err := compiler.Compile(invalidNestedOrderOperationNode); err == nil {
		t.Fatal("expected error on nested non-object order_by")
	}

	// 13. Nested relation limit and offset variations
	nestedLimitFloatOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", SelectionSet: []FieldNode{
				{Name: "posts", SelectionSet: []FieldNode{{Name: "id"}}, Arguments: map[string]any{"limit": float64(5), "offset": float64(1)}},
			}},
		},
	}
	if _, err := compiler.Compile(nestedLimitFloatOperationNode); err != nil {
		t.Fatalf("expected success with float limit in nested relation: %v", err)
	}

	nestedLimitInt64OperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", SelectionSet: []FieldNode{
				{Name: "posts", SelectionSet: []FieldNode{{Name: "id"}}, Arguments: map[string]any{"limit": int64(5), "offset": int64(1)}},
			}},
		},
	}
	if _, err := compiler.Compile(nestedLimitInt64OperationNode); err != nil {
		t.Fatalf("expected success with int64 limit in nested relation: %v", err)
	}

	nestedLimitBadOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", SelectionSet: []FieldNode{
				{Name: "posts", SelectionSet: []FieldNode{{Name: "id"}}, Arguments: map[string]any{"limit": "bad"}},
			}},
		},
	}
	if _, err := compiler.Compile(nestedLimitBadOperationNode); err == nil {
		t.Fatal("expected error on bad limit in nested relation")
	}

	nestedOffsetBadOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "users", SelectionSet: []FieldNode{
				{Name: "posts", SelectionSet: []FieldNode{{Name: "id"}}, Arguments: map[string]any{"offset": "bad"}},
			}},
		},
	}
	if _, err := compiler.Compile(nestedOffsetBadOperationNode); err == nil {
		t.Fatal("expected error on bad offset in nested relation")
	}

	// 14. Allowed schemas and excluded tables
	compiler.SetAllowedSchemas([]string{"public"})
	compiler.SetExcludedTables([]string{"secrets", "public.restricted"})

	if _, err := compiler.Compile(&OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "restricted", SelectionSet: []FieldNode{{Name: "id"}}},
		},
	}); err == nil {
		t.Fatal("expected error querying excluded table")
	}

	if _, err := compiler.Compile(&OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "tenant_a_orders", SelectionSet: []FieldNode{{Name: "id"}}},
		},
	}); err == nil {
		t.Fatal("expected error querying unallowed schema")
	}

	// Mutations with unallowed schema and excluded table
	if _, err := compiler.Compile(&OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "insert_restricted", Arguments: map[string]any{"objects": []any{map[string]any{"id": "1"}}}},
		},
	}); err == nil {
		t.Fatal("expected error inserting into excluded table")
	}

	if _, err := compiler.Compile(&OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "insert_tenant_a_orders", Arguments: map[string]any{"objects": []any{map[string]any{"id": "1"}}}},
		},
	}); err == nil {
		t.Fatal("expected error inserting into unallowed schema")
	}

	if _, err := compiler.Compile(&OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "update_restricted", Arguments: map[string]any{"_set": map[string]any{"a": 1}, "where": map[string]any{"id": "1"}}},
		},
	}); err == nil {
		t.Fatal("expected error updating excluded table")
	}

	if _, err := compiler.Compile(&OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "update_tenant_a_orders", Arguments: map[string]any{"_set": map[string]any{"a": 1}, "where": map[string]any{"id": "1"}}},
		},
	}); err == nil {
		t.Fatal("expected error updating unallowed schema")
	}

	if _, err := compiler.Compile(&OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "delete_restricted", Arguments: map[string]any{"where": map[string]any{"id": "1"}}},
		},
	}); err == nil {
		t.Fatal("expected error deleting from excluded table")
	}

	if _, err := compiler.Compile(&OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{Name: "delete_tenant_a_orders", Arguments: map[string]any{"where": map[string]any{"id": "1"}}},
		},
	}); err == nil {
		t.Fatal("expected error deleting from unallowed schema")
	}

	// Reset exclusions for subsequent tests
	compiler.SetAllowedSchemas(nil)
	compiler.SetExcludedTables(nil)

	// 15. Bidirectional relation (many-to-one reverse lookup)
	schemaIntrospector.tables["public.comments"] = &TableInfo{
		Schema:     "public",
		Name:       "comments",
		PrimaryKey: "id",
		Columns: map[string]ColumnInfo{
			"id":      {Name: "id", IsPrimaryKey: true},
			"post_id": {Name: "post_id"},
		},
		ForeignKeys: map[string]RelationInfo{
			"post": {
				RelationName:  "post",
				ForeignSchema: "public",
				ForeignTable:  "posts",
				LocalColumn:   "post_id",
				TargetColumn:  "id",
				IsArray:       false,
			},
		},
	}
	commentsOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "comments",
				SelectionSet: []FieldNode{
					{Name: "id"},
					{
						Name: "post",
						SelectionSet: []FieldNode{
							{Name: "id"},
						},
					},
				},
			},
		},
	}
	commentsSQLQuery, compileErr := compiler.Compile(commentsOperationNode)
	if compileErr != nil || !strings.Contains(commentsSQLQuery.SQL, "\"id\" = t0.\"post_id\"") {
		t.Fatalf("expected many-to-one join condition, got: %s, err: %v", commentsSQLQuery.SQL, compileErr)
	}

	// 16. Empty selection set on unintrospected table falls back to to_jsonb
	emptySelectOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{Name: "unintrospected_table"},
		},
	}
	emptySelectSQLQuery, compileErr := compiler.Compile(emptySelectOperationNode)
	if compileErr != nil || !strings.Contains(emptySelectSQLQuery.SQL, "to_jsonb(t0)") {
		t.Fatalf("expected to_jsonb(t0) for unintrospected table, got: %s, err: %v", emptySelectSQLQuery.SQL, compileErr)
	}

	// 17. Plural relation name mapping to singular table ending without s
	schemaIntrospector.tables["public.tag"] = &TableInfo{
		Schema:     "public",
		Name:       "tag",
		PrimaryKey: "id",
		Columns:    map[string]ColumnInfo{"id": {Name: "id", IsPrimaryKey: true}},
	}
	pluralRelationOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "posts",
				SelectionSet: []FieldNode{
					{
						Name: "tags",
						SelectionSet: []FieldNode{
							{Name: "id"},
						},
					},
				},
			},
		},
	}
	pluralRelationSQLQuery, compileErr := compiler.Compile(pluralRelationOperationNode)
	if compileErr != nil || !strings.Contains(pluralRelationSQLQuery.SQL, `"public"."tag"`) {
		t.Fatalf("expected singular table name resolution, got: %s, err: %v", pluralRelationSQLQuery.SQL, compileErr)
	}

	// 18. Nested relation with int typed limit and offset
	nestedIntLimitOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "posts",
				SelectionSet: []FieldNode{
					{
						Name: "comments",
						Arguments: map[string]any{
							"limit":  int(5),
							"offset": int(10),
						},
						SelectionSet: []FieldNode{{Name: "id"}},
					},
				},
			},
		},
	}
	nestedIntSQLQuery, compileErr := compiler.Compile(nestedIntLimitOperationNode)
	if compileErr != nil || !strings.Contains(nestedIntSQLQuery.SQL, "LIMIT 5 OFFSET 10") {
		t.Fatalf("expected LIMIT 5 OFFSET 10 in nested query, got: %s, err: %v", nestedIntSQLQuery.SQL, compileErr)
	}

	// 19. Empty order_by map and non-ASC/DESC direction
	emptyOrderByOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "posts",
				Arguments: map[string]any{
					"order_by": map[string]any{},
				},
				SelectionSet: []FieldNode{{Name: "id"}},
			},
		},
	}
	emptyOrderSQLQuery, compileErr := compiler.Compile(emptyOrderByOperationNode)
	if compileErr != nil || strings.Contains(emptyOrderSQLQuery.SQL, "ORDER BY") {
		t.Fatalf("expected no ORDER BY clause for empty order_by map, got: %s", emptyOrderSQLQuery.SQL)
	}

	customDirectionOperationNode := &OperationNode{
		Type: QueryOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "posts",
				Arguments: map[string]any{
					"order_by": map[string]any{"id": "CUSTOM_DIR"},
				},
				SelectionSet: []FieldNode{{Name: "id"}},
			},
		},
	}
	customDirectionSQLQuery, compileErr := compiler.Compile(customDirectionOperationNode)
	if compileErr != nil || !strings.Contains(customDirectionSQLQuery.SQL, "ORDER BY t0.\"id\" ASC") {
		t.Fatalf("expected default ASC for custom dir, got: %s, err: %v", customDirectionSQLQuery.SQL, compileErr)
	}

	// 20. Update and delete with empty where: {}
	if _, err := compiler.Compile(&OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "update_users",
				Arguments: map[string]any{
					"where": map[string]any{},
					"_set":  map[string]any{"name": "Alice"},
				},
			},
		},
	}); err == nil {
		t.Fatal("expected error on update with empty where map")
	}

	if _, err := compiler.Compile(&OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "delete_users",
				Arguments: map[string]any{
					"where": map[string]any{},
				},
			},
		},
	}); err == nil {
		t.Fatal("expected error on delete with empty where map")
	}

	// 21. Mutation return fields with alias and empty selection set
	aliasMutationOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "delete_users",
				Arguments: map[string]any{
					"where": map[string]any{"id": "1"},
				},
				SelectionSet: []FieldNode{
					{Name: "id", Alias: "user_id"},
				},
			},
		},
	}
	aliasMutationSQLQuery, compileErr := compiler.Compile(aliasMutationOperationNode)
	if compileErr != nil || !strings.Contains(aliasMutationSQLQuery.SQL, "'user_id', m0.\"id\"") {
		t.Fatalf("expected alias in mutation return fields, got: %s, err: %v", aliasMutationSQLQuery.SQL, compileErr)
	}

	emptyMutationSelectOperationNode := &OperationNode{
		Type: MutationOperationType,
		SelectionSet: []FieldNode{
			{
				Name: "delete_users",
				Arguments: map[string]any{
					"where": map[string]any{"id": "1"},
				},
			},
		},
	}
	emptyMutationSelectSQLQuery, compileErr := compiler.Compile(emptyMutationSelectOperationNode)
	if compileErr != nil || !strings.Contains(emptyMutationSelectSQLQuery.SQL, "to_json(m0)") {
		t.Fatalf("expected to_json(m0) when mutation has no selection set, got: %s, err: %v", emptyMutationSelectSQLQuery.SQL, compileErr)
	}
}
