package image

import (
	"context"
	"testing"

	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImagePresetUnit(t *testing.T) {
	t.Parallel()

	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	presetManager := NewPresetManager(kernel)
	ctx := context.Background()

	t.Run("create validation and duplicate errors", func(t *testing.T) {
		_, emptyNameErr := presetManager.Create(ctx, CreatePresetInput{
			Name:              "",
			ProcessingOptions: "rs:fill:100:100",
		})
		require.Error(t, emptyNameErr)

		_, emptyOptionsErr := presetManager.Create(ctx, CreatePresetInput{
			Name:              "bad-preset",
			ProcessingOptions: "",
		})
		require.Error(t, emptyOptionsErr)

		desc := "Standard thumbnail"
		createdPreset, createErr := presetManager.Create(ctx, CreatePresetInput{
			Name:              "thumb",
			ProcessingOptions: "rs:fill:150:150",
			Description:       &desc,
		})
		require.NoError(t, createErr)
		require.Equal(t, "thumb", createdPreset.Name)
		require.Equal(t, &desc, createdPreset.Description)

		// Duplicate
		_, duplicateErr := presetManager.Create(ctx, CreatePresetInput{
			Name:              "thumb",
			ProcessingOptions: "rs:fit:200:200",
		})
		require.Error(t, duplicateErr)
	})

	t.Run("get by id and list", func(t *testing.T) {
		presets, listErr := presetManager.List(ctx)
		require.NoError(t, listErr)
		require.NotEmpty(t, presets)

		// Get existing
		firstPreset := presets[0]
		foundPreset, getErr := presetManager.GetByID(ctx, firstPreset.ID)
		require.NoError(t, getErr)
		require.Equal(t, firstPreset.ID, foundPreset.ID)

		// Get nonexistent
		_, notFoundErr := presetManager.GetByID(ctx, uuid.New())
		require.Error(t, notFoundErr)
	})

	t.Run("resolve options from cache and load", func(t *testing.T) {
		options, resolveErr := presetManager.ResolveOptions("thumb")
		require.NoError(t, resolveErr)
		require.Equal(t, "rs:fill:150:150", options)

		_, notFoundErr := presetManager.ResolveOptions("nonexistent-preset")
		require.Error(t, notFoundErr)

		// Reload into fresh preset manager
		reloadedPresetManager := NewPresetManager(kernel)
		loadErr := reloadedPresetManager.Load(ctx)
		require.NoError(t, loadErr)

		reloadedOptions, reloadedResolveErr := reloadedPresetManager.ResolveOptions("thumb")
		require.NoError(t, reloadedResolveErr)
		require.Equal(t, "rs:fill:150:150", reloadedOptions)
	})

	t.Run("update and delete lifecycle", func(t *testing.T) {
		createdPreset, createErr := presetManager.Create(ctx, CreatePresetInput{
			Name:              "avatar-small",
			ProcessingOptions: "rs:fill:48:48",
		})
		require.NoError(t, createErr)

		updatedName := "avatar-thumb"
		updatedOptions := "rs:fill:64:64"
		updatedDesc := "Avatar thumbnail 64px"
		updatedPreset, updateErr := presetManager.Update(ctx, createdPreset.ID, UpdatePresetInput{
			Name:              &updatedName,
			ProcessingOptions: &updatedOptions,
			Description:       &updatedDesc,
		})
		require.NoError(t, updateErr)
		require.Equal(t, "avatar-thumb", updatedPreset.Name)
		require.Equal(t, "rs:fill:64:64", updatedPreset.ProcessingOptions)

		// Verify updated in cache
		resolvedOptions, resolveErr := presetManager.ResolveOptions("avatar-thumb")
		require.NoError(t, resolveErr)
		require.Equal(t, "rs:fill:64:64", resolvedOptions)

		// Delete
		deletedPreset, deleteErr := presetManager.Delete(ctx, createdPreset.ID)
		require.NoError(t, deleteErr)
		require.Equal(t, "avatar-thumb", deletedPreset.Name)

		// Cache reflects deletion
		_, missingErr := presetManager.ResolveOptions("avatar-thumb")
		require.Error(t, missingErr)

		// Delete again returns error
		_, deleteAgainErr := presetManager.Delete(ctx, createdPreset.ID)
		require.Error(t, deleteAgainErr)

		// Update nonexistent preset
		_, updateNonexistentErr := presetManager.Update(ctx, uuid.New(), UpdatePresetInput{})
		require.Error(t, updateNonexistentErr)
	})

	t.Run("preset operations with broken database fail gracefully", func(t *testing.T) {
		brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		brokenPresetManager := NewPresetManager(brokenKernel)
		ctx := context.Background()

		require.Error(t, brokenPresetManager.Load(ctx))

		_, listErr := brokenPresetManager.List(ctx)
		require.Error(t, listErr)

		_, createErr := brokenPresetManager.Create(ctx, CreatePresetInput{Name: "test", ProcessingOptions: "rs:fill:10:10"})
		require.Error(t, createErr)

		_, updateErr := brokenPresetManager.Update(ctx, uuid.New(), UpdatePresetInput{})
		require.Error(t, updateErr)

		_, deleteErr := brokenPresetManager.Delete(ctx, uuid.New())
		require.Error(t, deleteErr)
	})
}
