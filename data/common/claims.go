// Package common provides shared utilities for the data service.
package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"layr.sh/auth/jwt"
)

// AuthClaims is an alias for jwt.Claims to avoid duplicate claim definitions across layr.
type AuthClaims = jwt.Claims

// BuildClaimsMap constructs a unified map of all standard RFC 7519 and custom claims.
func BuildClaimsMap(authClaims jwt.Claims) map[string]any {
	claimsMap := make(map[string]any)
	if authClaims.Subject != "" {
		claimsMap["sub"] = authClaims.Subject
	}
	if authClaims.Role != "" {
		claimsMap["role"] = authClaims.Role
	}
	if authClaims.Issuer != "" {
		claimsMap["iss"] = authClaims.Issuer
	}
	if authClaims.Audience != "" {
		claimsMap["aud"] = authClaims.Audience
	}
	if authClaims.ExpiresAt != 0 {
		claimsMap["exp"] = authClaims.ExpiresAt
	}
	if authClaims.NotBefore != 0 {
		claimsMap["nbf"] = authClaims.NotBefore
	}
	if authClaims.IssuedAt != 0 {
		claimsMap["iat"] = authClaims.IssuedAt
	}
	if authClaims.JWTID != "" {
		claimsMap["jti"] = authClaims.JWTID
	}
	if authClaims.Email != "" {
		claimsMap["email"] = authClaims.Email
	}
	if authClaims.Phone != "" {
		claimsMap["phone"] = authClaims.Phone
	}
	if authClaims.IsAnonymous {
		claimsMap["is_anonymous"] = true
	}
	if len(authClaims.Scopes) > 0 {
		claimsMap["scopes"] = authClaims.Scopes
	}
	for claimKey, claimValue := range authClaims.Claims {
		if IsSafeClaimKey(claimKey) {
			claimsMap[claimKey] = claimValue
		}
	}
	return claimsMap
}

// ApplyRLS configures PostgreSQL session variables within a transaction for RLS evaluation.
func ApplyRLS(ctx context.Context, tx pgx.Tx, authClaims jwt.Claims) {
	claimsMap := BuildClaimsMap(authClaims)
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

// ExtractClaims reads JWT identity headers injected by upstream auth middleware.
func ExtractClaims(request *http.Request) jwt.Claims {
	authClaims := jwt.Claims{
		Claims: make(map[string]any),
	}

	if sub := request.Header.Get("X-JWT-Sub"); sub != "" {
		authClaims.Subject = sub
	}
	if role := request.Header.Get("X-JWT-Role"); role != "" {
		authClaims.Role = role
	}
	if iss := request.Header.Get("X-JWT-Iss"); iss != "" {
		authClaims.Issuer = iss
	}
	if aud := request.Header.Get("X-JWT-Aud"); aud != "" {
		authClaims.Audience = aud
	}
	if exp := request.Header.Get("X-JWT-Exp"); exp != "" {
		if parsed, err := strconv.ParseInt(exp, 10, 64); err == nil {
			authClaims.ExpiresAt = parsed
		}
	}
	if nbf := request.Header.Get("X-JWT-Nbf"); nbf != "" {
		if parsed, err := strconv.ParseInt(nbf, 10, 64); err == nil {
			authClaims.NotBefore = parsed
		}
	}
	if iat := request.Header.Get("X-JWT-Iat"); iat != "" {
		if parsed, err := strconv.ParseInt(iat, 10, 64); err == nil {
			authClaims.IssuedAt = parsed
		}
	}
	if jti := request.Header.Get("X-JWT-Jti"); jti != "" {
		authClaims.JWTID = jti
	}
	if email := request.Header.Get("X-JWT-Email"); email != "" {
		authClaims.Email = email
	}
	if phone := request.Header.Get("X-JWT-Phone"); phone != "" {
		authClaims.Phone = phone
	}
	if isAnon := request.Header.Get("X-JWT-Is-Anonymous"); isAnon == "true" {
		authClaims.IsAnonymous = true
	}
	if scopes := request.Header.Get("X-JWT-Scopes"); scopes != "" {
		authClaims.Scopes = strings.Split(scopes, " ")
	}

	for headerKey, headerValues := range request.Header {
		lowerHeaderKey := strings.ToLower(headerKey)
		if strings.HasPrefix(lowerHeaderKey, "x-jwt-claim-") && len(headerValues) > 0 {
			claimKey := strings.TrimPrefix(lowerHeaderKey, "x-jwt-claim-")
			headerValue := headerValues[0]
			switch claimKey {
			case "sub":
				if authClaims.Subject == "" {
					authClaims.Subject = headerValue
				}
			case "role":
				if authClaims.Role == "" {
					authClaims.Role = headerValue
				}
			case "iss":
				if authClaims.Issuer == "" {
					authClaims.Issuer = headerValue
				}
			case "aud":
				if authClaims.Audience == "" {
					authClaims.Audience = headerValue
				}
			case "exp":
				if authClaims.ExpiresAt == 0 {
					if parsed, err := strconv.ParseInt(headerValue, 10, 64); err == nil {
						authClaims.ExpiresAt = parsed
					}
				}
			case "nbf":
				if authClaims.NotBefore == 0 {
					if parsed, err := strconv.ParseInt(headerValue, 10, 64); err == nil {
						authClaims.NotBefore = parsed
					}
				}
			case "iat":
				if authClaims.IssuedAt == 0 {
					if parsed, err := strconv.ParseInt(headerValue, 10, 64); err == nil {
						authClaims.IssuedAt = parsed
					}
				}
			case "jti":
				if authClaims.JWTID == "" {
					authClaims.JWTID = headerValue
				}
			case "email":
				if authClaims.Email == "" {
					authClaims.Email = headerValue
				}
			case "phone":
				if authClaims.Phone == "" {
					authClaims.Phone = headerValue
				}
			case "is_anonymous":
				authClaims.IsAnonymous = (headerValue == "true")
			case "scopes":
				authClaims.Scopes = strings.Split(headerValue, " ")
			default:
				authClaims.Claims[claimKey] = headerValue
			}
		}
	}

	return authClaims
}
