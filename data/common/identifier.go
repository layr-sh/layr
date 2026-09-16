package common

import "unicode"

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
