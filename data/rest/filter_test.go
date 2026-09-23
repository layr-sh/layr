package rest

import (
	"fmt"
	"strings"
	"testing"

	"layr.sh/data/common"
)

func TestRestFilterParseQueryParamsUnit(t *testing.T) {
	// 1. Standard valid query
	urlValues := map[string][]string{
		"select":           {"id,name,posts(id,title)"},
		"order":            {"created_at.desc,id.asc.nullslast"},
		"limit":            {"25"},
		"offset":           {"50"},
		"count":            {"exact"},
		"on_conflict":      {"email"},
		"status":           {"eq.active"},
		"age":              {"gte.21", "lte.65"},
		"email":            {"ilike.*@company.com"},
		"tags":             {"cs.{typescript,go}"},
		"role":             {"in.(member,editor)"},
		"deleted_at":       {"is.null"},
		"bio":              {"fts(english).developer"},
		"is_active":        {"neq.false"},
		"score":            {"gt.10"},
		"points":           {"lt.100"},
		"name":             {"like.John*"},
		"archived_at":      {"is.not.null"},
		"cache_ttl":        {"60"},
		"cache_key":        {"user-list"},
		"cache_key_suffix": {"v1"},
	}

	queryParams, err := ParseQueryParams(urlValues, 50, 1000)
	if err != nil {
		t.Fatalf("unexpected error parsing valid params: %v", err)
	}

	if queryParams.Limit != 25 || queryParams.Offset != 50 || !queryParams.CountExact || queryParams.OnConflict != "email" {
		t.Fatalf("unexpected basic fields: %+v", queryParams)
	}

	if len(queryParams.Orders) != 2 || queryParams.Orders[0].Column != "created_at" || !queryParams.Orders[0].Desc || !queryParams.Orders[1].NullsLast {
		t.Fatalf("unexpected orders: %+v", queryParams.Orders)
	}

	if len(queryParams.Fields) != 2 || queryParams.Fields[0] != "id" || queryParams.Fields[1] != "name" {
		t.Fatalf("unexpected fields: %+v", queryParams.Fields)
	}

	if len(queryParams.Embedded) != 1 || queryParams.Embedded[0].Relation != "posts" {
		t.Fatalf("unexpected embedded: %+v", queryParams.Embedded)
	}

	// 2. Test limit clamping to maxLimit and valid asc order
	clampedQueryParams, err := ParseQueryParams(map[string][]string{"limit": {"5000"}, "offset": {"10"}, "order": {"name.asc"}}, 50, 1000)
	if err != nil || clampedQueryParams.Limit != 1000 || clampedQueryParams.Offset != 10 || clampedQueryParams.Orders[0].Desc {
		t.Fatalf("expected limit clamped to 1000, offset 10, got limit %d, offset %d (err: %v)", clampedQueryParams.Limit, clampedQueryParams.Offset, err)
	}

	// 3. Test select *
	starQueryParams, err := ParseQueryParams(map[string][]string{"select": {"*"}}, 50, 1000)
	if err != nil || len(starQueryParams.Fields) != 1 || starQueryParams.Fields[0] != "*" {
		t.Fatalf("expected select * to yield [*], got %+v", starQueryParams.Fields)
	}

	// Test empty valueList and double comma in order
	emptyValueQueryParams, err := ParseQueryParams(map[string][]string{"empty": {}}, 50, 1000)
	if err != nil {
		t.Fatalf("expected no error on empty valueList: %v", err)
	}
	_ = emptyValueQueryParams

	doubleCommaQueryParams, err := ParseQueryParams(map[string][]string{"order": {"id.asc,,created_at.desc.nullslast"}}, 50, 1000)
	if err != nil || len(doubleCommaQueryParams.Orders) != 2 {
		t.Fatalf("expected 2 orders parsed: %v", err)
	}

	// 4. Test error cases
	errorCases := []map[string][]string{
		{"select": {"invalid relation("}},
		{"select": {"invalid-field-name!"}},
		{"select": {"posts(invalid-sub-field!)"}},
		{"select": {"123badrel(id)"}},
		{"order": {"bad column!"}},
		{"limit": {"not-a-number"}},
		{"limit": {"-5"}},
		{"offset": {"not-a-number"}},
		{"offset": {"-10"}},
		{"on_conflict": {"bad-id!"}},
		{"bad-column!": {"eq.1"}},
		{"column": {"missingdot"}},
		{"column": {"is.invalid"}},
		{"column": {"in.no_parens"}},
		{"column": {"cs.no_braces"}},
		{"column": {"fts(bad-lang!).query"}},
		{"column": {"not.missingdot"}},
		{"column": {"unknownop.value"}},
	}

	for i, errorCase := range errorCases {
		t.Run(fmt.Sprintf("ErrorCase_%d", i), func(t *testing.T) {
			var parseErr error
			_, parseErr = ParseQueryParams(errorCase, 50, 1000)
			if parseErr == nil {
				t.Fatalf("case %d: expected error for %+v, got nil", i, errorCase)
			}
		})
	}

	// 5. Test negated operators
	negatedQueryParams, parseErr := ParseQueryParams(map[string][]string{
		"status": {"not.eq.closed"},
		"id":     {"not.in.(1,2)"},
		"name":   {"not.like.*draft*"},
	}, 50, 1000)
	if parseErr != nil || len(negatedQueryParams.Filters) != 3 {
		t.Fatalf("failed to parse negated filters: %v", parseErr)
	}
	for _, filterOp := range negatedQueryParams.Filters {
		t.Run(filterOp.Column, func(t *testing.T) {
			if !filterOp.Negated {
				t.Fatalf("expected filter %s to be negated", filterOp.Column)
			}
		})
	}

	// 5. Test isValidIdentifier edge cases
	if common.IsValidIdentifier("") {
		t.Fatal("expected empty identifier to be invalid")
	}
	if common.IsValidIdentifier(strings.Repeat("a", 64)) {
		t.Fatal("expected >63 char identifier to be invalid")
	}
	if common.IsValidIdentifier("1badstart") {
		t.Fatal("expected digit start identifier to be invalid")
	}
	if common.IsValidIdentifier("bad-char!") {
		t.Fatal("expected special char identifier to be invalid")
	}
	if !common.IsValidIdentifier("valid_Col123") {
		t.Fatal("expected valid_Col123 to be valid")
	}
}
