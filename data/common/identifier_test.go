package common

import (
	"strings"
	"testing"
)

func TestCommonIdentifierValidationUnit(t *testing.T) {
	// 1. Length constraints: empty string and exceeding 63 chars
	if IsValidIdentifier("") {
		t.Fatal("expected empty identifier to be invalid")
	}
	if IsValidIdentifier(strings.Repeat("a", 64)) {
		t.Fatal("expected 64-char identifier to be invalid")
	}
	if !IsValidIdentifier(strings.Repeat("a", 63)) {
		t.Fatal("expected 63-char identifier to be valid")
	}

	// 2. Initial character constraints
	// Must start with letter or underscore
	validStarts := []string{"users", "Users", "_private", "_123"}
	for _, identifier := range validStarts {
		if !IsValidIdentifier(identifier) {
			t.Fatalf("expected valid initial character for %q", identifier)
		}
	}

	// Cannot start with digits or special characters
	invalidStarts := []string{"1users", "9table", "-column", "$data", " users", ".table"}
	for _, identifier := range invalidStarts {
		if IsValidIdentifier(identifier) {
			t.Fatalf("expected invalid initial character for %q", identifier)
		}
	}

	// 3. Subsequent character constraints
	// Allowed: letters, digits, underscores
	validIdentifiers := []string{
		"users_table",
		"table_2026_archive",
		"_internal_id_1",
		"CamelCaseColumn",
		"lowercase",
		"UPPERCASE",
		"mix_123_ABC",
	}
	for _, identifier := range validIdentifiers {
		if !IsValidIdentifier(identifier) {
			t.Fatalf("expected valid identifier for %q", identifier)
		}
	}

	// Forbidden: dashes, spaces, punctuation, symbols
	invalidIdentifiers := []string{
		"users-table",
		"users table",
		"users.table",
		"table/name",
		"column;drop",
		"column'quote",
		"column\"double",
		"data@domain",
		"amount$total",
	}
	for _, identifier := range invalidIdentifiers {
		if IsValidIdentifier(identifier) {
			t.Fatalf("expected invalid identifier for %q", identifier)
		}
	}
}
