package image

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageControlPlaneHandlerSignUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()

	signAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeImageSignWrite,
		},
	}
	signCtx := core.WithAuthContext(context.Background(), signAuthContext)

	noScopeAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "",
		},
	}
	noScopeCtx := core.WithAuthContext(context.Background(), noScopeAuthContext)

	t.Run("missing scope returns 403", func(t *testing.T) {
		payload := `{"path":"/rs:fill:300:200/plain/local/bucket/pic.jpg@webp"}`
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/image/sign", bytes.NewReader([]byte(payload)))
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignURL(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)
	})

	t.Run("bad json returns 400", func(t *testing.T) {
		badJSONRequest := httptest.NewRequestWithContext(signCtx, http.MethodPost, "/v1/_/image/sign", bytes.NewReader([]byte("{bad-json")))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignURL(badJSONResponseRecorder, badJSONRequest)
		require.Equal(t, http.StatusBadRequest, badJSONResponseRecorder.Code)
	})

	t.Run("empty path returns 400", func(t *testing.T) {
		emptyPathRequest := httptest.NewRequestWithContext(signCtx, http.MethodPost, "/v1/_/image/sign", bytes.NewReader([]byte(`{"path":""}`)))
		emptyPathResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignURL(emptyPathResponseRecorder, emptyPathRequest)
		require.Equal(t, http.StatusBadRequest, emptyPathResponseRecorder.Code)
	})

	t.Run("valid path returns 200 and verifiable signature", func(t *testing.T) {
		targetPath := "/rs:fill:300:200/plain/local/bucket/pic.jpg@webp"
		payload := `{"path":"` + targetPath + `"}`
		authedRequest := httptest.NewRequestWithContext(signCtx, http.MethodPost, "/v1/_/image/sign", bytes.NewReader([]byte(payload)))
		authedResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignURL(authedResponseRecorder, authedRequest)
		require.Equal(t, http.StatusOK, authedResponseRecorder.Code)

		var signURLResponse SignURLResponse
		unmarshalErr := json.Unmarshal(authedResponseRecorder.Body.Bytes(), &signURLResponse)
		require.NoError(t, unmarshalErr)
		require.NotEmpty(t, signURLResponse.Signature)
		require.Contains(t, signURLResponse.URL, signURLResponse.Signature)

		signingKey := service.ConfigManager().SigningKey()
		signingSalt := service.ConfigManager().SigningSalt()
		require.True(t, VerifySignature(signingKey, signingSalt, signURLResponse.Signature, targetPath, false))

		// Path without leading slash
		noSlashPayload := `{"path":"rs:fill:100:100/plain/local/bucket/pic.jpg@webp"}`
		noSlashRequest := httptest.NewRequestWithContext(signCtx, http.MethodPost, "/v1/_/image/sign", bytes.NewReader([]byte(noSlashPayload)))
		noSlashResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleSignURL(noSlashResponseRecorder, noSlashRequest)
		require.Equal(t, http.StatusOK, noSlashResponseRecorder.Code)
	})
}
