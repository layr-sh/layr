package data

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func decodeDataJSON(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode data JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON value")
	}
	return nil
}

func bindDataArguments(args []any) []any {
	result := make([]any, len(args))
	for index, value := range args {
		result[index] = bindDataValue(value)
	}
	return result
}

func bindDataValue(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer
		}
		var numeric pgtype.Numeric
		if err := numeric.Scan(typed.String()); err == nil {
			return numeric
		}
		return typed.String()
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = bindDataValue(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = bindDataValue(item)
		}
		return result
	}
	return value
}

func normalizeDataNumbers(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer
		}
	case map[string]any:
		for key, item := range typed {
			typed[key] = normalizeDataNumbers(item)
		}
	case []any:
		for index, item := range typed {
			typed[index] = normalizeDataNumbers(item)
		}
	}
	return value
}

func queryJSONColumn(value any, fieldDescription pgconn.FieldDescription, raw []byte) any {
	if raw != nil && (fieldDescription.DataTypeOID == pgTypeOIDJSON || fieldDescription.DataTypeOID == pgTypeOIDJSONB) {
		if fieldDescription.DataTypeOID == pgTypeOIDJSONB && fieldDescription.Format == pgx.BinaryFormatCode {
			raw = raw[1:]
		}
		// pgx's decoded JSON values use float64. Keep the wire JSON instead, and copy
		// it because RawValues storage is only valid until the next row is read.
		return append(json.RawMessage(nil), raw...)
	}
	return queryJSONValue(value, fieldDescription.DataTypeOID)
}
