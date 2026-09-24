package image

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
	"layr.sh/filestorage"
)

func TestImageBaseHandlerInfoUnit(t *testing.T) {
	allMigrations := append(filestorage.Migrations, Migrations...)
	kernel, cleanup := core.SetupTestKernel(t, allMigrations)
	defer cleanup()

	service := NewService(kernel)
	baseHandler := service.BaseHandler()
	ctx := context.Background()

	pngBytes := createTestImagePNG(120, 80)

	// Seed public bucket and image object
	publicBucketID := uuid.New()
	const insertPublicBucketSQL = `
		INSERT INTO file_storage.buckets (id, name, is_public, backend, backend_config, created_at, last_updated_at)
		VALUES ($1, 'info-public', true, 'database', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err := kernel.DB().Exec(ctx, insertPublicBucketSQL, publicBucketID)
	require.NoError(t, err)

	publicObjectID := uuid.New()
	const insertPublicObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'photo.png', 'image/png', $3, 'chk-photo', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err = kernel.DB().Exec(ctx, insertPublicObjectSQL, publicObjectID, publicBucketID, len(pngBytes))
	require.NoError(t, err)

	const insertPublicChunkSQL = `
		INSERT INTO file_storage.chunks (object_id, chunk_index, chunk_data)
		VALUES ($1, 0, $2);
	`
	_, err = kernel.DB().Exec(ctx, insertPublicChunkSQL, publicObjectID, pngBytes)
	require.NoError(t, err)

	// Seed corrupt image object
	corruptBytes := []byte("not-an-image")
	corruptObjectID := uuid.New()
	const insertCorruptObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'corrupt.png', 'image/png', $3, 'chk-corrupt', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err = kernel.DB().Exec(ctx, insertCorruptObjectSQL, corruptObjectID, publicBucketID, len(corruptBytes))
	require.NoError(t, err)
	_, err = kernel.DB().Exec(ctx, insertPublicChunkSQL, corruptObjectID, corruptBytes)
	require.NoError(t, err)

	// Seed private bucket
	privateBucketID := uuid.New()
	const insertPrivateBucketSQL = `
		INSERT INTO file_storage.buckets (id, name, is_public, backend, backend_config, created_at, last_updated_at)
		VALUES ($1, 'info-private', false, 'database', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err = kernel.DB().Exec(ctx, insertPrivateBucketSQL, privateBucketID)
	require.NoError(t, err)

	privateObjectID := uuid.New()
	const insertPrivateObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'secret.png', 'image/png', $3, 'chk-secret', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err = kernel.DB().Exec(ctx, insertPrivateObjectSQL, privateObjectID, privateBucketID, len(pngBytes))
	require.NoError(t, err)
	_, err = kernel.DB().Exec(ctx, insertPublicChunkSQL, privateObjectID, pngBytes)
	require.NoError(t, err)

	t.Run("info probe inspects binary payload", func(t *testing.T) {
		probeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/image/info", bytes.NewReader(pngBytes))
		probeResponseRecorder := httptest.NewRecorder()
		baseHandler.handleProbeInfo(probeResponseRecorder, probeRequest)
		require.Equal(t, http.StatusOK, probeResponseRecorder.Code)

		var getInfoResponse GetInfoResponse
		require.NoError(t, json.Unmarshal(probeResponseRecorder.Body.Bytes(), &getInfoResponse))
		require.Equal(t, 120, getInfoResponse.Width)
		require.Equal(t, 80, getInfoResponse.Height)
		require.Equal(t, FormatPNG, getInfoResponse.Format)
	})

	t.Run("info probe multipart form upload", func(t *testing.T) {
		bodyBuffer := new(bytes.Buffer)
		boundary := "----WebKitFormBoundarySample"
		bodyBuffer.WriteString("--")
		bodyBuffer.WriteString(boundary)
		bodyBuffer.WriteString("\r\n")
		bodyBuffer.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"photo.png\"\r\n")
		bodyBuffer.WriteString("Content-Type: image/png\r\n\r\n")
		bodyBuffer.Write(pngBytes)
		bodyBuffer.WriteString("\r\n--")
		bodyBuffer.WriteString(boundary)
		bodyBuffer.WriteString("--\r\n")

		infoRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/image/info", bodyBuffer)
		infoRequest.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
		infoResponseRecorder := httptest.NewRecorder()
		baseHandler.handleProbeInfo(infoResponseRecorder, infoRequest)
		require.Equal(t, http.StatusOK, infoResponseRecorder.Code)

		var getInfoResponse GetInfoResponse
		require.NoError(t, json.Unmarshal(infoResponseRecorder.Body.Bytes(), &getInfoResponse))
		require.Equal(t, 120, getInfoResponse.Width)
		require.Equal(t, 80, getInfoResponse.Height)
	})

	t.Run("info probe error cases", func(t *testing.T) {
		// Empty body -> 422 Unprocessable Entity
		emptyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/image/info", bytes.NewReader(nil))
		emptyResponseRecorder := httptest.NewRecorder()
		baseHandler.handleProbeInfo(emptyResponseRecorder, emptyRequest)
		require.Equal(t, http.StatusUnprocessableEntity, emptyResponseRecorder.Code)

		// Corrupt bytes -> 422 Unprocessable Entity
		corruptRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/image/info", bytes.NewReader([]byte("not-an-image")))
		corruptResponseRecorder := httptest.NewRecorder()
		baseHandler.handleProbeInfo(corruptResponseRecorder, corruptRequest)
		require.Equal(t, http.StatusUnprocessableEntity, corruptResponseRecorder.Code)
	})

	t.Run("info get validation and signature errors", func(t *testing.T) {
		signingKey := service.ConfigManager().SigningKey()
		signingSalt := service.ConfigManager().SigningSalt()

		// Missing signature -> 400
		missingSignatureRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/info//", nil)
		missingSignatureResponseRecorder := httptest.NewRecorder()
		baseHandler.handleGetInfo(missingSignatureResponseRecorder, missingSignatureRequest)
		require.Equal(t, http.StatusBadRequest, missingSignatureResponseRecorder.Code)

		// Invalid signature -> 403
		badSignatureRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/info/bad/plain/b/k", nil)
		badSignatureRequest.SetPathValue("signature", "bad")
		badSignatureRequest.SetPathValue("path", "/plain/b/k")
		badSignatureResponseRecorder := httptest.NewRecorder()
		baseHandler.handleGetInfo(badSignatureResponseRecorder, badSignatureRequest)
		require.Equal(t, http.StatusForbidden, badSignatureResponseRecorder.Code)

		// Bad path -> 400
		badPath := "/plain/"
		badSig := SignPath(signingKey, signingSalt, badPath)
		badPathRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/info/"+badSig+badPath, nil)
		badPathRequest.SetPathValue("signature", badSig)
		badPathRequest.SetPathValue("path", badPath)
		badPathResponseRecorder := httptest.NewRecorder()
		baseHandler.handleGetInfo(badPathResponseRecorder, badPathRequest)
		require.Equal(t, http.StatusBadRequest, badPathResponseRecorder.Code)

		// With query parameter and nonexistent storage target -> 404
		queryTarget := "/rs:fill:100:100/plain/info-public/nonexistent.png"
		queryPath := queryTarget + "?v=1"
		querySig := SignPath(signingKey, signingSalt, queryPath)
		queryRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/info/"+querySig+queryTarget+"?v=1", nil)
		queryRequest.SetPathValue("signature", querySig)
		queryRequest.SetPathValue("path", queryTarget)
		queryResponseRecorder := httptest.NewRecorder()
		baseHandler.handleGetInfo(queryResponseRecorder, queryRequest)
		require.Equal(t, http.StatusNotFound, queryResponseRecorder.Code)
	})

	t.Run("info get storage variants", func(t *testing.T) {
		signingKey := service.ConfigManager().SigningKey()
		signingSalt := service.ConfigManager().SigningSalt()

		// 1. Success 200 OK
		successPath := "/plain/info-public/photo.png"
		successSig := SignPath(signingKey, signingSalt, successPath)
		successRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/info/"+successSig+successPath, nil)
		successRequest.SetPathValue("signature", successSig)
		successRequest.SetPathValue("path", successPath)
		successResponseRecorder := httptest.NewRecorder()
		baseHandler.handleGetInfo(successResponseRecorder, successRequest)
		require.Equal(t, http.StatusOK, successResponseRecorder.Code)

		var getInfoResponse GetInfoResponse
		require.NoError(t, json.Unmarshal(successResponseRecorder.Body.Bytes(), &getInfoResponse))
		require.Equal(t, 120, getInfoResponse.Width)
		require.Equal(t, 80, getInfoResponse.Height)

		// 2. Storage Access Denied 403
		deniedPath := "/plain/info-private/secret.png"
		deniedSig := SignPath(signingKey, signingSalt, deniedPath)
		deniedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/info/"+deniedSig+deniedPath, nil)
		deniedRequest.SetPathValue("signature", deniedSig)
		deniedRequest.SetPathValue("path", deniedPath)
		deniedResponseRecorder := httptest.NewRecorder()
		baseHandler.handleGetInfo(deniedResponseRecorder, deniedRequest)
		require.Equal(t, http.StatusForbidden, deniedResponseRecorder.Code)

		// 3. Bad Gateway 502 (unsupported scheme)
		badGatewayPath := "/plain/ftp://example.com/bad.png"
		badGatewaySig := SignPath(signingKey, signingSalt, badGatewayPath)
		badGatewayRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/info/"+badGatewaySig+badGatewayPath, nil)
		badGatewayRequest.SetPathValue("signature", badGatewaySig)
		badGatewayRequest.SetPathValue("path", badGatewayPath)
		badGatewayResponseRecorder := httptest.NewRecorder()
		baseHandler.handleGetInfo(badGatewayResponseRecorder, badGatewayRequest)
		require.Equal(t, http.StatusBadGateway, badGatewayResponseRecorder.Code)

		// 4. Corrupt asset inspection 422
		corruptPath := "/plain/info-public/corrupt.png"
		corruptSig := SignPath(signingKey, signingSalt, corruptPath)
		corruptRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/info/"+corruptSig+corruptPath, nil)
		corruptRequest.SetPathValue("signature", corruptSig)
		corruptRequest.SetPathValue("path", corruptPath)
		corruptResponseRecorder := httptest.NewRecorder()
		baseHandler.handleGetInfo(corruptResponseRecorder, corruptRequest)
		require.Equal(t, http.StatusUnprocessableEntity, corruptResponseRecorder.Code)
	})

	t.Run("multipart form upload edge cases", func(t *testing.T) {
		// 1. Corrupt multipart boundary -> 400
		corruptMultipartRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/image/info", bytes.NewReader([]byte("not-a-valid-multipart")))
		corruptMultipartRequest.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
		corruptResponseRecorder := httptest.NewRecorder()
		baseHandler.handleProbeInfo(corruptResponseRecorder, corruptMultipartRequest)
		require.Equal(t, http.StatusBadRequest, corruptResponseRecorder.Code)

		// 2. Field named "image" fallback
		imageFieldBuffer := new(bytes.Buffer)
		boundary := "----BoundaryImageField"
		imageFieldBuffer.WriteString("--")
		imageFieldBuffer.WriteString(boundary)
		imageFieldBuffer.WriteString("\r\n")
		imageFieldBuffer.WriteString("Content-Disposition: form-data; name=\"image\"; filename=\"photo.png\"\r\n")
		imageFieldBuffer.WriteString("Content-Type: image/png\r\n\r\n")
		imageFieldBuffer.Write(pngBytes)
		imageFieldBuffer.WriteString("\r\n--")
		imageFieldBuffer.WriteString(boundary)
		imageFieldBuffer.WriteString("--\r\n")

		imageFieldRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/image/info", imageFieldBuffer)
		imageFieldRequest.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
		imageFieldResponseRecorder := httptest.NewRecorder()
		baseHandler.handleProbeInfo(imageFieldResponseRecorder, imageFieldRequest)
		require.Equal(t, http.StatusOK, imageFieldResponseRecorder.Code)

		// 3. No file field in form -> 400
		noFileBuffer := new(bytes.Buffer)
		noFileBoundary := "----BoundaryNoFile"
		noFileBuffer.WriteString("--")
		noFileBuffer.WriteString(noFileBoundary)
		noFileBuffer.WriteString("\r\n")
		noFileBuffer.WriteString("Content-Disposition: form-data; name=\"title\"\r\n\r\n")
		noFileBuffer.WriteString("some-title\r\n")
		noFileBuffer.WriteString("--")
		noFileBuffer.WriteString(noFileBoundary)
		noFileBuffer.WriteString("--\r\n")

		noFileRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/image/info", noFileBuffer)
		noFileRequest.Header.Set("Content-Type", "multipart/form-data; boundary="+noFileBoundary)
		noFileResponseRecorder := httptest.NewRecorder()
		baseHandler.handleProbeInfo(noFileResponseRecorder, noFileRequest)
		require.Equal(t, http.StatusBadRequest, noFileResponseRecorder.Code)

		// 4. File field containing non-image bytes -> 422
		corruptFileBuffer := new(bytes.Buffer)
		corruptBoundary := "----BoundaryCorruptFile"
		corruptFileBuffer.WriteString("--")
		corruptFileBuffer.WriteString(corruptBoundary)
		corruptFileBuffer.WriteString("\r\n")
		corruptFileBuffer.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"corrupt.png\"\r\n")
		corruptFileBuffer.WriteString("Content-Type: image/png\r\n\r\n")
		corruptFileBuffer.WriteString("not-valid-png-binary-data")
		corruptFileBuffer.WriteString("\r\n--")
		corruptFileBuffer.WriteString(corruptBoundary)
		corruptFileBuffer.WriteString("--\r\n")

		corruptFileRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/image/info", corruptFileBuffer)
		corruptFileRequest.Header.Set("Content-Type", "multipart/form-data; boundary="+corruptBoundary)
		corruptFileResponseRecorder := httptest.NewRecorder()
		baseHandler.handleProbeInfo(corruptFileResponseRecorder, corruptFileRequest)
		require.Equal(t, http.StatusUnprocessableEntity, corruptFileResponseRecorder.Code)
	})
}
