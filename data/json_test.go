package data

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestDataExactJSONDecodingUnit(t *testing.T) {
	var payload map[string]any
	if err := decodeDataJSON(strings.NewReader(`{"integer":9007199254740993,"decimal":1234567890.123456789,"items":[1,1.25,null]}`), &payload); err != nil {
		t.Fatal(err)
	}
	normalizeDataNumbers(payload)
	if payload["integer"] != int64(9007199254740993) || payload["decimal"] != json.Number("1234567890.123456789") {
		t.Fatal("JSON values lost precision")
	}
	args := bindDataArguments([]any{json.Number("9007199254740993"), json.Number("1234567890.123456789"), true, payload})
	decimalJSON, err := json.Marshal(args[1])
	if err != nil {
		t.Fatal(err)
	}
	if args[0] != int64(9007199254740993) || string(decimalJSON) != "1234567890.123456789" || args[2] != true {
		t.Fatalf("inexact SQL arguments: %+v", args)
	}
	if bindDataValue(json.Number("not-a-number")) != "not-a-number" {
		t.Fatal("invalid numeric fallback changed")
	}
	for _, input := range []string{`{`, `{} {}`, `{} trailing`} {
		t.Run(input, func(t *testing.T) {
			if err := decodeDataJSON(strings.NewReader(input), &payload); err == nil {
				t.Fatal("invalid JSON accepted")
			}
		})
	}
	for _, testCase := range []struct {
		name   string
		oid    uint32
		format int16
		raw    []byte
	}{
		{"json", pgTypeOIDJSON, pgx.TextFormatCode, []byte(`{"number":9007199254740993}`)},
		{"jsonb text", pgTypeOIDJSONB, pgx.TextFormatCode, []byte(`{"number":9007199254740993}`)},
		{"jsonb binary", pgTypeOIDJSONB, pgx.BinaryFormatCode, append([]byte{1}, []byte(`{"number":9007199254740993}`)...)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			value := queryJSONColumn(nil, pgconn.FieldDescription{DataTypeOID: testCase.oid, Format: testCase.format}, testCase.raw).(json.RawMessage)
			if string(value) != `{"number":9007199254740993}` {
				t.Fatalf("inexact JSON column: %s", value)
			}
			testCase.raw[len(testCase.raw)-1] = '!'
			if !json.Valid(value) {
				t.Fatal("row result retained pgx-owned memory")
			}
		})
	}
	if queryJSONColumn(int64(5), pgconn.FieldDescription{}, nil) != int64(5) {
		t.Fatal("scalar column changed")
	}
}
