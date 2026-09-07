package core

import (
	"net"
	"net/http"
	"strings"
	"time"
)

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

// SetSessionCookie sets a secure __Host- cookie over HTTPS or a fallback plain cookie over HTTP.
func SetSessionCookie(responseWriter http.ResponseWriter, secureName, plainName, token string, expiresAt time.Time, isSecure bool) {
	cookieName := plainName
	if isSecure {
		cookieName = secureName
	}
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure,
	}
	http.SetCookie(responseWriter, cookie)
}

// ClearSessionCookie clears both secure and insecure session cookies.
func ClearSessionCookie(responseWriter http.ResponseWriter, secureName, plainName string, isSecure bool) {
	cookieNames := []string{plainName}
	if isSecure {
		cookieNames = append(cookieNames, secureName)
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
			Secure:   name == secureName || isSecure,
		}
		http.SetCookie(responseWriter, cookie)
	}
}

// ExtractRequestSessionToken extracts a session token from Authorization Bearer headers or specified cookies.
func ExtractRequestSessionToken(request *http.Request, secureName, plainName string) string {
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
	if secureName != "" {
		if cookie, err := request.Cookie(secureName); err == nil && cookie.Value != "" {
			return cookie.Value
		}
	}
	if plainName != "" {
		if cookie, err := request.Cookie(plainName); err == nil && cookie.Value != "" {
			return cookie.Value
		}
	}
	return ""
}
