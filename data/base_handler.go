package data

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"layr.sh/core"
)

// BaseHandler coordinates all public data plane HTTP routes (REST, Ephemeral KV, GraphQL, and Realtime CDC).
type BaseHandler struct {
	*Service
}

// NewBaseHandler initializes the BaseHandler with service coordinator.
func NewBaseHandler(service *Service) *BaseHandler {
	return &BaseHandler{
		Service: service,
	}
}

// writeDBErrorResponse maps PostgreSQL errors to appropriate RFC 9457 HTTP responses.
func (handler *BaseHandler) writeDBErrorResponse(responseWriter http.ResponseWriter, request *http.Request, err error) {
	statusCode := http.StatusInternalServerError
	message := "Service temporarily unavailable"
	debugLog := err.Error()

	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "42501": // insufficient_privilege
			statusCode = http.StatusForbidden
			message = "Access denied"
		case "23505": // unique_violation
			statusCode = http.StatusConflict
			message = "Resource already exists"
		case "23503", "23001": // foreign_key_violation or restrict_violation
			statusCode = http.StatusConflict
			message = "Invalid reference"
		case "42P01", "42883": // undefined_table or undefined_function
			statusCode = http.StatusNotFound
			message = "Resource not found"
		case "42703": // undefined_column
			statusCode = http.StatusBadRequest
			message = "Invalid field specified"
		}
	}

	core.WriteErrorResponse(responseWriter, request, statusCode, message, debugLog)
}

// isRLSBypassed returns true if the request caller has the necessary service account scope to bypass RLS.
func (handler *BaseHandler) isRLSBypassed(request *http.Request, requiredScope string) bool {
	authContext := core.GetAuthContext(request.Context())
	if authContext.IsServiceAccount() && authContext.HasScope(requiredScope) {
		return true
	}
	secretKey := core.ExtractRequestServiceAccountKey(request)
	if secretKey == "" {
		return false
	}
	clientIP := core.ExtractRequestClientIP(request)
	serviceAccount, err := handler.kernel.ServiceAccountManager().Authenticate(request.Context(), secretKey, clientIP)
	if err != nil {
		return false
	}
	return core.HasScope(serviceAccount.Scopes, requiredScope)
}

// resolveTransaction commits or rolls back a transaction based on the error state.
func (handler *BaseHandler) resolveTransaction(ctx context.Context, tx pgx.Tx, err error) error {
	if tx == nil {
		return err
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

const (
	minPathSegments         = 2
	minRecordIDPathSegments = 3
)

// parsePath extracts schema, table, and optional record_id from the request URL.
func (handler *BaseHandler) parsePath(path string) (string, string, string, error) {
	trimmed := strings.TrimPrefix(path, "/v1/data/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < minPathSegments || parts[0] == "" || parts[1] == "" {
		return "", "", "", errors.New("path must include schema and table name")
	}
	recordID := ""
	if len(parts) >= minRecordIDPathSegments {
		recordID = parts[2]
	}
	return parts[0], parts[1], recordID, nil
}
