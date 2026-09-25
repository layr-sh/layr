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
	"uuid"
)

func TestImageControlPlaneHandlerPresetUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()

	readPresetAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeImagePresetRead,
		},
	}
	readPresetCtx := core.WithAuthContext(context.Background(), readPresetAuthContext)

	writePresetAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeImagePresetWrite,
		},
	}
	writePresetCtx := core.WithAuthContext(context.Background(), writePresetAuthContext)

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

	t.Run("list presets unit", func(t *testing.T) {
		// Insufficient scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/image/presets", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListPresets(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)

		// Valid scope -> 200
		authedRequest := httptest.NewRequestWithContext(readPresetCtx, http.MethodGet, "/v1/_/image/presets", nil)
		authedResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListPresets(authedResponseRecorder, authedRequest)
		require.Equal(t, http.StatusOK, authedResponseRecorder.Code)
	})

	t.Run("crud preset lifecycle unit", func(t *testing.T) {
		// 1. Create preset - missing scope -> 403
		createPayload := `{"name":"thumbnail","processing_options":"rs:fill:150:150/q:80"}`
		noScopeCreateRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/image/presets", bytes.NewReader([]byte(createPayload)))
		noScopeCreateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreatePreset(noScopeCreateResponseRecorder, noScopeCreateRequest)
		require.Equal(t, http.StatusForbidden, noScopeCreateResponseRecorder.Code)

		// 2. Create preset - bad json -> 400
		badJSONRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodPost, "/v1/_/image/presets", bytes.NewReader([]byte("{bad-json")))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreatePreset(badJSONResponseRecorder, badJSONRequest)
		require.Equal(t, http.StatusBadRequest, badJSONResponseRecorder.Code)

		// 3. Create preset - validation error (empty name) -> 400
		invalidCreateRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodPost, "/v1/_/image/presets", bytes.NewReader([]byte(`{"name":"","processing_options":"rs:fill:10:10"}`)))
		invalidCreateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreatePreset(invalidCreateResponseRecorder, invalidCreateRequest)
		require.Equal(t, http.StatusBadRequest, invalidCreateResponseRecorder.Code)

		// 4. Create preset - valid -> 201
		validCreateRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodPost, "/v1/_/image/presets", bytes.NewReader([]byte(createPayload)))
		validCreateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreatePreset(validCreateResponseRecorder, validCreateRequest)
		require.Equal(t, http.StatusCreated, validCreateResponseRecorder.Code)

		var createdPreset Preset
		require.NoError(t, json.Unmarshal(validCreateResponseRecorder.Body.Bytes(), &createdPreset))
		require.Equal(t, "thumbnail", createdPreset.Name)

		// 5. Create preset - duplicate name -> 409
		duplicateRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodPost, "/v1/_/image/presets", bytes.NewReader([]byte(createPayload)))
		duplicateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreatePreset(duplicateResponseRecorder, duplicateRequest)
		require.Equal(t, http.StatusConflict, duplicateResponseRecorder.Code)

		// 6. Get preset missing scope -> 403
		presetIDString := createdPreset.ID.String()
		noScopeGetRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/image/presets/"+presetIDString, nil)
		noScopeGetRequest.SetPathValue("preset_id", presetIDString)
		noScopeGetResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetPreset(noScopeGetResponseRecorder, noScopeGetRequest)
		require.Equal(t, http.StatusForbidden, noScopeGetResponseRecorder.Code)

		// 7. Get preset by ID -> 200
		getRequest := httptest.NewRequestWithContext(readPresetCtx, http.MethodGet, "/v1/_/image/presets/"+presetIDString, nil)
		getRequest.SetPathValue("preset_id", presetIDString)
		getResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetPreset(getResponseRecorder, getRequest)
		require.Equal(t, http.StatusOK, getResponseRecorder.Code)

		// 8. Get preset invalid UUID -> 400
		badUUIDRequest := httptest.NewRequestWithContext(readPresetCtx, http.MethodGet, "/v1/_/image/presets/invalid-uuid", nil)
		badUUIDRequest.SetPathValue("preset_id", "invalid-uuid")
		badUUIDResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetPreset(badUUIDResponseRecorder, badUUIDRequest)
		require.Equal(t, http.StatusBadRequest, badUUIDResponseRecorder.Code)

		// 9. Get nonexistent preset -> 404
		randomID := uuid.New()
		notFoundRequest := httptest.NewRequestWithContext(readPresetCtx, http.MethodGet, "/v1/_/image/presets/"+randomID.String(), nil)
		notFoundRequest.SetPathValue("preset_id", randomID.String())
		notFoundResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetPreset(notFoundResponseRecorder, notFoundRequest)
		require.Equal(t, http.StatusNotFound, notFoundResponseRecorder.Code)

		// 10. Update preset missing scope -> 403
		updatePayload := `{"processing_options":"rs:fill:200:200/q:85"}`
		noScopeUpdateRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPut, "/v1/_/image/presets/"+presetIDString, bytes.NewReader([]byte(updatePayload)))
		noScopeUpdateRequest.SetPathValue("preset_id", presetIDString)
		noScopeUpdateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdatePreset(noScopeUpdateResponseRecorder, noScopeUpdateRequest)
		require.Equal(t, http.StatusForbidden, noScopeUpdateResponseRecorder.Code)

		// 11. Update preset -> 200
		updateRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodPut, "/v1/_/image/presets/"+presetIDString, bytes.NewReader([]byte(updatePayload)))
		updateRequest.SetPathValue("preset_id", presetIDString)
		updateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdatePreset(updateResponseRecorder, updateRequest)
		require.Equal(t, http.StatusOK, updateResponseRecorder.Code)

		// 10. Update preset bad UUID -> 400
		badUUIDUpdateRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodPut, "/v1/_/image/presets/invalid-uuid", bytes.NewReader([]byte(updatePayload)))
		badUUIDUpdateRequest.SetPathValue("preset_id", "invalid-uuid")
		badUUIDUpdateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdatePreset(badUUIDUpdateResponseRecorder, badUUIDUpdateRequest)
		require.Equal(t, http.StatusBadRequest, badUUIDUpdateResponseRecorder.Code)

		// 11. Update preset bad JSON -> 400
		badJSONUpdateRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodPut, "/v1/_/image/presets/"+presetIDString, bytes.NewReader([]byte("{bad-json")))
		badJSONUpdateRequest.SetPathValue("preset_id", presetIDString)
		badJSONUpdateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdatePreset(badJSONUpdateResponseRecorder, badJSONUpdateRequest)
		require.Equal(t, http.StatusBadRequest, badJSONUpdateResponseRecorder.Code)

		// 12. Update nonexistent preset -> 404
		nonexistentUpdateRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodPut, "/v1/_/image/presets/"+randomID.String(), bytes.NewReader([]byte(updatePayload)))
		nonexistentUpdateRequest.SetPathValue("preset_id", randomID.String())
		nonexistentUpdateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdatePreset(nonexistentUpdateResponseRecorder, nonexistentUpdateRequest)
		require.Equal(t, http.StatusNotFound, nonexistentUpdateResponseRecorder.Code)

		// 13. Delete preset missing scope -> 403
		noScopeDeleteRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodDelete, "/v1/_/image/presets/"+presetIDString, nil)
		noScopeDeleteRequest.SetPathValue("preset_id", presetIDString)
		noScopeDeleteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeletePreset(noScopeDeleteResponseRecorder, noScopeDeleteRequest)
		require.Equal(t, http.StatusForbidden, noScopeDeleteResponseRecorder.Code)

		// 14. Delete preset bad UUID -> 400
		badUUIDDeleteRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodDelete, "/v1/_/image/presets/invalid-uuid", nil)
		badUUIDDeleteRequest.SetPathValue("preset_id", "invalid-uuid")
		badUUIDDeleteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeletePreset(badUUIDDeleteResponseRecorder, badUUIDDeleteRequest)
		require.Equal(t, http.StatusBadRequest, badUUIDDeleteResponseRecorder.Code)

		// 15. Delete preset -> 204
		deleteRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodDelete, "/v1/_/image/presets/"+presetIDString, nil)
		deleteRequest.SetPathValue("preset_id", presetIDString)
		deleteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeletePreset(deleteResponseRecorder, deleteRequest)
		require.Equal(t, http.StatusNoContent, deleteResponseRecorder.Code)

		// 16. Delete again -> 404
		deleteAgainRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodDelete, "/v1/_/image/presets/"+presetIDString, nil)
		deleteAgainRequest.SetPathValue("preset_id", presetIDString)
		deleteAgainResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeletePreset(deleteAgainResponseRecorder, deleteAgainRequest)
		require.Equal(t, http.StatusNotFound, deleteAgainResponseRecorder.Code)
	})
}

func TestImageControlPlaneHandlerPresetDatabaseFailureUnit(t *testing.T) {
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenService := NewService(brokenKernel)
	brokenControlPlaneHandler := brokenService.ControlPlaneHandler()
	targetUUID := uuid.New()
	targetUUIDString := targetUUID.String()

	readPresetAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeImagePresetRead,
		},
	}
	readPresetCtx := core.WithAuthContext(context.Background(), readPresetAuthContext)

	writePresetAuthContext := core.AuthContext{
		ServiceAccountID: "sa-test",
		JWT: core.JWTClaims{
			Subject:  "sa-test",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeImagePresetWrite,
		},
	}
	writePresetCtx := core.WithAuthContext(context.Background(), writePresetAuthContext)

	// 1. List presets DB error -> 500
	listRequest := httptest.NewRequestWithContext(readPresetCtx, http.MethodGet, "/v1/_/image/presets", nil)
	listResponseRecorder := httptest.NewRecorder()
	brokenControlPlaneHandler.handleListPresets(listResponseRecorder, listRequest)
	require.Equal(t, http.StatusInternalServerError, listResponseRecorder.Code)

	// 2. Get preset DB error -> 500
	getRequest := httptest.NewRequestWithContext(readPresetCtx, http.MethodGet, "/v1/_/image/presets/"+targetUUIDString, nil)
	getRequest.SetPathValue("preset_id", targetUUIDString)
	getResponseRecorder := httptest.NewRecorder()
	brokenControlPlaneHandler.handleGetPreset(getResponseRecorder, getRequest)
	require.Equal(t, http.StatusInternalServerError, getResponseRecorder.Code)

	// 3. Update preset DB error -> 500
	updatePayload := `{"processing_options":"rs:fill:100:100"}`
	updateRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodPut, "/v1/_/image/presets/"+targetUUIDString, bytes.NewReader([]byte(updatePayload)))
	updateRequest.SetPathValue("preset_id", targetUUIDString)
	updateResponseRecorder := httptest.NewRecorder()
	brokenControlPlaneHandler.handleUpdatePreset(updateResponseRecorder, updateRequest)
	require.Equal(t, http.StatusInternalServerError, updateResponseRecorder.Code)

	// 4. Delete preset DB error -> 500
	deleteRequest := httptest.NewRequestWithContext(writePresetCtx, http.MethodDelete, "/v1/_/image/presets/"+targetUUIDString, nil)
	deleteRequest.SetPathValue("preset_id", targetUUIDString)
	deleteResponseRecorder := httptest.NewRecorder()
	brokenControlPlaneHandler.handleDeletePreset(deleteResponseRecorder, deleteRequest)
	require.Equal(t, http.StatusInternalServerError, deleteResponseRecorder.Code)

	// 5. Direct PresetManager Update and Delete error branches
	newProcessingOptions := "rs:fill:50:50"
	_, managerUpdateErr := brokenService.PresetManager().Update(writePresetCtx, targetUUID, UpdatePresetInput{
		ProcessingOptions: &newProcessingOptions,
	})
	require.Error(t, managerUpdateErr)

	_, managerDeleteErr := brokenService.PresetManager().Delete(writePresetCtx, targetUUID)
	require.Error(t, managerDeleteErr)
}
