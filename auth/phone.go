package auth

import (
	"errors"
	"strings"
	"unicode"
)

// Named constants for ITU-T E.164 phone digit length bounds.
const (
	minPhoneDigits = 8
	maxPhoneDigits = 15
)

var (
	// ErrInvalidPhone is returned when a phone number is not valid E.164 format.
	ErrInvalidPhone = errors.New("invalid phone number: must be in international E.164 format (+<country_code><number>) with 8 to 15 digits")
)

// NormalizePhone standardizes a raw phone number into ITU-T E.164 format (+<country_code><number>).
// It strips whitespace, hyphens, parentheses, and dots, asserts a leading '+', validates
// that only digits follow, and asserts that the digit count is between 8 and 15 digits.
func NormalizePhone(rawPhone string) (string, error) {
	log.Tracef("NormalizePhone called with raw input length: %d", len(rawPhone))

	trimmedPhone := strings.TrimSpace(rawPhone)
	if trimmedPhone == "" {
		log.Debugf("phone normalization rejected: empty or whitespace-only input")
		return "", ErrInvalidPhone
	}

	if !strings.HasPrefix(trimmedPhone, "+") {
		log.Debugf("phone normalization rejected: missing leading '+' prefix in %q", trimmedPhone)
		return "", ErrInvalidPhone
	}

	var builder strings.Builder
	builder.WriteByte('+')

	digitCount := 0
	for _, character := range trimmedPhone[1:] {
		switch character {
		case ' ', '\t', '\r', '\n', '-', '(', ')', '.':
			continue
		default:
			if unicode.IsDigit(character) {
				builder.WriteRune(character)
				digitCount++
			} else {
				log.Debugf("phone normalization rejected: invalid non-digit character %q", character)
				return "", ErrInvalidPhone
			}
		}
	}

	if digitCount < minPhoneDigits || digitCount > maxPhoneDigits {
		log.Debugf("phone normalization rejected: digit count %d outside valid range [%d, %d]", digitCount, minPhoneDigits, maxPhoneDigits)
		return "", ErrInvalidPhone
	}

	normalizedPhone := builder.String()
	log.Tracef("phone normalization succeeded: %s", normalizedPhone)
	return normalizedPhone, nil
}
