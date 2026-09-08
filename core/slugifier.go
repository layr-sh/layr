package core

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mozillazg/go-unidecode"
)

const (
	defaultWordSeparator               = "-"
	defaultInvalidCharacterReplacement = "-"
	defaultAllowedSet                  = "a-zA-Z0-9"
)

// Slugifier creates clean, URL-friendly identifier slugs from arbitrary text.
type Slugifier struct {
	skipToLower                 bool    // convert to lower or not
	wordSeparator               *string // separator char between words
	allowedSet                  *string // allowed chars, as a regex set (without [])
	invalidCharacterReplacement *string // replacement for illegal chars
	invalidCharacterPattern     *regexp.Regexp
	duplicateSeparatorPattern   *regexp.Regexp
	initialized                 bool
}

// NewSlugifier creates a new slugifier. Defaults to lowercasing and using a dash ("-") as the word separator and
// as the invalid character replacement.
func NewSlugifier() Slugifier {
	slugifier := Slugifier{}
	slugifier.InvalidCharacter(defaultInvalidCharacterReplacement)
	slugifier.WordSeparator(defaultWordSeparator)
	slugifier.initialize()
	return slugifier
}

// initialize initializes the slugifier. This is called automatically by Slugify() if it hasn't already been called.
func (slugifier *Slugifier) initialize() {
	if slugifier.invalidCharacterReplacement == nil {
		slugifier.InvalidCharacter(defaultInvalidCharacterReplacement)
	}

	if slugifier.wordSeparator == nil {
		separator := defaultWordSeparator
		slugifier.wordSeparator = &separator
	}

	if slugifier.allowedSet == nil {
		allowed := defaultAllowedSet
		slugifier.allowedSet = &allowed
	}

	separatorPattern := regexp.QuoteMeta(*slugifier.wordSeparator)
	slugifier.invalidCharacterPattern = regexp.MustCompile(fmt.Sprintf("[^%s"+*slugifier.allowedSet+"]", separatorPattern))
	if separatorPattern != "" {
		slugifier.duplicateSeparatorPattern = regexp.MustCompile(fmt.Sprintf("%s{2,}", separatorPattern))
	} else {
		slugifier.duplicateSeparatorPattern = nil
	}

	slugifier.initialized = true
}

// ToLower sets the flag indicating if the slugified result should be lowercased.
// Returns the slugifier for easy chaining.
func (slugifier *Slugifier) ToLower(toLower bool) *Slugifier {
	slugifier.skipToLower = !toLower
	return slugifier
}

// WordSeparator sets the word separator character to use. Defaults to a dash ("-"). The word separator is used to
// replace whitespace. Leading and trailing word separators are trimmed. Multiple successive word separators are
// replaced with a single word separator.
// Returns the slugifier for easy chaining.
func (slugifier *Slugifier) WordSeparator(wordSeparator string) *Slugifier {
	slugifier.wordSeparator = &wordSeparator
	slugifier.invalidCharacterPattern = nil
	slugifier.duplicateSeparatorPattern = nil
	slugifier.initialized = false
	return slugifier
}

// AllowedSet sets the allowed set of characters. Defaults to "a-zA-Z0-9". The allowed set is used to replace
// characters that are not in the allowed set with the invalid character replacement.
// It must be a valid regex set (typically in [] brackets), any characters that need escaping must
// be properly escaped. The word separator is automatically added to the allowed set.
func (slugifier *Slugifier) AllowedSet(allowedSet string) *Slugifier {
	slugifier.allowedSet = &allowedSet
	slugifier.invalidCharacterPattern = nil
	slugifier.duplicateSeparatorPattern = nil
	slugifier.initialized = false
	return slugifier
}

// InvalidCharacter sets the character to use to replace invalid characters (anything not a-z, A-Z, 0-9, the
// word separator, or the InvalidCharacter). Defaults to a dash ("-"). Leading and trailing
// InvalidCharacterReplacements are trimmed. Multiple successive InvalidCharacterReplacements are NOT replaced with a single
// InvalidCharacter.
// Returns the slugifier for easy chaining.
func (slugifier *Slugifier) InvalidCharacter(invalidCharacterReplacement string) *Slugifier {
	slugifier.invalidCharacterReplacement = &invalidCharacterReplacement
	return slugifier
}

// Slugify implements making a pretty slug from the given text.
// e.g. Slugify("kožušček hello world") => "kozuscek-hello-world"
func (slugifier *Slugifier) Slugify(text string) string {
	if !slugifier.initialized {
		slugifier.initialize()
	}
	text = unidecode.Unidecode(text)
	text = strings.Join(strings.Fields(text), *slugifier.wordSeparator)
	text = slugifier.invalidCharacterPattern.ReplaceAllString(text, *slugifier.invalidCharacterReplacement)
	if slugifier.duplicateSeparatorPattern != nil {
		text = slugifier.duplicateSeparatorPattern.ReplaceAllString(text, *slugifier.wordSeparator)
	}

	// trim leading and trailing word separators and invalidCharacterReplacements
	for len(text) > 0 &&
		(string(text[0]) == *slugifier.wordSeparator || string(text[0]) == *slugifier.invalidCharacterReplacement) {
		text = text[1:]
	}
	for len(text) > 0 &&
		(string(text[len(text)-1]) == *slugifier.wordSeparator || string(text[len(text)-1]) == *slugifier.invalidCharacterReplacement) {
		text = text[:len(text)-1]
	}

	if !slugifier.skipToLower {
		text = strings.ToLower(text)
	}
	return text
}
