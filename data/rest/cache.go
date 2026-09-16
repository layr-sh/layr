// Package rest provides REST auto-CRUD querying, relational embedding, and caching.
package rest

import (
	"fmt"
	"strings"

	datakv "layr.sh/data/kv"
)

// GenerateUserVisibleRESTKey builds the user-visible key ("rest:<suffix>" or "rest:<sha256>").
func GenerateUserVisibleRESTKey(schema, table string, queryParams *QueryParams, customSuffix string) string {
	return GenerateUserVisibleRESTKeyWithVersion(schema, table, queryParams, customSuffix, 0)
}

// GenerateUserVisibleRESTKeyWithVersion builds the user-visible key scoped to a table cache version.
func GenerateUserVisibleRESTKeyWithVersion(schema, table string, queryParams *QueryParams, customSuffix string, tableVersion int64) string {
	var embeddedStrings []string
	for _, embedded := range queryParams.Embedded {
		embeddedStrings = append(embeddedStrings, formatEmbeddedField(embedded))
	}

	var filterStrings []string
	for _, filterOp := range queryParams.Filters {
		var valueString string
		if len(filterOp.Values) > 0 {
			valueString = strings.Join(filterOp.Values, ",")
		} else {
			valueString = filterOp.Value
		}
		filterStrings = append(filterStrings, fmt.Sprintf("%s.%s.%s", filterOp.Column, filterOp.Op, valueString))
	}

	var orderStrings []string
	for _, orderOp := range queryParams.Orders {
		orderStrings = append(orderStrings, fmt.Sprintf("%s.%t.%t", orderOp.Column, orderOp.Desc, orderOp.NullsLast))
	}

	return datakv.BuildRESTQueryKeyWithVersion(
		schema,
		table,
		tableVersion,
		queryParams.Fields,
		embeddedStrings,
		filterStrings,
		orderStrings,
		queryParams.Limit,
		queryParams.Offset,
		queryParams.CountExact,
		customSuffix,
	)
}

func formatEmbeddedField(embeddedField EmbeddedField) string {
	sortedFields := append([]string(nil), embeddedField.Fields...)
	var childParts []string
	for _, child := range embeddedField.Children {
		childParts = append(childParts, formatEmbeddedField(child))
	}
	return fmt.Sprintf("%s(%s){%s}", embeddedField.Relation, strings.Join(sortedFields, ","), strings.Join(childParts, ";"))
}
