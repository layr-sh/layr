package data

import (
	"context"
	"layr.sh/auth"
	"layr.sh/core"
	"layr.sh/data/realtime"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDataRealtimeRowAuthorizationIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	service := NewService(kernel)
	ctx := context.Background()
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer service.Stop()
	if _, err := kernel.DB().Exec(ctx, `CREATE TABLE public.review_events(id integer PRIMARY KEY, owner text); INSERT INTO public.review_events VALUES(1,'alice'),(2,'bob'); ALTER TABLE public.review_events ENABLE ROW LEVEL SECURITY; CREATE POLICY read_owner ON public.review_events FOR SELECT USING(owner=current_setting('request.jwt.sub', true));`); err != nil {
		t.Fatal(err)
	}
	userCtx := core.WithAuthContext(ctx, core.AuthContext{UserID: "alice", JWT: core.JWTClaims{Subject: "alice", Role: "authenticated"}})
	request := httptest.NewRequestWithContext(userCtx, http.MethodGet, "/v1/realtime", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	cdcEvent := realtime.CDCEvent{Schema: "public", Table: "review_events", Event: "INSERT", Record: map[string]any{"id": 1, "owner": "alice"}}
	authorize := func(valueCDCEvent realtime.CDCEvent) bool {
		_, allowed := service.BaseHandler().authorizeRealtimeEvent(request, valueCDCEvent)
		return allowed
	}
	if !authorize(cdcEvent) {
		t.Fatal("owner could not receive its visible row")
	}
	hiddenCDCEvent := cdcEvent
	hiddenCDCEvent.Record = map[string]any{"id": 2, "owner": "bob"}
	if authorize(hiddenCDCEvent) {
		t.Fatal("another owner's event leaked")
	}
	updateCDCEvent := cdcEvent
	updateCDCEvent.Event = "UPDATE"
	updateCDCEvent.OldRecord = map[string]any{"owner": "bob", "secret": "old private data"}
	deliveredCDCEvent, allowed := service.BaseHandler().authorizeRealtimeEvent(request, updateCDCEvent)
	if !allowed || deliveredCDCEvent.OldRecord != nil {
		t.Fatal("old row image was not redacted")
	}
	if _, err := kernel.DB().Exec(ctx, `UPDATE public.review_events SET owner='bob' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if authorize(cdcEvent) {
		t.Fatal("stale ownership event leaked")
	}
	deletedCDCEvent := cdcEvent
	deletedCDCEvent.Event = "DELETE"
	if authorize(deletedCDCEvent) {
		t.Fatal("an RLS-protected deleted row leaked")
	}
	missingCDCEvent := cdcEvent
	missingCDCEvent.Table = "missing_events"
	if authorize(missingCDCEvent) {
		t.Fatal("missing table event accepted")
	}
	protectedCDCEvent := cdcEvent
	protectedCDCEvent.Schema = "auth"
	if authorize(protectedCDCEvent) {
		t.Fatal("protected schema exposed")
	}
	unexposedCDCEvent := cdcEvent
	unexposedCDCEvent.Schema = "unexposed"
	if authorize(unexposedCDCEvent) {
		t.Fatal("unconfigured schema exposed")
	}
	config := service.configManager.Get()
	config.REST.ExcludedTables = []string{"public.review_events"}
	service.configManager.SetMemoryConfig(config)
	if authorize(cdcEvent) {
		t.Fatal("qualified excluded table was exposed")
	}
	config.REST.ExcludedTables = []string{"review_events"}
	service.configManager.SetMemoryConfig(config)
	if authorize(cdcEvent) {
		t.Fatal("excluded table was exposed")
	}
	config.REST.ExcludedTables = nil
	config.Realtime.Enabled = false
	service.configManager.SetMemoryConfig(config)
	if authorize(cdcEvent) {
		t.Fatal("disabled realtime was ignored")
	}
	config.Realtime.Enabled = true
	service.configManager.SetMemoryConfig(config)
	expiredCtx := core.WithAuthContext(ctx, core.AuthContext{JWT: core.JWTClaims{Subject: "alice", ExpiresAt: time.Now().Add(-time.Minute).Unix()}})
	if _, allowed := service.BaseHandler().authorizeRealtimeEvent(request.WithContext(expiredCtx), cdcEvent); allowed {
		t.Fatal("expired token accepted")
	}
	invalidCDCEvent := cdcEvent
	invalidCDCEvent.Record = map[string]any{"value": func() {}}
	if authorize(invalidCDCEvent) {
		t.Fatal("non-serializable event accepted")
	}
	serviceAccount, err := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{Name: "realtime-console-user", Scopes: []string{core.ScopeDataQueryRead}})
	if err != nil {
		t.Fatal(err)
	}
	controlPlaneCtx := core.WithAuthContext(ctx, core.AuthContext{ServiceAccountID: serviceAccount.ID, JWT: core.JWTClaims{Role: "service_role"}})
	controlPlaneRequest := request.WithContext(controlPlaneCtx)
	if _, allowed := service.BaseHandler().authorizeRealtimeEvent(controlPlaneRequest, deletedCDCEvent); !allowed {
		t.Fatal("scoped administrator could not receive delete")
	}
	if _, err := kernel.ServiceAccountManager().Update(ctx, serviceAccount.ID, core.UpdateServiceAccountInput{Scopes: []string{core.ScopeAuthUserRead}, AllowedIPs: []string{"192.0.2.1"}}); err != nil {
		t.Fatal(err)
	}
	if _, allowed := service.BaseHandler().authorizeRealtimeEvent(controlPlaneRequest, cdcEvent); allowed {
		t.Fatal("new IP restriction was ignored")
	}
	if _, err := kernel.ServiceAccountManager().Update(ctx, serviceAccount.ID, core.UpdateServiceAccountInput{AllowedIPs: []string{"127.0.0.1"}}); err != nil {
		t.Fatal(err)
	}
	if _, allowed := service.BaseHandler().authorizeRealtimeEvent(controlPlaneRequest, hiddenCDCEvent); allowed {
		t.Fatal("unscoped Service Account bypassed RLS")
	}
	disabled := false
	if _, err := kernel.ServiceAccountManager().Update(ctx, serviceAccount.ID, core.UpdateServiceAccountInput{IsEnabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, allowed := service.BaseHandler().authorizeRealtimeEvent(controlPlaneRequest, cdcEvent); allowed {
		t.Fatal("disabled account retained realtime access")
	}
	if _, err := kernel.DB().Exec(ctx, `ALTER TABLE public.review_events DISABLE ROW LEVEL SECURITY`); err != nil {
		t.Fatal(err)
	}
	if !authorize(deletedCDCEvent) {
		t.Fatal("public deleted row was incorrectly hidden")
	}
	if _, err := kernel.DB().Exec(ctx, `REVOKE SELECT ON public.review_events FROM layr_data_client`); err != nil {
		t.Fatal(err)
	}
	if authorize(cdcEvent) || authorize(deletedCDCEvent) {
		t.Fatal("table privileges were bypassed")
	}
	if err := service.grantQuerySchemas(ctx, []string{"public", "core", "missing_schema"}); err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.DB().Exec(ctx, `DROP OWNED BY layr_data_client; DROP ROLE layr_data_client`); err != nil {
		t.Fatal(err)
	}
	if authorize(cdcEvent) {
		t.Fatal("missing query role did not fail closed")
	}
	if err := service.grantQuerySchemas(ctx, []string{"public"}); err == nil {
		t.Fatal("missing query role did not fail privilege setup")
	}
	service.Stop()
	kernel.DB().Close()
	if authorize(cdcEvent) {
		t.Fatal("database failure did not fail closed")
	}
	if err := service.grantQuerySchemas(ctx, []string{"public"}); err == nil {
		t.Fatal("closed database did not fail schema grants")
	}
}

func TestDataRealtimeRevokedSessionsIntegration(t *testing.T) {
	migrations := append(append([]core.DatabaseMigration{}, Migrations...), auth.Migrations...)
	kernel, cleanup := core.SetupTestKernel(t, migrations)
	defer cleanup()
	ctx := context.Background()
	service := NewService(kernel)
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer service.Stop()
	var userID, sessionID string
	if err := kernel.DB().QueryRow(ctx, `INSERT INTO auth.users DEFAULT VALUES RETURNING id::text`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := kernel.DB().QueryRow(ctx, `INSERT INTO auth.sessions(user_id,refresh_token_hash,expires_at) VALUES($1,'realtime-test',clock_timestamp()+interval '1 hour') RETURNING id::text`, userID).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.DB().Exec(ctx, `CREATE TABLE public.review_session_rows(id integer PRIMARY KEY); INSERT INTO public.review_session_rows VALUES(1)`); err != nil {
		t.Fatal(err)
	}
	cdcEvent := realtime.CDCEvent{Schema: "public", Table: "review_session_rows", Event: "INSERT", Record: map[string]any{"id": 1}}
	userCtx := core.WithAuthContext(ctx, core.AuthContext{UserID: userID, JWT: core.JWTClaims{Subject: userID, SessionID: sessionID, Role: "authenticated"}})
	userRequest := httptest.NewRequestWithContext(userCtx, http.MethodGet, "/v1/realtime", nil)
	if _, allowed := service.BaseHandler().authorizeRealtimeEvent(userRequest, cdcEvent); !allowed {
		t.Fatal("live application session rejected")
	}
	if _, err := kernel.DB().Exec(ctx, `UPDATE auth.users SET locked_until=clock_timestamp()+interval '1 hour' WHERE id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	if _, allowed := service.BaseHandler().authorizeRealtimeEvent(userRequest, cdcEvent); allowed {
		t.Fatal("locked user received an event")
	}
	if _, err := kernel.DB().Exec(ctx, `UPDATE auth.users SET locked_until=NULL WHERE id=$1;`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.DB().Exec(ctx, `DELETE FROM auth.sessions WHERE id=$1`, sessionID); err != nil {
		t.Fatal(err)
	}
	if _, allowed := service.BaseHandler().authorizeRealtimeEvent(userRequest, cdcEvent); allowed {
		t.Fatal("revoked application session received an event")
	}
	var consoleUserID, consoleSessionID string
	if err := kernel.DB().QueryRow(ctx, `INSERT INTO console.users(email,password_hash) VALUES('realtime@example.test','test') RETURNING id::text`).Scan(&consoleUserID); err != nil {
		t.Fatal(err)
	}
	if err := kernel.DB().QueryRow(ctx, `INSERT INTO console.sessions(user_id,session_token_hash,ip_address,expires_at) VALUES($1,'realtime-console','127.0.0.1',clock_timestamp()+interval '1 hour') RETURNING id::text`, consoleUserID).Scan(&consoleSessionID); err != nil {
		t.Fatal(err)
	}
	serviceAccount, err := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{Name: "session-reader", Scopes: []string{core.ScopeDataQueryRead}})
	if err != nil {
		t.Fatal(err)
	}
	consoleCtx := core.WithAuthContext(ctx, core.AuthContext{ServiceAccountID: serviceAccount.ID, JWT: core.JWTClaims{Subject: consoleUserID, SessionID: consoleSessionID, Audience: "layr:console:user", Role: "service_role"}})
	consoleRequest := httptest.NewRequestWithContext(consoleCtx, http.MethodGet, "/v1/realtime", nil)
	if _, allowed := service.BaseHandler().authorizeRealtimeEvent(consoleRequest, cdcEvent); !allowed {
		t.Fatal("live console session rejected")
	}
	if _, err := kernel.DB().Exec(ctx, `DELETE FROM console.sessions WHERE id=$1`, consoleSessionID); err != nil {
		t.Fatal(err)
	}
	if _, allowed := service.BaseHandler().authorizeRealtimeEvent(consoleRequest, cdcEvent); allowed {
		t.Fatal("revoked console session received an event")
	}
}
