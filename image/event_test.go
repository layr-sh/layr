package image

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageEventUnit(t *testing.T) {
	t.Parallel()

	t.Run("config updated event creation", func(t *testing.T) {
		t.Parallel()
		configUpdatedEventData := ConfigUpdatedEventData(DefaultConfig())
		event := NewConfigUpdatedEvent("cfg-1", configUpdatedEventData)
		require.Equal(t, "image.config.updated", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "cfg-1", *event.ResourceID)
	})

	t.Run("preset created event creation", func(t *testing.T) {
		t.Parallel()
		presetCreatedEventData := PresetCreatedEventData(Preset{Name: "thumbnail"})
		event := NewPresetCreatedEvent("preset-1", presetCreatedEventData)
		require.Equal(t, "image.preset.created", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "preset-1", *event.ResourceID)
	})

	t.Run("preset updated event creation", func(t *testing.T) {
		t.Parallel()
		presetUpdatedEventData := PresetUpdatedEventData(Preset{Name: "thumbnail-lg"})
		event := NewPresetUpdatedEvent("preset-2", presetUpdatedEventData)
		require.Equal(t, "image.preset.updated", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "preset-2", *event.ResourceID)
	})

	t.Run("preset deleted event creation", func(t *testing.T) {
		t.Parallel()
		presetDeletedEventData := PresetDeletedEventData{PresetName: "thumbnail"}
		event := NewPresetDeletedEvent("preset-3", presetDeletedEventData)
		require.Equal(t, "image.preset.deleted", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "preset-3", *event.ResourceID)
	})

	t.Run("image transform completed event creation", func(t *testing.T) {
		t.Parallel()
		transformCompletedEventData := TransformCompletedEventData{
			SourceURL: "local/bucket/photo.jpg",
			Format:    "jpeg",
			ByteSize:  2048,
			Width:     100,
			Height:    100,
			CacheHit:  true,
		}
		event := NewTransformCompletedEvent("local/bucket/photo.jpg", transformCompletedEventData)
		require.Equal(t, "image.transform.completed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "local/bucket/photo.jpg", *event.ResourceID)
	})

	t.Run("transform failed event creation", func(t *testing.T) {
		t.Parallel()
		transformFailedEventData := TransformFailedEventData{
			SourceURL:  "local/bucket/missing.jpg",
			Reason:     "Image asset not found",
			StatusCode: 404,
		}
		event := NewTransformFailedEvent("local/bucket/missing.jpg", transformFailedEventData)
		require.Equal(t, "image.transform.failed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "local/bucket/missing.jpg", *event.ResourceID)
	})

	t.Run("inspect completed event creation", func(t *testing.T) {
		t.Parallel()
		inspectCompletedEventData := InspectCompletedEventData{
			SourceURL: "local/bucket/photo.jpg",
			Format:    "png",
			ByteSize:  4096,
		}
		event := NewInspectCompletedEvent("local/bucket/photo.jpg", inspectCompletedEventData)
		require.Equal(t, "image.inspect.completed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "local/bucket/photo.jpg", *event.ResourceID)
	})

	t.Run("cache flushed event creation", func(t *testing.T) {
		t.Parallel()
		cacheFlushedEventData := CacheFlushedEventData{
			SourceURL: "*",
		}
		event := NewCacheFlushedEvent("*", cacheFlushedEventData)
		require.Equal(t, "image.cache.flushed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "*", *event.ResourceID)
	})

	t.Run("cache invalidated event creation", func(t *testing.T) {
		t.Parallel()
		cacheInvalidatedEventData := CacheInvalidatedEventData{
			CacheKey:  "key-123",
			SourceURL: "local/bucket/photo.jpg",
		}
		event := NewCacheInvalidatedEvent("key-123", cacheInvalidatedEventData)
		require.Equal(t, "image.cache.invalidated", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "key-123", *event.ResourceID)
	})

	t.Run("inspect failed event creation", func(t *testing.T) {
		t.Parallel()
		inspectFailedEventData := InspectFailedEventData{
			SourceURL:  "local/bucket/corrupt.png",
			Reason:     "failed to introspect image metadata",
			StatusCode: 422,
		}
		event := NewInspectFailedEvent("local/bucket/corrupt.png", inspectFailedEventData)
		require.Equal(t, "image.inspect.failed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "local/bucket/corrupt.png", *event.ResourceID)
	})

	t.Run("threat ssrf blocked event creation", func(t *testing.T) {
		t.Parallel()
		threatSSRFBlockedEventData := ThreatSSRFBlockedEventData{
			SourceURL:  "http://169.254.169.254/latest/meta-data/",
			ResolvedIP: "169.254.169.254",
			Reason:     "blocked restricted or private IP address",
		}
		event := NewThreatSSRFBlockedEvent("http://169.254.169.254/latest/meta-data/", threatSSRFBlockedEventData)
		require.Equal(t, "image.threat.ssrf_blocked", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "http://169.254.169.254/latest/meta-data/", *event.ResourceID)
	})

	t.Run("url signed event creation", func(t *testing.T) {
		t.Parallel()
		urlSignedEventData := URLSignedEventData{
			Path:      "/rs:fill:300:200/plain/bucket/img.jpg",
			URL:       "/v1/image/sig/rs:fill:300:200/plain/bucket/img.jpg",
			Signature: "sig",
		}
		event := NewURLSignedEvent("/rs:fill:300:200/plain/bucket/img.jpg", urlSignedEventData)
		require.Equal(t, "image.url.signed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "/rs:fill:300:200/plain/bucket/img.jpg", *event.ResourceID)
	})
}

func TestImageEventConstructorParameterSignaturesUnit(t *testing.T) {
	fileSet := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fileSet, "event.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse event.go: %v", err)
	}

	testedCount := 0
	for _, decl := range parsedFile.Decls {
		functionDeclaration, ok := decl.(*ast.FuncDecl)
		if !ok || functionDeclaration.Recv != nil {
			continue
		}
		name := functionDeclaration.Name.Name
		if !strings.HasPrefix(name, "New") || !strings.HasSuffix(name, "Event") || name == "NewEvent" {
			continue
		}

		testedCount++
		t.Run(name, func(t *testing.T) {
			params := functionDeclaration.Type.Params.List
			if len(params) == 0 {
				t.Fatalf("constructor %s has no parameters", name)
			}
			firstParamField := params[0]
			if len(firstParamField.Names) == 0 {
				t.Fatalf("constructor %s first parameter has no name", name)
			}
			paramName := firstParamField.Names[0].Name
			if paramName != "resourceID" {
				t.Errorf("constructor %s first parameter expected 'resourceID', got '%s'", name, paramName)
			}
		})
	}

	if testedCount == 0 {
		t.Fatal("no constructors found to test")
	}
}
