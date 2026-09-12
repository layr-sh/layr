package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"layr.sh/auth/jwt"
	"layr.sh/core"
)

func TestAuthSessionSelfServiceIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanupDatabase := setupTestDatabase(t)
	defer cleanupDatabase()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial auth config: %v", err)
	}

	testKVStore := newInMemoryKVStore()
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()

	handler := NewHandler(db, configManager, cryptoKeyManager)
	handler.SetKVStore(testKVStore)
	handler.SetEventBus(eventBus)

	emittedEvents := make([]core.Event, 0)
	eventBus.Subscribe("auth.session.deleted", func(eventCtx context.Context, event core.Event) error {
		emittedEvents = append(emittedEvents, event)
		return nil
	})

	// Seed user
	userID := "01918a24-3333-7000-8000-000000000003"
	userEmail := "session.user@example.com"
	_, err := db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, $2, 'authenticated', false, clock_timestamp(), clock_timestamp())
	`, userID, userEmail)
	if err != nil {
		t.Fatalf("failed to insert user for session tests: %v", err)
	}

	// Seed 3 active sessions for this user
	session1ID := "01918a24-4444-7000-8000-000000000001"
	session1Refresh := "session1_refresh_token_12345678901234567890"
	session1Hash := jwt.HashRefreshToken(session1Refresh)

	session2ID := "01918a24-4444-7000-8000-000000000002"
	session2Refresh := "session2_refresh_token_12345678901234567890"
	session2Hash := jwt.HashRefreshToken(session2Refresh)

	session3ID := "01918a24-4444-7000-8000-000000000003"
	session3Refresh := "session3_refresh_token_12345678901234567890"
	session3Hash := jwt.HashRefreshToken(session3Refresh)

	device1UA := "Chrome macOS"
	device2UA := "Mobile Safari iOS"
	device3UA := "Firefox Linux"

	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.sessions (id, user_id, refresh_token_hash, user_agent, expires_at, created_at)
		VALUES 
			($1, $4, $5, $8, clock_timestamp() + interval '1 hour', clock_timestamp() - interval '20 minutes'),
			($2, $4, $6, $9, clock_timestamp() + interval '1 hour', clock_timestamp() - interval '10 minutes'),
			($3, $4, $7, $10, clock_timestamp() + interval '1 hour', clock_timestamp());
	`, session1ID, session2ID, session3ID, userID, session1Hash, session2Hash, session3Hash, device1UA, device2UA, device3UA)

	_ = testKVStore.Set(ctx, "auth:session:"+session1Hash, "active1", time.Hour)
	_ = testKVStore.Set(ctx, "auth:session:"+session2Hash, "active2", time.Hour)
	_ = testKVStore.Set(ctx, "auth:session:"+session3Hash, "active3", time.Hour)

	userToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     userID,
		Email:       userEmail,
		Role:        "authenticated",
		IsAnonymous: false,
	}, 3600)

	// 1. List Sessions with Cookie (Session 1 matches)
	cookieListRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user/sessions", nil)
	cookieListRequest.Header.Set("Authorization", "Bearer "+userToken)
	cookieListRequest.AddCookie(&http.Cookie{Name: AuthSessionCookieName, Value: session1Refresh})
	cookieListResponseRecorder := httptest.NewRecorder()
	handler.handleListSessions(cookieListResponseRecorder, cookieListRequest)
	if cookieListResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on list sessions, got: %d (%s)", cookieListResponseRecorder.Code, cookieListResponseRecorder.Body.String())
	}

	var cookieListUserSessionsResponse ListUserSessionsResponse
	_ = json.NewDecoder(cookieListResponseRecorder.Body).Decode(&cookieListUserSessionsResponse)
	if cookieListUserSessionsResponse.Count != 3 {
		t.Fatalf("expected 3 sessions, got: %d", cookieListUserSessionsResponse.Count)
	}
	var currentFound bool
	for _, s := range cookieListUserSessionsResponse.Sessions {
		if s.ID == session1ID && s.IsCurrent {
			currentFound = true
		}
	}
	if !currentFound {
		t.Fatalf("expected session1 to be flagged as current")
	}

	// 2. List Sessions with X-Refresh-Token (Session 2 matches)
	refreshTokenListRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user/sessions", nil)
	refreshTokenListRequest.Header.Set("Authorization", "Bearer "+userToken)
	refreshTokenListRequest.Header.Set("X-Refresh-Token", session2Refresh)
	refreshTokenListResponseRecorder := httptest.NewRecorder()
	handler.handleListSessions(refreshTokenListResponseRecorder, refreshTokenListRequest)
	var refreshTokenListUserSessionsResponse ListUserSessionsResponse
	_ = json.NewDecoder(refreshTokenListResponseRecorder.Body).Decode(&refreshTokenListUserSessionsResponse)
	currentFound = false
	for _, s := range refreshTokenListUserSessionsResponse.Sessions {
		if s.ID == session2ID && s.IsCurrent {
			currentFound = true
		}
	}
	if !currentFound {
		t.Fatalf("expected session2 to be flagged as current via header")
	}

	// 2b. List Sessions with X-Session-ID (Session 3 matches)
	sessionIDListRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user/sessions", nil)
	sessionIDListRequest.Header.Set("Authorization", "Bearer "+userToken)
	sessionIDListRequest.Header.Set("X-Session-ID", session3ID)
	sessionIDListResponseRecorder := httptest.NewRecorder()
	handler.handleListSessions(sessionIDListResponseRecorder, sessionIDListRequest)
	var sessionIDListUserSessionsResponse ListUserSessionsResponse
	_ = json.NewDecoder(sessionIDListResponseRecorder.Body).Decode(&sessionIDListUserSessionsResponse)
	currentFound = false
	for _, s := range sessionIDListUserSessionsResponse.Sessions {
		if s.ID == session3ID && s.IsCurrent {
			currentFound = true
		}
	}
	if !currentFound {
		t.Fatalf("expected session3 to be flagged as current via X-Session-ID header")
	}

	// 3. List Sessions for single session user (Single session fallback)
	singleUserID := "01918a24-5555-7000-8000-000000000005"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, 'single@example.com', 'authenticated', false, clock_timestamp(), clock_timestamp())
	`, singleUserID)
	singleSessionID := "01918a24-5555-7000-8000-000000000015"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.sessions (id, user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, 'singlehash', clock_timestamp() + interval '1 hour', clock_timestamp())
	`, singleSessionID, singleUserID)
	singleToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject: singleUserID,
		Role:    "authenticated",
	}, 3600)
	singleListRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user/sessions", nil)
	singleListRequest.Header.Set("Authorization", "Bearer "+singleToken)
	singleListResponseRecorder := httptest.NewRecorder()
	handler.handleListSessions(singleListResponseRecorder, singleListRequest)
	var singleListUserSessionsResponse ListUserSessionsResponse
	_ = json.NewDecoder(singleListResponseRecorder.Body).Decode(&singleListUserSessionsResponse)
	if singleListUserSessionsResponse.Count != 1 || !singleListUserSessionsResponse.Sessions[0].IsCurrent {
		t.Fatalf("expected single session fallback to mark IsCurrent = true")
	}

	// 4. Revoke Single Session -> 204
	revokeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user/sessions/"+session1ID, nil)
	revokeRequest.SetPathValue("session_id", session1ID)
	revokeRequest.Header.Set("Authorization", "Bearer "+userToken)
	revokeResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeSession(revokeResponseRecorder, revokeRequest)
	if revokeResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on revoke session, got: %d (%s)", revokeResponseRecorder.Code, revokeResponseRecorder.Body.String())
	}

	// Verify deleted from DB and KV store
	var session1DBCount int
	_ = db.QueryRow(ctx, "SELECT count(*) FROM layr_auth.sessions WHERE id = $1", session1ID).Scan(&session1DBCount)
	if session1DBCount != 0 {
		t.Fatalf("expected session1 to be deleted from database")
	}
	if _, getErr := testKVStore.Get(ctx, "auth:session:"+session1Hash); getErr == nil {
		t.Fatalf("expected session1 to be purged from KV store")
	}

	// 5. Revoke Non-Existent Session -> 404
	ghostSessionID := "01918a24-9999-7000-8000-000000000099"
	ghostRevokeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user/sessions/"+ghostSessionID, nil)
	ghostRevokeRequest.SetPathValue("session_id", ghostSessionID)
	ghostRevokeRequest.Header.Set("Authorization", "Bearer "+userToken)
	ghostRevokeResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeSession(ghostRevokeResponseRecorder, ghostRevokeRequest)
	if ghostRevokeResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on ghost session revoke, got: %d", ghostRevokeResponseRecorder.Code)
	}

	// 6. Revoke Other Sessions with Ambiguous Identifier -> 400
	ambiguousRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/sessions/revoke-others", nil)
	ambiguousRequest.Header.Set("Authorization", "Bearer "+userToken)
	ambiguousResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeOtherSessions(ambiguousResponseRecorder, ambiguousRequest)
	if ambiguousResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on ambiguous revoke-others, got: %d", ambiguousResponseRecorder.Code)
	}

	// 7. Revoke Other Sessions with Session ID Header -> 200
	session4ID := "01918a24-5555-7000-8000-000000000004"
	session4Refresh := "test_refresh_token_4"
	session4Hash := jwt.HashRefreshToken(session4Refresh)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.sessions (id, user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, clock_timestamp() + interval '1 hour', clock_timestamp())
	`, session4ID, userID, session4Hash)

	revokeOthersWithSessionIDRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/sessions/revoke-others", nil)
	revokeOthersWithSessionIDRequest.Header.Set("Authorization", "Bearer "+userToken)
	revokeOthersWithSessionIDRequest.Header.Set("X-Session-ID", session4ID)
	revokeOthersWithSessionIDResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeOtherSessions(revokeOthersWithSessionIDResponseRecorder, revokeOthersWithSessionIDRequest)
	if revokeOthersWithSessionIDResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on revoke others with session id, got: %d (%s)", revokeOthersWithSessionIDResponseRecorder.Code, revokeOthersWithSessionIDResponseRecorder.Body.String())
	}
	var withSessionIDRevokeOtherSessionsResponse RevokeOtherSessionsResponse
	_ = json.NewDecoder(revokeOthersWithSessionIDResponseRecorder.Body).Decode(&withSessionIDRevokeOtherSessionsResponse)
	if withSessionIDRevokeOtherSessionsResponse.RevokedCount != 2 {
		t.Fatalf("expected 2 sessions revoked with session id, got: %d", withSessionIDRevokeOtherSessionsResponse.RevokedCount)
	}

	// 7b. Revoke Other Sessions with Refresh Token Header
	// Insert session 5 so we have 2 sessions (session 4 and session 5)
	session5ID := "01918a24-5555-7000-8000-000000000005"
	session5Refresh := "test_refresh_token_5"
	session5Hash := jwt.HashRefreshToken(session5Refresh)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.sessions (id, user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, clock_timestamp() + interval '1 hour', clock_timestamp())
	`, session5ID, userID, session5Hash)

	revokeOthersRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/sessions/revoke-others", nil)
	revokeOthersRequest.Header.Set("Authorization", "Bearer "+userToken)
	revokeOthersRequest.Header.Set("X-Refresh-Token", session5Refresh)
	revokeOthersResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeOtherSessions(revokeOthersResponseRecorder, revokeOthersRequest)
	if revokeOthersResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on revoke others, got: %d (%s)", revokeOthersResponseRecorder.Code, revokeOthersResponseRecorder.Body.String())
	}
	var withRefreshTokenRevokeOtherSessionsResponse RevokeOtherSessionsResponse
	_ = json.NewDecoder(revokeOthersResponseRecorder.Body).Decode(&withRefreshTokenRevokeOtherSessionsResponse)
	if withRefreshTokenRevokeOtherSessionsResponse.RevokedCount != 1 {
		t.Fatalf("expected 1 session revoked, got: %d", withRefreshTokenRevokeOtherSessionsResponse.RevokedCount)
	}

	// 8. Revoke Other Sessions when only 1 active session remains -> 200 (Count = 0)
	singleActiveRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/sessions/revoke-others", nil)
	singleActiveRequest.Header.Set("Authorization", "Bearer "+userToken)
	singleActiveResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeOtherSessions(singleActiveResponseRecorder, singleActiveRequest)
	if singleActiveResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 when only 1 session active, got: %d", singleActiveResponseRecorder.Code)
	}
	var unaffectedRevokeOtherSessionsResponse RevokeOtherSessionsResponse
	_ = json.NewDecoder(singleActiveResponseRecorder.Body).Decode(&unaffectedRevokeOtherSessionsResponse)
	if unaffectedRevokeOtherSessionsResponse.RevokedCount != 0 {
		t.Fatalf("expected 0 revoked sessions, got: %d", unaffectedRevokeOtherSessionsResponse.RevokedCount)
	}

	// 9. Canceled Context on List, Revoke, and RevokeOthers -> 500
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		canceledListRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user/sessions", nil).WithContext(canceledCtx)
		canceledListRequest.Header.Set("Authorization", "Bearer "+userToken)
		canceledListResponseRecorder := httptest.NewRecorder()
		handler.handleListSessions(canceledListResponseRecorder, canceledListRequest)
		if canceledListResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on canceled list sessions, got: %d", canceledListResponseRecorder.Code)
		}

		canceledRevokeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user/sessions/"+session2ID, nil).WithContext(canceledCtx)
		canceledRevokeRequest.SetPathValue("session_id", session2ID)
		canceledRevokeRequest.Header.Set("Authorization", "Bearer "+userToken)
		canceledRevokeResponseRecorder := httptest.NewRecorder()
		handler.handleRevokeSession(canceledRevokeResponseRecorder, canceledRevokeRequest)
		if canceledRevokeResponseRecorder.Code != http.StatusNotFound {
			t.Fatalf("expected 404 on canceled revoke session, got: %d", canceledRevokeResponseRecorder.Code)
		}

		canceledRevokeOthersRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/sessions/revoke-others", nil).WithContext(canceledCtx)
		canceledRevokeOthersRequest.Header.Set("Authorization", "Bearer "+userToken)
		canceledRevokeOthersResponseRecorder := httptest.NewRecorder()
		handler.handleRevokeOtherSessions(canceledRevokeOthersResponseRecorder, canceledRevokeOthersRequest)
		if canceledRevokeOthersResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on canceled revoke others, got: %d", canceledRevokeOthersResponseRecorder.Code)
		}

		canceledRevokeOthersWithSessionRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/sessions/revoke-others", nil).WithContext(canceledCtx)
		canceledRevokeOthersWithSessionRequest.Header.Set("Authorization", "Bearer "+userToken)
		canceledRevokeOthersWithSessionRequest.Header.Set("X-Session-ID", session5ID)
		canceledRevokeOthersWithSessionResponseRecorder := httptest.NewRecorder()
		handler.handleRevokeOtherSessions(canceledRevokeOthersWithSessionResponseRecorder, canceledRevokeOthersWithSessionRequest)
		if canceledRevokeOthersWithSessionResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on canceled revoke others with session id, got: %d", canceledRevokeOthersWithSessionResponseRecorder.Code)
		}
	}
}
