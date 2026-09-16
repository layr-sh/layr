package rest

import (
	"reflect"
	"strings"
	"testing"
)

func TestRestBuilderSelectAndFiltersUnit(t *testing.T) {
	queryBuilder := NewQueryBuilder("public", "users")
	tableMetadata := TableMetadata{
		Schema:     "public",
		Table:      "users",
		PrimaryKey: "id",
		Columns:    []string{"id", "name", "email", "status"},
		ForeignKeys: map[string]RelationForeignKey{
			"posts": {
				FromColumn: "id",
				ToTable:    "posts",
				ToColumn:   "user_id",
			},
		},
	}

	// 1. Select all with fallback relation embedding
	queryParams := &QueryParams{
		Fields: []string{"*"},
		Embedded: []EmbeddedField{
			{Relation: "orders", Fields: []string{"id", "total"}},
		},
		Filters: []FilterOp{
			{Column: "status", Op: "eq", Value: "active"},
			{Column: "status", Op: "neq", Value: "locked"},
			{Column: "age", Op: "gt", Value: "18"},
			{Column: "age", Op: "gte", Value: "21"},
			{Column: "age", Op: "lt", Value: "80"},
			{Column: "age", Op: "lte", Value: "65"},
			{Column: "name", Op: "like", Value: "A%"},
			{Column: "email", Op: "ilike", Value: "%@domain.com"},
			{Column: "deleted_at", Op: "is", Value: "null"},
			{Column: "restored_at", Op: "is", Value: "not.null"},
			{Column: "role", Op: "in", Values: []string{"member", "editor"}},
			{Column: "tags", Op: "cs", Values: []string{"go", "sql"}},
			{Column: "bio", Op: "fts", Value: "developer", Extra: "english"},
		},
		Orders: []OrderBy{
			{Column: "created_at", Desc: true},
		},
		Limit:  10,
		Offset: 20,
	}

	selectSQLStatement, err := queryBuilder.BuildSelect(queryParams, tableMetadata)
	if err != nil {
		t.Fatalf("unexpected error building select: %v", err)
	}

	if len(selectSQLStatement.Args) != 12 {
		t.Fatalf("expected 12 args, got %d: %+v", len(selectSQLStatement.Args), selectSQLStatement.Args)
	}

	// 2. Count query
	countSQLStatement, err := queryBuilder.BuildCount(queryParams)
	if err != nil {
		t.Fatalf("unexpected error building count: %v", err)
	}
	if countSQLStatement.SQL == "" {
		t.Fatal("empty count SQL")
	}

	// 3. Select with defined ForeignKeys and with empty Fields slice
	secondQueryParams := &QueryParams{
		Fields: []string{"id", "name"},
		Embedded: []EmbeddedField{
			{Relation: "posts"},
		},
	}
	foreignKeySQLStatement, err := queryBuilder.BuildSelect(secondQueryParams, tableMetadata)
	if err != nil || foreignKeySQLStatement.SQL == "" {
		t.Fatalf("unexpected error building select with FK: %v", err)
	}

	emptyFieldsQueryParams := &QueryParams{Fields: []string{}}
	emptyFieldsSQLStatement, err := queryBuilder.BuildSelect(emptyFieldsQueryParams, tableMetadata)
	if err != nil || !strings.Contains(emptyFieldsSQLStatement.SQL, `"users".*`) {
		t.Fatalf("expected users.* in select with empty fields: %v", err)
	}

	starColumnsQueryParams := &QueryParams{Fields: []string{"*", "name"}}
	starColumnsSQLStatement, err := queryBuilder.BuildSelect(starColumnsQueryParams, tableMetadata)
	if err != nil || !strings.Contains(starColumnsSQLStatement.SQL, `"users".*, "users"."name"`) {
		t.Fatalf("expected users.*, users.name in select with star columns: %v", err)
	}

	nullsLastQueryParams := &QueryParams{
		Orders: []OrderBy{
			{Column: "created_at", Desc: true, NullsLast: true},
		},
	}
	nullsLastSQLStatement, err := queryBuilder.BuildSelect(nullsLastQueryParams, tableMetadata)
	if err != nil || !strings.Contains(nullsLastSQLStatement.SQL, "NULLS LAST") {
		t.Fatalf("expected NULLS LAST in SQL: %v", err)
	}

	// 4. BuildWhere unsupported clause error
	unsupportedQueryParams := &QueryParams{
		Filters: []FilterOp{
			{Column: "status", Op: "unsupported", Value: "active"},
		},
	}
	_, err = queryBuilder.BuildSelect(unsupportedQueryParams, tableMetadata)
	if err == nil {
		t.Fatal("expected error on unsupported clause")
	}
	_, err = queryBuilder.BuildCount(unsupportedQueryParams)
	if err == nil {
		t.Fatal("expected error on unsupported clause count")
	}
}

func TestRestBuilderInsertUpdateDeleteUnit(t *testing.T) {
	queryBuilder := NewQueryBuilder("", "documents")

	// 1. Insert single & bulk with on conflict and heterogeneous columns (DEFAULT placeholder)
	bulk := []map[string]any{
		{"title": "Doc 1", "author": "Alice"},
		{"title": "Doc 2"}, // missing author -> DEFAULT
	}
	insertSQLStatement, err := queryBuilder.BuildInsert(bulk, "title")
	if err != nil {
		t.Fatalf("unexpected insert error: %v", err)
	}
	if len(insertSQLStatement.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(insertSQLStatement.Args))
	}

	// 2. Insert with on_conflict when only 1 column (ON CONFLICT DO NOTHING)
	singleColumn := []map[string]any{
		{"id": "doc_1"},
	}
	insertNothingSQLStatement, err := queryBuilder.BuildInsert(singleColumn, "id")
	if err != nil || !reflect.DeepEqual(insertNothingSQLStatement.Args, []any{"doc_1"}) {
		t.Fatalf("unexpected insert nothing: %+v, err: %v", insertNothingSQLStatement, err)
	}

	// 3. Insert errors
	_, err = queryBuilder.BuildInsert(nil, "")
	if err == nil {
		t.Fatal("expected error on nil records insert")
	}
	_, err = queryBuilder.BuildInsert([]map[string]any{{"bad column!": "value"}}, "")
	if err == nil {
		t.Fatal("expected error on bad column insert")
	}
	_, err = queryBuilder.BuildInsert([]map[string]any{{}}, "")
	if err == nil {
		t.Fatal("expected error on empty map insert")
	}

	// 4. Update
	updateSQLStatement, err := queryBuilder.BuildUpdate(map[string]any{"title": "Updated"}, []FilterOp{{Column: "id", Op: "eq", Value: "123"}})
	if err != nil {
		t.Fatalf("unexpected error on BuildUpdate: %v", err)
	}
	if len(updateSQLStatement.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(updateSQLStatement.Args))
	}

	// Update errors
	_, err = queryBuilder.BuildUpdate(nil, nil)
	if err == nil {
		t.Fatal("expected error on nil update values")
	}
	_, err = queryBuilder.BuildUpdate(map[string]any{"bad column!": "value"}, nil)
	if err == nil {
		t.Fatal("expected error on bad column update")
	}
	_, err = queryBuilder.BuildUpdate(map[string]any{"title": "value"}, []FilterOp{{Column: "status", Op: "bad", Value: "value"}})
	if err == nil {
		t.Fatal("expected error on bad filter in update")
	}

	// 5. Delete
	deleteSQLStatement, err := queryBuilder.BuildDelete([]FilterOp{{Column: "id", Op: "eq", Value: "123"}})
	if err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
	if len(deleteSQLStatement.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(deleteSQLStatement.Args))
	}

	// Delete error
	_, err = queryBuilder.BuildDelete([]FilterOp{{Column: "bad column!", Op: "eq", Value: "value"}})
	if err == nil {
		t.Fatal("expected error on bad column in filter for delete")
	}
	_, err = queryBuilder.BuildDelete([]FilterOp{{Column: "status", Op: "bad", Value: "value"}})
	if err == nil {
		t.Fatal("expected error on bad filter in delete")
	}

	// 6. Negated filters and embedded projection coverage
	negatedFilters := []FilterOp{
		{Column: "c1", Op: "eq", Value: "v", Negated: true},
		{Column: "c2", Op: "neq", Value: "v", Negated: true},
		{Column: "c3", Op: "gt", Value: "10", Negated: true},
		{Column: "c4", Op: "gte", Value: "20", Negated: true},
		{Column: "c5", Op: "lt", Value: "30", Negated: true},
		{Column: "c6", Op: "lte", Value: "40", Negated: true},
		{Column: "c7", Op: "like", Value: "%a%", Negated: true},
		{Column: "c8", Op: "ilike", Value: "%b%", Negated: true},
		{Column: "c9", Op: "is", Value: "null", Negated: true},
		{Column: "c10", Op: "is", Value: "not.null", Negated: true},
		{Column: "c11", Op: "in", Values: []string{"1", "2"}, Negated: true},
		{Column: "c12", Op: "cs", Values: []string{"x"}, Negated: true},
		{Column: "c13", Op: "fts", Value: "phrase", Extra: "english", Negated: true},
	}
	negatedSQLStatement, err := queryBuilder.BuildSelect(&QueryParams{
		Filters: negatedFilters,
		Embedded: []EmbeddedField{
			{Relation: "comments", Fields: []string{"id", "body"}},
			{Relation: "tags", Fields: []string{"invalid-field!"}},
		},
	}, TableMetadata{PrimaryKey: ""})
	if err != nil {
		t.Fatalf("unexpected error building negated select: %v", err)
	}
	if !strings.Contains(negatedSQLStatement.SQL, "NOT IN") || !strings.Contains(negatedSQLStatement.SQL, "websearch_to_tsquery") {
		t.Fatalf("expected negated SQL constructs: %s", negatedSQLStatement.SQL)
	}
}
