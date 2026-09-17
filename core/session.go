package core

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// SessionCookieNameSecure is the standard isolated __Host cookie for application users over HTTPS.
	SessionCookieNameSecure = "__Host-session"
	// SessionCookieNameInsecure is the fallback cookie used over non-HTTPS/plain HTTP connections.
	SessionCookieNameInsecure = "session"
)

type authContextKey struct{}

// AuthContext represents authenticated caller identity and session state in request context across all Layr services.
type AuthContext struct {
	UserID           string    `json:"user_id,omitempty"`
	ServiceAccountID string    `json:"service_account_id,omitempty"`
	JWT              JWTClaims `json:"jwt"`
	RefreshTokenHash string    `json:"refresh_token_hash,omitempty"`
}

// Role returns the caller's canonical role from JWT claims.
func (authContext AuthContext) Role() string {
	return authContext.JWT.Role
}

// HasScope checks if the authenticated context satisfies the required scope.
func (authContext AuthContext) HasScope(requiredScope string) bool {
	return authContext.JWT.HasScope(requiredScope)
}

// IsAuthenticated returns true if the request was successfully authenticated as a user or service account.
func (authContext AuthContext) IsAuthenticated() bool {
	return authContext.UserID != "" || authContext.ServiceAccountID != ""
}

// IsUser returns true if the authenticated caller is an end user account (not a service account).
func (authContext AuthContext) IsUser() bool {
	return authContext.UserID != ""
}

// IsServiceAccount returns true if the authenticated caller is a service account / M2M caller.
func (authContext AuthContext) IsServiceAccount() bool {
	return authContext.ServiceAccountID != "" || authContext.JWT.Role == "service_role" || strings.HasSuffix(authContext.JWT.Audience, ":service_account")
}

// WithAuthContext stores the authenticated AuthContext in context.
func WithAuthContext(ctx context.Context, authContext AuthContext) context.Context {
	isServiceAccount := authContext.ServiceAccountID != "" || authContext.JWT.Role == "service_role" || strings.HasSuffix(authContext.JWT.Audience, ":service_account")
	if isServiceAccount {
		if authContext.ServiceAccountID == "" && authContext.JWT.Subject != "" {
			authContext.ServiceAccountID = authContext.JWT.Subject
		}
		authContext.UserID = "" // Service account must NEVER have UserID populated!
		if authContext.JWT.Role == "" {
			authContext.JWT.Role = "service_role"
		}
	} else if authContext.JWT.Subject != "" {
		if authContext.UserID == "" {
			authContext.UserID = authContext.JWT.Subject
		}
		if authContext.JWT.Role == "" {
			authContext.JWT.Role = "authenticated"
		}
		authContext.ServiceAccountID = ""
	} else if authContext.UserID != "" {
		if authContext.JWT.Role == "" {
			authContext.JWT.Role = "authenticated"
		}
		authContext.ServiceAccountID = ""
	}
	return context.WithValue(ctx, authContextKey{}, authContext)
}

// GetAuthContext retrieves the AuthContext from context if present, or an empty AuthContext.
func GetAuthContext(ctx context.Context) AuthContext {
	if ctx == nil {
		return AuthContext{}
	}
	if authContext, ok := ctx.Value(authContextKey{}).(AuthContext); ok {
		return authContext
	}
	return AuthContext{}
}

// IsSecureRequest determines if an incoming HTTP request is over TLS or terminated HTTPS.
func IsSecureRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	if request.TLS != nil {
		return true
	}
	return strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https")
}

// ExtractRequestClientIP extracts the canonical client IP from headers (X-Forwarded-For, X-Real-IP) or RemoteAddr.
func ExtractRequestClientIP(request *http.Request) string {
	if request == nil {
		return "127.0.0.1"
	}
	trustProxy := true
	if activeConfig := GetConfig(); activeConfig != nil {
		trustProxy = activeConfig.Server.TrustProxyHeaders
	}
	if trustProxy {
		if forwardedFor := request.Header.Get("X-Forwarded-For"); forwardedFor != "" {
			parts := strings.Split(forwardedFor, ",")
			if len(parts) > 0 {
				ipAddress := strings.TrimSpace(parts[0])
				if ipAddress != "" {
					return ipAddress
				}
			}
		}
		if realIP := request.Header.Get("X-Real-IP"); realIP != "" {
			ipAddress := strings.TrimSpace(realIP)
			if ipAddress != "" {
				return ipAddress
			}
		}
	}
	if request.RemoteAddr != "" {
		host, _, err := net.SplitHostPort(request.RemoteAddr)
		if err == nil && host != "" {
			return host
		}
		trimmedAddress := strings.Trim(strings.TrimSpace(request.RemoteAddr), "[]")
		if trimmedAddress != "" {
			return trimmedAddress
		}
	}
	return "127.0.0.1"
}

// SetSessionCookie writes the session cookie to the response based on request security (HTTPS vs HTTP).
func SetSessionCookie(responseWriter http.ResponseWriter, request *http.Request, refreshToken string, expiresAt time.Time) {
	isSecure := IsSecureRequest(request)
	cookieName := SessionCookieNameInsecure
	if isSecure {
		cookieName = SessionCookieNameSecure
	}
	log.Tracef("setting session cookie %s (secure: %t)", cookieName, isSecure)
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    refreshToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure,
	}
	http.SetCookie(responseWriter, cookie)
}

// ClearSessionCookie clears session cookies across secure and insecure variants.
func ClearSessionCookie(responseWriter http.ResponseWriter, request *http.Request) {
	isSecure := IsSecureRequest(request)
	log.Tracef("clearing session cookies (secure: %t)", isSecure)
	cookieNames := []string{SessionCookieNameInsecure}
	if isSecure {
		cookieNames = append(cookieNames, SessionCookieNameSecure)
	}
	for _, name := range cookieNames {
		cookie := &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   name == SessionCookieNameSecure || isSecure,
		}
		http.SetCookie(responseWriter, cookie)
	}
}

// ExtractRequestSessionToken extracts session/access token from Bearer header or standard session cookies.
func ExtractRequestSessionToken(request *http.Request) string {
	if request == nil {
		return ""
	}
	if authHeader := request.Header.Get("Authorization"); authHeader != "" {
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			token := strings.TrimSpace(authHeader[7:])
			if token != "" {
				return token
			}
		}
	}
	if cookie, err := request.Cookie(SessionCookieNameSecure); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	if cookie, err := request.Cookie(SessionCookieNameInsecure); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}
