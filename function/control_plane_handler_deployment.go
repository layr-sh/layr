// Package function defines the serverless and edge function execution engine.
package function

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// decodeCodeBundle converts either raw script strings or base64-encoded strings into raw bytes.
func decodeCodeBundle(rawBundle string) []byte {
	trimmedBundle := strings.TrimSpace(rawBundle)
	if decodedBytes, decodeErr := base64.StdEncoding.DecodeString(trimmedBundle); decodeErr == nil &&
		len(decodedBytes) > 0 &&
		!strings.Contains(trimmedBundle, " ") &&
		!strings.Contains(trimmedBundle, "\n") {
		return decodedBytes
	}
	return []byte(rawBundle)
}

// packageBundleFiles creates an in-memory tar archive of file maps and returns deterministic bytes and manifest.
func packageBundleFiles(files map[string]string) ([]byte, map[string]any, error) {
	var tarBuffer bytes.Buffer
	manifest, packageErr := packageBundleFilesTo(files, &tarBuffer)
	if packageErr != nil {
		return nil, nil, packageErr
	}
	return tarBuffer.Bytes(), manifest, nil
}

func packageBundleFilesTo(files map[string]string, writer io.Writer) (map[string]any, error) {
	tarWriter := tar.NewWriter(writer)
	manifest := make(map[string]any, len(files))

	filenames := make([]string, 0, len(files))
	for name := range files {
		filenames = append(filenames, name)
	}
	sort.Strings(filenames)

	for _, name := range filenames {
		cleanName := filepath.Clean(name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return nil, fmt.Errorf("invalid bundle file path: %s", name)
		}
		content := []byte(files[name])
		header := &tar.Header{
			Name:    filepath.ToSlash(cleanName),
			Mode:    filePermissions,
			Size:    int64(len(content)),
			ModTime: time.Unix(0, 0),
		}
		if writeHeaderErr := tarWriter.WriteHeader(header); writeHeaderErr != nil {
			return nil, fmt.Errorf("failed to write tar header: %w", writeHeaderErr)
		}
		if _, writeContentErr := tarWriter.Write(content); writeContentErr != nil {
			return nil, fmt.Errorf("failed to write tar content: %w", writeContentErr)
		}
		manifest[cleanName] = map[string]any{
			"size": len(content),
		}
	}
	if closeErr := tarWriter.Close(); closeErr != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", closeErr)
	}
	return manifest, nil
}

// handleCreateDeployment handles POST /v1/_/function/endpoints/{id}/deploy uploading and deploying code.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateDeployment(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create deployment request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointDeploy) {
		return
	}

	endpointIDString := request.PathValue("id")
	endpointID, parseErr := uuid.Parse(endpointIDString)
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	existingEndpoint, fetchErr := controlPlaneHandler.fetchEndpointByID(request.Context(), endpointID)
	if fetchErr != nil {
		if errors.Is(fetchErr, ErrEndpointNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fetchErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchErr.Error())
		return
	}

	var createDeploymentInput CreateDeploymentInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createDeploymentInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	var bundleBytes []byte
	var bundleFormat string
	var bundleFilesManifest map[string]any

	if len(createDeploymentInput.BundleFiles) > 0 {
		packedBytes, manifest, packErr := packageBundleFiles(createDeploymentInput.BundleFiles)
		if packErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, packErr.Error())
			return
		}
		bundleBytes = packedBytes
		bundleFormat = "tar"
		bundleFilesManifest = manifest
	} else {
		bundleFormat = strings.TrimSpace(createDeploymentInput.BundleFormat)
		if bundleFormat == "" {
			bundleFormat = "raw"
		}
		bundleBytes = decodeCodeBundle(createDeploymentInput.BundleContent)
		bundleFilesManifest = map[string]any{
			existingEndpoint.Entrypoint: map[string]any{"size": len(bundleBytes)},
		}
	}

	maxBundleSizeBytes := controlPlaneHandler.configManager.Get().MaxBundleSizeBytes
	if maxBundleSizeBytes > 0 && int64(len(bundleBytes)) > maxBundleSizeBytes {
		core.WriteErrorResponse(responseWriter, request, http.StatusRequestEntityTooLarge, ErrBundleTooLarge.Error())
		return
	}

	sha256Hash := sha256.New()
	sha256Hash.Write(bundleBytes)
	bundleHash := hex.EncodeToString(sha256Hash.Sum(nil))

	environmentVariables := createDeploymentInput.EnvironmentVariables
	if environmentVariables == nil {
		environmentVariables = make(map[string]string)
	}

	environmentJSON, _ := json.Marshal(environmentVariables)
	filesJSON, _ := json.Marshal(bundleFilesManifest)

	var workerdRuntimeConfigJSON []byte
	if createDeploymentInput.WorkerdRuntimeConfig != nil {
		workerdRuntimeConfigJSON, _ = json.Marshal(createDeploymentInput.WorkerdRuntimeConfig)
	}

	const insertDeploymentSQL = `
		WITH new_dep AS (
			INSERT INTO function.deployments (
				endpoint_id, version, bundle_format, bundle_content, bundle_hash, bundle_files, environment_variables, workerd_runtime_config, status, created_at
			) VALUES (
				$1,
				(SELECT COALESCE(MAX(version), 0) + 1 FROM function.deployments WHERE endpoint_id = $1),
				$2, $3, $4, $5, $6, $7, 'active', clock_timestamp()
			)
			RETURNING id, endpoint_id, version, bundle_format, bundle_hash, bundle_files, workerd_runtime_config, status, created_at
		),
		updated_ep AS (
			UPDATE function.endpoints f
			SET active_deployment_id = new_dep.id, updated_at = clock_timestamp()
			FROM new_dep
			WHERE f.id = new_dep.endpoint_id
		)
		SELECT id, endpoint_id, version, bundle_format, bundle_hash, bundle_files, workerd_runtime_config, status, created_at FROM new_dep;
	`

	var deployment Deployment
	deployment.BundleFormat = bundleFormat
	deployment.BundleContent = bundleBytes
	deployment.BundleFiles = bundleFilesManifest
	deployment.EnvironmentVariables = environmentVariables
	deployment.WorkerdRuntimeConfig = createDeploymentInput.WorkerdRuntimeConfig

	var rawFilesJSON []byte
	var rawWorkerdRuntimeConfigJSON []byte
	queryErr := controlPlaneHandler.kernel.DB().QueryRow(
		request.Context(),
		insertDeploymentSQL,
		endpointID,
		bundleFormat,
		bundleBytes,
		bundleHash,
		filesJSON,
		environmentJSON,
		workerdRuntimeConfigJSON,
	).Scan(
		&deployment.ID,
		&deployment.EndpointID,
		&deployment.Version,
		&deployment.BundleFormat,
		&deployment.BundleHash,
		&rawFilesJSON,
		&rawWorkerdRuntimeConfigJSON,
		&deployment.Status,
		&deployment.CreatedAt,
	)
	if queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}

	if len(rawFilesJSON) > 0 {
		_ = json.Unmarshal(rawFilesJSON, &deployment.BundleFiles)
	}
	if len(rawWorkerdRuntimeConfigJSON) > 0 && string(rawWorkerdRuntimeConfigJSON) != "null" {
		_ = json.Unmarshal(rawWorkerdRuntimeConfigJSON, &deployment.WorkerdRuntimeConfig)
	}

	// Deploy to the runtime runner
	existingEndpoint.ActiveDeploymentID = &deployment.ID
	if deployErr := controlPlaneHandler.engine.Deploy(request.Context(), existingEndpoint, &deployment); deployErr != nil {
		_, _ = controlPlaneHandler.kernel.DB().Exec(request.Context(), "DELETE FROM function.deployments WHERE id = $1;", deployment.ID)
		if errors.Is(deployErr, ErrRunnerDownloadFailed) {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadGateway, deployErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, deployErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewDeploymentCreatedEvent(deployment.ID.String(), DeploymentCreatedEventData(deployment)),
	)

	log.Debugf("deployment %s successfully created", deployment.ID)
	core.WriteJSONResponse(responseWriter, http.StatusCreated, deployment)
}

// handleListDeployments handles GET /v1/_/function/endpoints/{id}/deployments returning deployment history.
func (controlPlaneHandler *ControlPlaneHandler) handleListDeployments(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list deployments request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointRead) {
		return
	}

	endpointIDString := request.PathValue("id")
	endpointID, parseErr := uuid.Parse(endpointIDString)
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	const querySQL = `
		SELECT id, endpoint_id, version, bundle_format, bundle_hash, bundle_files, environment_variables, workerd_runtime_config, status, created_at
		FROM function.deployments
		WHERE endpoint_id = $1
		ORDER BY version DESC;
	`

	rows, queryErr := controlPlaneHandler.kernel.DB().Query(request.Context(), querySQL, endpointID)
	if queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}
	defer rows.Close()

	deployments := make([]Deployment, 0)
	for rows.Next() {
		var deployment Deployment
		var rawEnvironmentJSON []byte
		var rawFilesJSON []byte
		var rawWorkerdRuntimeConfigJSON []byte
		_ = rows.Scan(
			&deployment.ID,
			&deployment.EndpointID,
			&deployment.Version,
			&deployment.BundleFormat,
			&deployment.BundleHash,
			&rawFilesJSON,
			&rawEnvironmentJSON,
			&rawWorkerdRuntimeConfigJSON,
			&deployment.Status,
			&deployment.CreatedAt,
		)
		if len(rawFilesJSON) > 0 {
			_ = json.Unmarshal(rawFilesJSON, &deployment.BundleFiles)
		}
		if len(rawEnvironmentJSON) > 0 {
			_ = json.Unmarshal(rawEnvironmentJSON, &deployment.EnvironmentVariables)
		}
		if len(rawWorkerdRuntimeConfigJSON) > 0 && string(rawWorkerdRuntimeConfigJSON) != "null" {
			_ = json.Unmarshal(rawWorkerdRuntimeConfigJSON, &deployment.WorkerdRuntimeConfig)
		}
		deployments = append(deployments, deployment)
	}

	log.Debugf("retrieved %d deployment(s)", len(deployments))
	core.WriteJSONResponse(responseWriter, http.StatusOK, ListDeploymentsResponse{
		Deployments: deployments,
		Count:       len(deployments),
	})
}

// handleRollbackDeployment handles POST /v1/_/function/endpoints/{id}/rollback restoring an earlier deployment.
func (controlPlaneHandler *ControlPlaneHandler) handleRollbackDeployment(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling rollback deployment request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointDeploy) {
		return
	}

	endpointIDString := request.PathValue("id")
	endpointID, parseErr := uuid.Parse(endpointIDString)
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	existingEndpoint, fetchErr := controlPlaneHandler.fetchEndpointByID(request.Context(), endpointID)
	if fetchErr != nil {
		if errors.Is(fetchErr, ErrEndpointNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fetchErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchErr.Error())
		return
	}

	var rollbackDeploymentInput RollbackDeploymentInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&rollbackDeploymentInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	const querySQL = `
		SELECT id, endpoint_id, version, bundle_format, bundle_content, bundle_hash, bundle_files, environment_variables, workerd_runtime_config, status, created_at
		FROM function.deployments
		WHERE endpoint_id = $1 AND version = $2;
	`

	var deployment Deployment
	var rawEnvironmentJSON []byte
	var rawFilesJSON []byte
	var rawWorkerdRuntimeConfigJSON []byte
	queryErr := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), querySQL, endpointID, rollbackDeploymentInput.TargetVersion).Scan(
		&deployment.ID,
		&deployment.EndpointID,
		&deployment.Version,
		&deployment.BundleFormat,
		&deployment.BundleContent,
		&deployment.BundleHash,
		&rawFilesJSON,
		&rawEnvironmentJSON,
		&rawWorkerdRuntimeConfigJSON,
		&deployment.Status,
		&deployment.CreatedAt,
	)
	if queryErr != nil {
		if errors.Is(queryErr, pgx.ErrNoRows) || errors.Is(queryErr, sql.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, ErrDeploymentNotFound.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}

	if len(rawFilesJSON) > 0 {
		_ = json.Unmarshal(rawFilesJSON, &deployment.BundleFiles)
	}
	if len(rawEnvironmentJSON) > 0 {
		_ = json.Unmarshal(rawEnvironmentJSON, &deployment.EnvironmentVariables)
	}
	if len(rawWorkerdRuntimeConfigJSON) > 0 && string(rawWorkerdRuntimeConfigJSON) != "null" {
		_ = json.Unmarshal(rawWorkerdRuntimeConfigJSON, &deployment.WorkerdRuntimeConfig)
	}

	var previousDeploymentID *uuid.UUID
	if existingEndpoint.ActiveDeploymentID != nil {
		previousActiveID := *existingEndpoint.ActiveDeploymentID
		previousDeploymentID = &previousActiveID
	}

	const updateSQL = `
		UPDATE function.endpoints
		SET active_deployment_id = $1, updated_at = clock_timestamp()
		WHERE id = $2;
	`
	if _, updateErr := controlPlaneHandler.kernel.DB().Exec(request.Context(), updateSQL, deployment.ID, endpointID); updateErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, updateErr.Error())
		return
	}

	existingEndpoint.ActiveDeploymentID = &deployment.ID
	if deployErr := controlPlaneHandler.engine.Deploy(request.Context(), existingEndpoint, &deployment); deployErr != nil {
		if previousDeploymentID != nil {
			_, _ = controlPlaneHandler.kernel.DB().Exec(request.Context(), updateSQL, *previousDeploymentID, endpointID)
		}
		if errors.Is(deployErr, ErrRunnerDownloadFailed) {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadGateway, deployErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, deployErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewDeploymentRolledBackEvent(deployment.ID.String(), DeploymentRolledBackEventData(deployment)),
	)

	log.Debugf("deployment %s successfully rolled back", deployment.ID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, deployment)
}
