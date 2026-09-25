// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

type capturingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	stdout     string
	stderr     string
}

func (capturingWriter *capturingResponseWriter) WriteHeader(code int) {
	capturingWriter.statusCode = code
	if rawStdout := capturingWriter.Header().Get("X-Layr-Stdout"); rawStdout != "" {
		if unescaped, unescapeErr := url.QueryUnescape(rawStdout); unescapeErr == nil {
			capturingWriter.stdout = unescaped
		}
		capturingWriter.Header().Del("X-Layr-Stdout")
	}
	if rawStderr := capturingWriter.Header().Get("X-Layr-Stderr"); rawStderr != "" {
		if unescaped, unescapeErr := url.QueryUnescape(rawStderr); unescapeErr == nil {
			capturingWriter.stderr = unescaped
		}
		capturingWriter.Header().Del("X-Layr-Stderr")
	}
	capturingWriter.ResponseWriter.WriteHeader(code)
}

func (capturingWriter *capturingResponseWriter) Write(contentBytes []byte) (int, error) {
	if capturingWriter.statusCode == 0 {
		capturingWriter.WriteHeader(http.StatusOK)
	}
	bytesWritten, writeErr := capturingWriter.ResponseWriter.Write(contentBytes)
	if writeErr != nil {
		return bytesWritten, fmt.Errorf("failed to write response bytes: %w", writeErr)
	}
	return bytesWritten, nil
}

func (baseHandler *BaseHandler) fetchEndpointByName(ctx context.Context, name string) (*Endpoint, error) {
	const querySQL = `
		SELECT id, name, description, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, active_deployment_id, created_at, updated_at
		FROM function.endpoints
		WHERE name = $1;
	`

	var storedEndpoint Endpoint
	scanErr := baseHandler.kernel.DB().QueryRow(ctx, querySQL, name).Scan(
		&storedEndpoint.ID,
		&storedEndpoint.Name,
		&storedEndpoint.Description,
		&storedEndpoint.Runtime,
		&storedEndpoint.Entrypoint,
		&storedEndpoint.MemoryLimitMB,
		&storedEndpoint.TimeoutSeconds,
		&storedEndpoint.IsPublic,
		&storedEndpoint.ActiveDeploymentID,
		&storedEndpoint.CreatedAt,
		&storedEndpoint.UpdatedAt,
	)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return nil, ErrEndpointNotFound
		}
		return nil, fmt.Errorf("failed to query endpoint by name: %w", scanErr)
	}

	return &storedEndpoint, nil
}

func (baseHandler *BaseHandler) fetchActiveDeployment(ctx context.Context, endpointID uuid.UUID) (*Deployment, error) {
	const querySQL = `
		SELECT d.id, d.endpoint_id, d.version, d.bundle_format, d.bundle_content, d.bundle_hash, d.bundle_files, d.environment_variables, d.workerd_runtime_config, d.status, d.created_at
		FROM function.deployments d
		JOIN function.endpoints f ON f.active_deployment_id = d.id
		WHERE f.id = $1;
	`

	var deployment Deployment
	var rawEnvironmentJSON []byte
	var rawFilesJSON []byte
	var rawWorkerdRuntimeConfigJSON []byte
	queryErr := baseHandler.kernel.DB().QueryRow(ctx, querySQL, endpointID).Scan(
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
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return nil, ErrNoActiveDeployment
		}
		return nil, fmt.Errorf("failed to query active deployment: %w", queryErr)
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

	return &deployment, nil
}

// handleInvokeEndpoint reverse-proxies public HTTP requests to the target endpoint's active deployment.
func (baseHandler *BaseHandler) handleInvokeEndpoint(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling endpoint invoke request")
	name := request.PathValue("endpoint_name")
	if name == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Endpoint name is required")
		return
	}

	requestCtx := request.Context()
	storedEndpoint, getErr := baseHandler.fetchEndpointByName(requestCtx, name)
	if getErr != nil {
		if errors.Is(getErr, ErrEndpointNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Endpoint not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, getErr.Error())
		return
	}

	activeDeployment, depErr := baseHandler.fetchActiveDeployment(requestCtx, storedEndpoint.ID)
	if depErr != nil && !errors.Is(depErr, ErrNoActiveDeployment) {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, depErr.Error())
		return
	}

	baseHandler.invokeEndpoint(responseWriter, request, storedEndpoint, activeDeployment)
}

func (baseHandler *BaseHandler) invokeEndpoint(
	responseWriter http.ResponseWriter,
	request *http.Request,
	endpoint *Endpoint,
	deployment *Deployment,
) {
	requestCtx := request.Context()

	if !endpoint.IsPublic {
		authContext := core.GetAuthContext(requestCtx)
		if authContext.IsServiceAccount() {
			if !authContext.HasScope(core.ScopeFunctionEndpointInvoke) {
				core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied")
				return
			}
		} else if !authContext.IsAuthenticated() {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
			return
		}
	}

	if deployment == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusServiceUnavailable, "No active deployment")
		return
	}

	injectAuthHeaders(request)

	capturingWriter := &capturingResponseWriter{ResponseWriter: responseWriter}
	startTime := time.Now()
	forwardErr := baseHandler.engine.Forward(capturingWriter, request, endpoint)
	durationMs := time.Since(startTime).Milliseconds()

	statusCode := capturingWriter.statusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	var errorMessage *string
	if forwardErr != nil {
		forwardErrorMessage := forwardErr.Error()
		errorMessage = &forwardErrorMessage
		if statusCode < http.StatusBadRequest {
			statusCode = http.StatusBadGateway
			core.WriteErrorResponse(capturingWriter, request, http.StatusBadGateway, forwardErrorMessage)
		}
	}

	deploymentID := &deployment.ID

	const insertExecutionSQL = `
		INSERT INTO function.executions (
			endpoint_id, deployment_id, method, path, status_code, duration_ms, stdout, stderr, error_message
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		) RETURNING id, endpoint_id, deployment_id, method, path, status_code, duration_ms, stdout, stderr, error_message, created_at;
	`

	var recordedExecution Execution
	_ = baseHandler.kernel.DB().QueryRow(
		requestCtx,
		insertExecutionSQL,
		endpoint.ID,
		deploymentID,
		request.Method,
		request.URL.Path,
		statusCode,
		durationMs,
		capturingWriter.stdout,
		capturingWriter.stderr,
		errorMessage,
	).Scan(
		&recordedExecution.ID,
		&recordedExecution.EndpointID,
		&recordedExecution.DeploymentID,
		&recordedExecution.Method,
		&recordedExecution.Path,
		&recordedExecution.StatusCode,
		&recordedExecution.DurationMs,
		&recordedExecution.Stdout,
		&recordedExecution.Stderr,
		&recordedExecution.ErrorMessage,
		&recordedExecution.CreatedAt,
	)

	if forwardErr != nil {
		baseHandler.kernel.EventBus().Publish(requestCtx, NewExecutionFailedEvent(recordedExecution.ID.String(), ExecutionFailedEventData(recordedExecution)))
	} else {
		baseHandler.kernel.EventBus().Publish(requestCtx, NewExecutionCompletedEvent(recordedExecution.ID.String(), ExecutionCompletedEventData(recordedExecution)))
	}
}

func injectAuthHeaders(request *http.Request) {
	requestCtx := request.Context()
	authContext := core.GetAuthContext(requestCtx)

	authContextJSON, marshalErr := json.Marshal(authContext)
	if marshalErr == nil {
		request.Header.Set("X-Layr-Auth-Context", string(authContextJSON))
	}
}
