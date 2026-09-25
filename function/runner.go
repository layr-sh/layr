package function

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// Standard function service errors.
var (
	ErrEndpointNotFound     = errors.New("endpoint not found")
	ErrDeploymentNotFound   = errors.New("deployment not found")
	ErrUnsupportedRuntime   = errors.New("unsupported function runtime")
	ErrEngineNotInitialized = errors.New("function runtime engine is not initialized")
	ErrExecutionTimeout     = errors.New("function execution timed out")
	ErrExecutionFailed      = errors.New("function execution failed")
	ErrRunnerDownloadFailed = errors.New("failed to download runtime runner binary")
)

// RunnerHealth represents runtime status and telemetry.
type RunnerHealth struct {
	Available      bool   `json:"available"`
	ActiveIsolates int    `json:"active_isolates"`
	ProcessPID     int    `json:"process_pid,omitempty"`
	MemoryUsageMB  int64  `json:"memory_usage_mb"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

// Runner defines the pluggable execution backend contract.
type Runner interface {
	// Name returns the identifier of the runtime runner (e.g. "workerd", "docker").
	Name() string

	// Start initializes dependencies, checks binaries, and boots the runtime supervisor.
	Start(ctx context.Context) error

	// Stop gracefully terminates processes or containers.
	Stop() error

	// Deploy writes code bundles and triggers zero-downtime hot reload.
	Deploy(ctx context.Context, targetEndpoint *Endpoint, deployment *Deployment) error

	// Undeploy unloads an endpoint from the active runtime.
	Undeploy(ctx context.Context, endpointName string) error

	// Forward reverse-proxies an HTTP request to the designated endpoint isolate/container.
	Forward(
		responseWriter http.ResponseWriter,
		request *http.Request,
		targetEndpoint *Endpoint,
	) error

	// Health returns runtime health metrics and readiness.
	Health(ctx context.Context) (*RunnerHealth, error)
}

// Engine coordinates multi-runtime delegation across registered runners.
type Engine struct {
	rwMutex sync.RWMutex
	runners map[string]Runner
}

// NewEngine initializes a new multi-runtime Engine coordinator.
func NewEngine() *Engine {
	return &Engine{
		runners: make(map[string]Runner),
	}
}

// RegisterRunner registers an execution runner under its name.
func (engine *Engine) RegisterRunner(runner Runner) {
	if runner == nil {
		return
	}
	engine.rwMutex.Lock()
	defer engine.rwMutex.Unlock()
	engine.runners[runner.Name()] = runner
}

// GetRunner retrieves a registered runner by runtime name.
func (engine *Engine) GetRunner(name string) (Runner, error) {
	engine.rwMutex.RLock()
	defer engine.rwMutex.RUnlock()
	runner, exists := engine.runners[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedRuntime, name)
	}
	return runner, nil
}

// Start boots all registered runtime runners.
func (engine *Engine) Start(ctx context.Context) error {
	engine.rwMutex.RLock()
	defer engine.rwMutex.RUnlock()

	for name, runner := range engine.runners {
		if err := runner.Start(ctx); err != nil {
			return fmt.Errorf("failed to start %s runner: %w", name, err)
		}
	}
	return nil
}

// Stop gracefully shuts down all registered runtime runners.
func (engine *Engine) Stop() error {
	engine.rwMutex.RLock()
	defer engine.rwMutex.RUnlock()

	var stopErrors []error
	for name, runner := range engine.runners {
		if err := runner.Stop(); err != nil {
			stopErrors = append(stopErrors, fmt.Errorf("failed to stop %s runner: %w", name, err))
		}
	}
	if len(stopErrors) > 0 {
		return errors.Join(stopErrors...)
	}
	return nil
}

// Deploy delegates deployment to the appropriate runner based on endpoint runtime.
func (engine *Engine) Deploy(ctx context.Context, targetEndpoint *Endpoint, deployment *Deployment) error {
	if targetEndpoint == nil {
		return ErrEndpointNotFound
	}
	runner, err := engine.GetRunner(targetEndpoint.Runtime)
	if err != nil {
		return err
	}
	if deployErr := runner.Deploy(ctx, targetEndpoint, deployment); deployErr != nil {
		return fmt.Errorf("deploy failed: %w", deployErr)
	}
	return nil
}

// Undeploy delegates endpoint unloading to the designated runtime runner.
func (engine *Engine) Undeploy(ctx context.Context, runtime string, endpointName string) error {
	runner, err := engine.GetRunner(runtime)
	if err != nil {
		return err
	}
	if undeployErr := runner.Undeploy(ctx, endpointName); undeployErr != nil {
		return fmt.Errorf("undeploy failed: %w", undeployErr)
	}
	return nil
}

// Forward delegates HTTP request proxying to the designated runtime runner.
func (engine *Engine) Forward(
	responseWriter http.ResponseWriter,
	request *http.Request,
	targetEndpoint *Endpoint,
) error {
	if targetEndpoint == nil {
		return ErrEndpointNotFound
	}
	runner, err := engine.GetRunner(targetEndpoint.Runtime)
	if err != nil {
		return err
	}
	if forwardErr := runner.Forward(responseWriter, request, targetEndpoint); forwardErr != nil {
		return fmt.Errorf("forward failed: %w", forwardErr)
	}
	return nil
}

// Health aggregates health reports across all registered runners.
func (engine *Engine) Health(ctx context.Context) (map[string]*RunnerHealth, error) {
	engine.rwMutex.RLock()
	defer engine.rwMutex.RUnlock()

	results := make(map[string]*RunnerHealth)
	for name, runner := range engine.runners {
		runnerHealth, err := runner.Health(ctx)
		if err != nil {
			results[name] = &RunnerHealth{
				Available:    false,
				ErrorMessage: err.Error(),
			}
		} else {
			results[name] = runnerHealth
		}
	}
	return results, nil
}
