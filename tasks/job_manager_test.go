package tasks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksJobManagerUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	configManager := NewConfigManager(kernel)
	jobManager := NewJobManager(kernel, configManager)
	require.NotNil(t, jobManager)

	t.Run("validate target payload HTTP validations", func(t *testing.T) {
		t.Parallel()
		err := validateTargetPayload(TargetTypeHTTP, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "target_payload is required")

		err = validateTargetPayload(TargetTypeHTTP, []byte("{invalid-json"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid http target_payload")

		missingURLErr := validateTargetPayload(TargetTypeHTTP, []byte(`{"headers":{"a":"b"}}`))
		require.Error(t, missingURLErr)
		require.Contains(t, missingURLErr.Error(), "url is required")

		badURLErr := validateTargetPayload(TargetTypeHTTP, []byte(`{"url":"ftp://example.com"}`))
		require.Error(t, badURLErr)
		require.Contains(t, badURLErr.Error(), "valid http or https url is required")

		validHTTPPayload, marshalErr := json.Marshal(map[string]any{
			"url": "https://example.com/webhook",
		})
		require.NoError(t, marshalErr)
		require.NoError(t, validateTargetPayload(TargetTypeHTTP, validHTTPPayload))
	})

	t.Run("validate target payload SQL validations", func(t *testing.T) {
		t.Parallel()
		err := validateTargetPayload(TargetTypeSQL, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "target_payload is required")

		err = validateTargetPayload(TargetTypeSQL, []byte("{invalid-json"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid sql target_payload")

		missingQueryErr := validateTargetPayload(TargetTypeSQL, []byte(`{"params":[]}`))
		require.Error(t, missingQueryErr)
		require.Contains(t, missingQueryErr.Error(), "query is required")

		validSQLPayload, marshalErr := json.Marshal(map[string]any{
			"query": "SELECT 1",
		})
		require.NoError(t, marshalErr)
		require.NoError(t, validateTargetPayload(TargetTypeSQL, validSQLPayload))
	})

	t.Run("validate target payload unknown target type returns error", func(t *testing.T) {
		t.Parallel()
		err := validateTargetPayload("unsupported", []byte(`{}`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported target_type")
	})
}
