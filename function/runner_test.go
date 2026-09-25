package function

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	name         string
	startErr     error
	stopErr      error
	deployErr    error
	undeployErr  error
	forwardErr   error
	healthErr    error
	runnerHealth *RunnerHealth
}

func (runner *mockRunner) Name() string {
	return runner.name
}

func (runner *mockRunner) Start(ctx context.Context) error {
	return runner.startErr
}

func (runner *mockRunner) Stop() error {
	return runner.stopErr
}

func (runner *mockRunner) Deploy(ctx context.Context, targetEndpoint *Endpoint, deployment *Deployment) error {
	return runner.deployErr
}

func (runner *mockRunner) Undeploy(ctx context.Context, endpointName string) error {
	return runner.undeployErr
}

func (runner *mockRunner) Forward(
	responseWriter http.ResponseWriter,
	request *http.Request,
	targetEndpoint *Endpoint,
) error {
	if runner.forwardErr != nil {
		return runner.forwardErr
	}
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = responseWriter.Write([]byte("mock response"))
	return nil
}

func (runner *mockRunner) Health(ctx context.Context) (*RunnerHealth, error) {
	if runner.healthErr != nil {
		return nil, runner.healthErr
	}
	return runner.runnerHealth, nil
}

func TestFunctionRunnerEngineUnit(t *testing.T) {
	t.Parallel()

	t.Run("nil runner registration ignored", func(t *testing.T) {
		t.Parallel()
		engine := NewEngine()
		engine.RegisterRunner(nil)
		_, err := engine.GetRunner("any")
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrUnsupportedRuntime))
	})

	t.Run("runner lifecycle delegation", func(t *testing.T) {
		t.Parallel()
		engine := NewEngine()
		testMockRunner := &mockRunner{
			name: "test-runtime",
			runnerHealth: &RunnerHealth{
				Available:      true,
				ActiveIsolates: 1,
			},
		}
		engine.RegisterRunner(testMockRunner)

		ctx := context.Background()
		require.NoError(t, engine.Start(ctx))

		retrievedRunner, err := engine.GetRunner("test-runtime")
		require.NoError(t, err)
		require.Equal(t, "test-runtime", retrievedRunner.Name())

		testEndpoint := &Endpoint{Name: "test-func", Runtime: "test-runtime"}
		testDeployment := &Deployment{Version: 1}

		require.NoError(t, engine.Deploy(ctx, testEndpoint, testDeployment))
		require.NoError(t, engine.Undeploy(ctx, "test-runtime", "test-func"))

		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/function/test-func", nil)
		require.NoError(t, engine.Forward(responseRecorder, request, testEndpoint))
		require.Equal(t, http.StatusOK, responseRecorder.Code)
		require.Equal(t, "mock response", responseRecorder.Body.String())

		healthMap, healthErr := engine.Health(ctx)
		require.NoError(t, healthErr)
		require.True(t, healthMap["test-runtime"].Available)

		require.NoError(t, engine.Stop())
	})

	t.Run("runner error paths", func(t *testing.T) {
		t.Parallel()
		engine := NewEngine()
		failingRunner := &mockRunner{
			name:        "failing",
			startErr:    errors.New("start failed"),
			stopErr:     errors.New("stop failed"),
			deployErr:   errors.New("deploy failed"),
			undeployErr: errors.New("undeploy failed"),
			forwardErr:  errors.New("forward failed"),
			healthErr:   errors.New("health check failed"),
		}
		engine.RegisterRunner(failingRunner)

		ctx := context.Background()
		require.Error(t, engine.Start(ctx))

		testEndpoint := &Endpoint{Name: "err-func", Runtime: "failing"}
		testDeployment := &Deployment{Version: 1}

		require.Error(t, engine.Deploy(ctx, testEndpoint, testDeployment))
		require.Error(t, engine.Undeploy(ctx, "failing", "err-func"))

		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/function/err-func", nil)
		require.Error(t, engine.Forward(responseRecorder, request, testEndpoint))

		healthMap, _ := engine.Health(ctx)
		require.False(t, healthMap["failing"].Available)
		require.Equal(t, "health check failed", healthMap["failing"].ErrorMessage)

		require.Error(t, engine.Stop())
	})

	t.Run("nil endpoint parameter returns ErrEndpointNotFound", func(t *testing.T) {
		t.Parallel()
		engine := NewEngine()
		ctx := context.Background()
		require.ErrorIs(t, engine.Deploy(ctx, nil, nil), ErrEndpointNotFound)

		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		require.ErrorIs(t, engine.Forward(responseRecorder, request, nil), ErrEndpointNotFound)
	})

	t.Run("unsupported runtime errors", func(t *testing.T) {
		t.Parallel()
		engine := NewEngine()
		ctx := context.Background()
		testEndpoint := &Endpoint{Name: "unknown", Runtime: "unknown-runtime"}

		require.ErrorIs(t, engine.Deploy(ctx, testEndpoint, nil), ErrUnsupportedRuntime)
		require.ErrorIs(t, engine.Undeploy(ctx, "unknown-runtime", "unknown"), ErrUnsupportedRuntime)

		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		require.ErrorIs(t, engine.Forward(responseRecorder, request, testEndpoint), ErrUnsupportedRuntime)
	})
}
