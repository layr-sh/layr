// Package kv provides key-value storage and caching mechanisms for data queries.
package kv

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"layr.sh/core"
)

// DefaultSaltSecret is the fallback salt used for visitor hashing.
const DefaultSaltSecret = "data-cache-salt"

// AuthContext captures security identity and privacy-preserving visitor attribution.
type AuthContext struct {
	Role        string
	Subject     string
	VisitorHash string
}

// ExtractClientIP extracts the real client IP address from request headers or remote address.
func ExtractClientIP(request *http.Request) string {
	return core.ExtractRequestClientIP(request)
}

// ComputeVisitorHash generates a privacy-preserving daily rotating visitor hash.
func ComputeVisitorHash(clientIP, userAgent string, now time.Time, saltSecret string) string {
	if saltSecret == "" {
		saltSecret = DefaultSaltSecret
	}
	bucket := now.Unix() / (24 * 3600)
	rawString := fmt.Sprintf("%s:%s:%s:%d", clientIP, saltSecret, userAgent, bucket)
	hash := sha256.Sum256([]byte(rawString))
	return hex.EncodeToString(hash[:])
}

// ExtractAuthContext inspects claims and headers to build the caller AuthContext.
func ExtractAuthContext(request *http.Request, saltSecret string) AuthContext {
	jwtClaims := core.GetAuthContext(request.Context()).JWT
	if jwtClaims.Subject != "" || (jwtClaims.Role != "" && jwtClaims.Role != "anon") {
		return AuthContext{
			Role:        jwtClaims.Role,
			Subject:     jwtClaims.Subject,
			VisitorHash: "",
		}
	}

	role := jwtClaims.Role
	if role == "" {
		role = "anon"
	}

	clientIP := ExtractClientIP(request)
	userAgent := request.Header.Get("User-Agent")
	visitorHash := ComputeVisitorHash(clientIP, userAgent, time.Now().UTC(), saltSecret)

	return AuthContext{
		Role:        role,
		Subject:     "",
		VisitorHash: visitorHash,
	}
}

// BuildInternalKey constructs the internal isolated KVStore key for any user-visible key.
func BuildInternalKey(authContext AuthContext, userVisibleKey string) string {
	isCache := strings.HasPrefix(userVisibleKey, "rest:") || strings.HasPrefix(userVisibleKey, "graphql:")

	if authContext.Role != "anon" && authContext.VisitorHash == "" {
		role := authContext.Role
		if role == "" {
			role = "authenticated"
		}
		if authContext.Subject != "" {
			if isCache {
				return fmt.Sprintf("cache:%s:%s:%s", role, authContext.Subject, userVisibleKey)
			}
			return fmt.Sprintf("%s:%s:%s", role, authContext.Subject, userVisibleKey)
		}
		if isCache {
			return fmt.Sprintf("cache:%s::%s", role, userVisibleKey)
		}
		return fmt.Sprintf("%s::%s", role, userVisibleKey)
	}

	role := authContext.Role
	if role == "" {
		role = "anon"
	}
	if isCache {
		return fmt.Sprintf("cache:%s:%s:%s", role, authContext.VisitorHash, userVisibleKey)
	}
	return fmt.Sprintf("%s:%s:%s", role, authContext.VisitorHash, userVisibleKey)
}

// RESTQueryTuple represents the canonical hashing structure for REST queries.
type RESTQueryTuple struct {
	Schema       string `json:"schema"`
	Table        string `json:"table"`
	TableVersion int64  `json:"table_version,omitempty"`
	Select       string `json:"select"`
	Filters      string `json:"filters"`
	Order        string `json:"order"`
	Limit        int    `json:"limit"`
	Offset       int    `json:"offset"`
	CountExact   bool   `json:"count_exact"`
}

// BuildRESTQueryKey builds the user-visible REST query key ("rest:<suffix>" or "rest:<sha256>").
func BuildRESTQueryKey(schema, table string, fields []string, embeddedStrings []string, filterStrings []string, orderStrings []string, limit, offset int, countExact bool, customSuffix string) string {
	return BuildRESTQueryKeyWithVersion(schema, table, 0, fields, embeddedStrings, filterStrings, orderStrings, limit, offset, countExact, customSuffix)
}

// BuildRESTQueryKeyWithVersion builds the user-visible REST query key scoped to a table cache version.
func BuildRESTQueryKeyWithVersion(schema, table string, tableVersion int64, fields []string, embeddedStrings []string, filterStrings []string, orderStrings []string, limit, offset int, countExact bool, customSuffix string) string {
	if customSuffix != "" {
		return fmt.Sprintf("rest:%s", customSuffix)
	}

	var selectParts []string
	if len(fields) > 0 {
		sortedFields := append([]string(nil), fields...)
		sort.Strings(sortedFields)
		selectParts = append(selectParts, strings.Join(sortedFields, ","))
	}
	selectParts = append(selectParts, embeddedStrings...)
	sort.Strings(selectParts)
	normSelect := strings.Join(selectParts, ";")

	sortedFilters := append([]string(nil), filterStrings...)
	sort.Strings(sortedFilters)
	normFilters := strings.Join(sortedFilters, "&")

	normOrder := strings.Join(orderStrings, ",")

	restQueryTuple := RESTQueryTuple{
		Schema:       schema,
		Table:        table,
		TableVersion: tableVersion,
		Select:       normSelect,
		Filters:      normFilters,
		Order:        normOrder,
		Limit:        limit,
		Offset:       offset,
		CountExact:   countExact,
	}

	tupleJSON, _ := json.Marshal(restQueryTuple)
	hash := sha256.Sum256(tupleJSON)
	return fmt.Sprintf("rest:%x", hash)
}

// GraphQLQueryTuple represents the canonical hashing structure for GraphQL queries.
type GraphQLQueryTuple struct {
	Query        string         `json:"query"`
	Variables    map[string]any `json:"variables,omitempty"`
	TableVersion int64          `json:"table_version,omitempty"`
}

// BuildGraphQLQueryKey builds the user-visible GraphQL query key ("graphql:<suffix>" or "graphql:<sha256>").
func BuildGraphQLQueryKey(query string, variables map[string]any, customSuffix string) string {
	return BuildGraphQLQueryKeyWithVersion(query, variables, customSuffix, 0)
}

// BuildGraphQLQueryKeyWithVersion builds the user-visible GraphQL query key scoped to a table cache version.
func BuildGraphQLQueryKeyWithVersion(query string, variables map[string]any, customSuffix string, tableVersion int64) string {
	if customSuffix != "" {
		return fmt.Sprintf("graphql:%s", customSuffix)
	}

	graphQLQueryTuple := GraphQLQueryTuple{
		Query:        strings.TrimSpace(query),
		Variables:    variables,
		TableVersion: tableVersion,
	}

	tupleJSON, _ := json.Marshal(graphQLQueryTuple)
	hash := sha256.Sum256(tupleJSON)
	return fmt.Sprintf("graphql:%x", hash)
}
