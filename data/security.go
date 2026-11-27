package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"layr.sh/data/realtime"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
	"layr.sh/data/common"
)

const dataQueryRole = "layr_data_client"

func (service *Service) tableExposed(schema, table string) bool {
	if IsProtectedSchema(schema) {
		return false
	}
	config := service.configManager.Get()
	allowed := false
	for _, candidate := range config.Schemas {
		if candidate == schema {
			allowed = true
			break
		}
	}
	if !allowed {
		return false
	}
	for _, excluded := range config.REST.ExcludedTables {
		if excluded == table || excluded == schema+"."+table {
			return false
		}
	}
	return true
}

func (handler *BaseHandler) authorizeRealtimeEvent(request *http.Request, cdcEvent realtime.CDCEvent) (realtime.CDCEvent, bool) {
	authContext := core.GetAuthContext(request.Context())
	if !handler.configManager.Get().Realtime.Enabled || !handler.tableExposed(cdcEvent.Schema, cdcEvent.Table) {
		return cdcEvent, false
	}
	if authContext.JWT.ExpiresAt > 0 && time.Now().Unix() >= authContext.JWT.ExpiresAt {
		return cdcEvent, false
	}
	const eventAuthorizationTimeout = 3 * time.Second
	ctx, cancel := context.WithTimeout(context.WithoutCancel(request.Context()), eventAuthorizationTimeout)
	defer cancel()
	if authContext.JWT.SessionID != "" {
		var active bool
		var query string
		if authContext.IsServiceAccount() && authContext.JWT.Audience == "layr:console:user" {
			query = `SELECT EXISTS(SELECT 1 FROM console.sessions s JOIN console.users u ON u.id=s.user_id WHERE s.id::text=$1 AND s.user_id::text=$2 AND s.expires_at>clock_timestamp() AND u.is_enabled)`
		} else {
			query = `SELECT EXISTS(SELECT 1 FROM auth.sessions s JOIN auth.users u ON u.id=s.user_id WHERE s.id::text=$1 AND s.user_id::text=$2 AND s.expires_at>clock_timestamp() AND (u.locked_until IS NULL OR u.locked_until<=clock_timestamp()) AND (COALESCE((SELECT (value->'sessions'->>'idle_timeout_seconds')::int FROM auth.config WHERE key='runtime'),604800)<=0 OR s.last_used_at>clock_timestamp()-make_interval(secs=>COALESCE((SELECT (value->'sessions'->>'idle_timeout_seconds')::int FROM auth.config WHERE key='runtime'),604800))))`
		}
		if err := handler.kernel.DB().QueryRow(ctx, query, authContext.JWT.SessionID, authContext.JWT.Subject).Scan(&active); err != nil || !active {
			return cdcEvent, false
		}
	}
	if authContext.IsServiceAccount() {
		serviceAccount, err := handler.kernel.ServiceAccountManager().Get(ctx, authContext.ServiceAccountID)
		if err != nil || !serviceAccount.IsEnabled || serviceAccount.ExpiresAt != nil && !serviceAccount.ExpiresAt.After(time.Now()) {
			return cdcEvent, false
		}
		if len(serviceAccount.AllowedIPs) > 0 && !core.IsIPAllowed(core.ExtractRequestClientIP(request), serviceAccount.AllowedIPs) {
			return cdcEvent, false
		}
		if core.HasScope(serviceAccount.Scopes, core.ScopeDataQueryRead) {
			return cdcEvent, true
		}
	}
	if cdcEvent.Truncated && len(cdcEvent.Record) == 0 {
		return cdcEvent, false
	}
	var rls bool
	if err := handler.kernel.DB().QueryRow(ctx, `SELECT c.relrowsecurity FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, cdcEvent.Schema, cdcEvent.Table).Scan(&rls); err != nil {
		return cdcEvent, false
	}
	// Deleted rows cannot be re-authorized by PostgreSQL RLS. Only privileged
	// subscribers receive their payloads when RLS is enabled.
	if cdcEvent.Event == "DELETE" && rls {
		return cdcEvent, false
	}
	tx, err := handler.kernel.DB().Begin(ctx)
	if err != nil {
		return cdcEvent, false
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := applyQueryRLS(ctx, tx, authContext.JWT); err != nil {
		return cdcEvent, false
	}
	table := pgx.Identifier{cdcEvent.Schema, cdcEvent.Table}.Sanitize()
	if cdcEvent.Event == "DELETE" {
		if _, err := tx.Exec(ctx, "SELECT 1 FROM "+table+" LIMIT 0"); err != nil {
			return cdcEvent, false
		}
	} else {
		record, err := json.Marshal(cdcEvent.Record)
		if err != nil {
			return cdcEvent, false
		}
		var visible bool
		// Matching the cdcEvent image also prevents leaking an older owner's payload
		// when the live row changes ownership before the cdcEvent is dispatched.
		query := "SELECT EXISTS(SELECT 1 FROM " + table + " AS event_row WHERE to_jsonb(event_row) @> $1::jsonb)"
		if err := tx.QueryRow(ctx, query, record).Scan(&visible); err != nil || !visible {
			return cdcEvent, false
		}
	}
	cdcEvent.OldRecord = nil
	return cdcEvent, true
}

// applyQueryRLS prevents the schema-management connection's ownership or
// superuser privileges from bypassing policies on application requests.
func applyQueryRLS(ctx context.Context, tx pgx.Tx, jwtClaims core.JWTClaims) error {
	common.ApplyRLS(ctx, tx, jwtClaims)
	_, err := tx.Exec(ctx, "SET LOCAL ROLE "+dataQueryRole)
	return err
}

func (service *Service) grantQuerySchemas(ctx context.Context, schemas []string) error {
	rows, err := service.kernel.DB().Query(ctx, `SELECT nspname FROM pg_namespace WHERE nspname = ANY($1)`, schemas)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		if !IsProtectedSchema(name) {
			names = append(names, name)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, name := range names {
		schema := quoteIdent(name)
		sql := fmt.Sprintf(`GRANT USAGE ON SCHEMA %s TO %[2]s;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %[1]s TO %[2]s;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA %[1]s TO %[2]s;
ALTER DEFAULT PRIVILEGES IN SCHEMA %[1]s GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %[2]s;
ALTER DEFAULT PRIVILEGES IN SCHEMA %[1]s GRANT USAGE, SELECT ON SEQUENCES TO %[2]s;`, schema, dataQueryRole)
		if _, err := service.kernel.DB().Exec(ctx, sql); err != nil {
			return fmt.Errorf("configure data query privileges for %s: %w", name, err)
		}
	}
	return nil
}
