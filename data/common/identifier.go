package common

import (
	"strings"
	"unicode"
)

// IsProtectedSchema identifies platform and PostgreSQL schemas that public data APIs must never expose.
func IsProtectedSchema(schema string) bool {
	schema = strings.ToLower(strings.TrimSpace(schema))
	switch schema {
	case "system", "core", "auth", "data", "file_storage", "tasks", "image", "function", "console", "information_schema":
		return true
	}
	return strings.HasPrefix(schema, "pg_")
}

// IsValidIdentifier validates that an identifier follows PostgreSQL naming conventions (letter or underscore followed by letters, digits, underscores, max 63 chars).
func IsValidIdentifier(identifier string) bool {
	if identifier == "" || len(identifier) > 63 {
		return false
	}
	for index, runeChar := range identifier {
		if index == 0 {
			if !unicode.IsLetter(runeChar) && runeChar != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(runeChar) && !unicode.IsDigit(runeChar) && runeChar != '_' {
				return false
			}
		}
	}
	return true
}
