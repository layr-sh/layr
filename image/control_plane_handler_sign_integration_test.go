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

func TestImageControlPlaneHandlerSignIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "image-signer",
		Scopes: []string{
			core.ScopeImageSignWrite,
		},
	})
	require.NoError(t, accountErr)
	authKey := serviceAccount.SecretKey

	t.Run("sign url integration", func(t *testing.T) {
		signPayload := `{"path":"/rs:fit:300:300/plain/my-bucket/pic.png@webp"}`
		signRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/image/sign", bytes.NewReader([]byte(signPayload)))
		signRequest.Header.Set("Content-Type", "application/json")
		signRequest.Header.Set("X-Service-Account-Key", authKey)
		signResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(signResponseRecorder, signRequest)
		require.Equal(t, http.StatusOK, signResponseRecorder.Code)

		var signURLResponse SignURLResponse
		require.NoError(t, json.Unmarshal(signResponseRecorder.Body.Bytes(), &signURLResponse))
		require.NotEmpty(t, signURLResponse.Signature)

		expectedSignature := SignPath(service.ConfigManager().SigningKey(), service.ConfigManager().SigningSalt(), "/rs:fit:300:300/plain/my-bucket/pic.png@webp")
		require.Equal(t, expectedSignature, signURLResponse.Signature)
	})
}
