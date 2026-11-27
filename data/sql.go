package data

import (
	"fmt"
	"strings"
	"unicode"
)

// validateSQLTransaction prevents a script from committing or detaching the
// transaction owned by the scratchpad. Quoted function bodies and comments are
// skipped; keyword matching alone would reject valid PostgreSQL scripts.
func validateSQLTransaction(sql string) error {
	start := true
	for position := 0; position < len(sql); {
		character := sql[position]
		if unicode.IsSpace(rune(character)) {
			position++
			continue
		}
		if strings.HasPrefix(sql[position:], "--") {
			end := strings.IndexAny(sql[position:], "\r\n")
			if end < 0 {
				return nil
			}
			position += end
			continue
		}
		if strings.HasPrefix(sql[position:], "/*") {
			position += 2
			depth := 1
			for position < len(sql) && depth > 0 {
				switch {
				case strings.HasPrefix(sql[position:], "/*"):
					depth++
					position += 2
				case strings.HasPrefix(sql[position:], "*/"):
					depth--
					position += 2
				default:
					position++
				}
			}
			if depth > 0 {
				return fmt.Errorf("unterminated SQL comment")
			}
			continue
		}
		if character == '\'' || character == '"' {
			escaped := character == '\'' && position > 0 && (sql[position-1] == 'e' || sql[position-1] == 'E') && (position == 1 || !sqlIdentifierByte(sql[position-2]))
			position++
			closed := false
			for position < len(sql) {
				if escaped && sql[position] == '\\' {
					position += 2
					continue
				}
				if sql[position] == character {
					position++
					if position < len(sql) && sql[position] == character {
						position++
						continue
					}
					closed = true
					break
				}
				position++
			}
			if !closed {
				return fmt.Errorf("unterminated SQL quote")
			}
			start = false
			continue
		}
		if character == '$' && (position == 0 || !sqlIdentifierByte(sql[position-1])) {
			end := strings.IndexByte(sql[position+1:], '$')
			if end >= 0 && validSQLDollarTag(sql[position+1:position+1+end]) {
				tag := sql[position : position+end+2]
				position += len(tag)
				closeIndex := strings.Index(sql[position:], tag)
				if closeIndex < 0 {
					return fmt.Errorf("unterminated SQL dollar quote")
				}
				position += closeIndex + len(tag)
				start = false
				continue
			}
		}
		if character == ';' {
			start = true
			position++
			continue
		}
		if start {
			end := position
			for end < len(sql) && (sql[end] >= 'a' && sql[end] <= 'z' || sql[end] >= 'A' && sql[end] <= 'Z' || sql[end] == '_') {
				end++
			}
			switch command := strings.ToUpper(sql[position:end]); command {
			case "BEGIN", "START", "COMMIT", "END", "ROLLBACK", "ABORT", "SAVEPOINT", "RELEASE", "PREPARE":
				return fmt.Errorf("explicit transaction control (%s) is not supported in the SQL scratchpad", command)
			}
			start = false
			if end > position {
				position = end
				continue
			}
		}
		position++
	}
	return nil
}

func sqlIdentifierByte(character byte) bool {
	return character >= 128 || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '$'
}

func validSQLDollarTag(tag string) bool {
	for index, character := range tag {
		if !unicode.IsLetter(character) && character != '_' && (index == 0 || !unicode.IsDigit(character)) {
			return false
		}
	}
	return true
}
