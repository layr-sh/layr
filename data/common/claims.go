// Package common provides shared utilities for the data service.
package common

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// BuildClaimsMap constructs a unified map of all standard RFC 7519 and custom claims.
func BuildClaimsMap(jwtClaims core.JWTClaims) map[string]any {
	claimsMap := make(map[string]any)
	if jwtClaims.Subject != "" {
		claimsMap["sub"] = jwtClaims.Subject
	}
	if jwtClaims.SessionID != "" {
		claimsMap["sid"] = jwtClaims.SessionID
	}
	if jwtClaims.Role != "" {
		claimsMap["role"] = jwtClaims.Role
	}
	if jwtClaims.Issuer != "" {
		claimsMap["iss"] = jwtClaims.Issuer
	}
	if jwtClaims.Audience != "" {
		claimsMap["aud"] = jwtClaims.Audience
	}
	if jwtClaims.ExpiresAt != 0 {
		claimsMap["exp"] = jwtClaims.ExpiresAt
	}
	if jwtClaims.NotBefore != 0 {
		claimsMap["nbf"] = jwtClaims.NotBefore
	}
	if jwtClaims.IssuedAt != 0 {
		claimsMap["iat"] = jwtClaims.IssuedAt
	}
	if jwtClaims.JWTID != "" {
		claimsMap["jti"] = jwtClaims.JWTID
	}
	if jwtClaims.Email != "" {
		claimsMap["email"] = jwtClaims.Email
	}
	if jwtClaims.Phone != "" {
		claimsMap["phone"] = jwtClaims.Phone
	}
	if jwtClaims.IsAnonymous {
		claimsMap["is_anonymous"] = true
	}
	if jwtClaims.Scope != "" {
		claimsMap["scope"] = jwtClaims.Scope
	}
	for claimKey, claimValue := range jwtClaims.Claims {
		if IsSafeClaimKey(claimKey) {
			claimsMap[claimKey] = claimValue
		}
	}
	return claimsMap
}

// ApplyRLS configures PostgreSQL session variables within a transaction for RLS evaluation.
func ApplyRLS(ctx context.Context, tx pgx.Tx, jwtClaims core.JWTClaims) {
	claimsMap := BuildClaimsMap(jwtClaims)
	if len(claimsMap) == 0 {
		return
	}

	for claimKey, claimValue := range claimsMap {
		if IsSafeClaimKey(claimKey) {
			setting := fmt.Sprintf("request.jwt.%s", claimKey)
			var stringValue string
			switch typedValue := claimValue.(type) {
			case string:
				stringValue = typedValue
			case int64:
				stringValue = strconv.FormatInt(typedValue, 10)
			case int:
				stringValue = strconv.Itoa(typedValue)
			case float64:
				stringValue = strconv.FormatFloat(typedValue, 'f', -1, 64)
			case bool:
				stringValue = strconv.FormatBool(typedValue)
			case []string:
				stringValue = strings.Join(typedValue, " ")
			default:
				if jsonBytes, err := json.Marshal(claimValue); err == nil {
					stringValue = string(jsonBytes)
				} else {
					stringValue = fmt.Sprintf("%v", claimValue)
				}
			}
			_, _ = tx.Exec(ctx, "SELECT set_config($1, $2, true)", setting, stringValue)
		}
	}

	// Full JSON claims payload in request.jwt
	if claimsJSON, err := json.Marshal(claimsMap); err == nil {
		_, _ = tx.Exec(ctx, "SELECT set_config('request.jwt', $1, true)", string(claimsJSON))
	}
}

// IsSafeClaimKey asserts that a claim key is safe for PostgreSQL configuration names.
func IsSafeClaimKey(claimKey string) bool {
	if claimKey == "" || len(claimKey) > 63 {
		return false
	}
	for _, runeChar := range claimKey {
		if (runeChar >= 'a' && runeChar <= 'z') || (runeChar >= 'A' && runeChar <= 'Z') || (runeChar >= '0' && runeChar <= '9') || runeChar == '_' || runeChar == '-' {
			continue
		}
		return false
	}
	return true
}
