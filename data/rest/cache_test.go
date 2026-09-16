package rest

import (
	"strings"
	"testing"
)

func TestRestCacheGenerateKeyUnit(t *testing.T) {
	queryParams := &QueryParams{
		Fields: []string{"id", "title"},
		Embedded: []EmbeddedField{
			{
				Relation: "author",
				Fields:   []string{"id", "name"},
				Children: []EmbeddedField{
					{
						Relation: "profile",
						Fields:   []string{"bio"},
					},
				},
			},
		},
		Filters: []FilterOp{
			{Column: "status", Op: "eq", Value: "published"},
			{Column: "tags", Op: "in", Values: []string{"tech", "news"}},
		},
		Orders: []OrderBy{
			{Column: "created_at", Desc: true, NullsLast: true},
		},
		Limit:      20,
		Offset:     40,
		CountExact: true,
	}

	// 1. Custom suffix override
	customKey := GenerateUserVisibleRESTKey("public", "articles", queryParams, "my_custom_cache_key")
	if customKey != "rest:my_custom_cache_key" {
		t.Fatalf("expected custom key rest:my_custom_cache_key, got: %s", customKey)
	}

	// 2. Computed SHA256 key
	computedKey := GenerateUserVisibleRESTKey("public", "articles", queryParams, "")
	if !strings.HasPrefix(computedKey, "rest:") || len(computedKey) != 5+64 {
		t.Fatalf("expected rest:<sha256> format, got: %s", computedKey)
	}

	// 3. Empty queryParams
	emptyParamsKey := GenerateUserVisibleRESTKey("public", "articles", &QueryParams{}, "")
	if !strings.HasPrefix(emptyParamsKey, "rest:") {
		t.Fatalf("expected valid key for empty query params, got: %s", emptyParamsKey)
	}

	// 4. Test formatEmbeddedField with empty fields
	emptyEmbeddedField := EmbeddedField{
		Relation: "comments",
		Fields:   nil,
	}
	formatted := formatEmbeddedField(emptyEmbeddedField)
	if formatted != "comments(){}" {
		t.Fatalf("expected comments(){}, got: %s", formatted)
	}
}
