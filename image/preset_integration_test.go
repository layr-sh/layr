package image

import (
	"context"
	"testing"

	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImagePresetPostgresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	presetManager := NewPresetManager(kernel)

	// 1. Initial Load
	require.NoError(t, presetManager.Load(ctx))

	// 2. Create preset with description
	desc := "Large hero banner"
	createdPreset, createErr := presetManager.Create(ctx, CreatePresetInput{
		Name:              "hero_banner",
		ProcessingOptions: "rs:fill:1200:600/q:90",
		Description:       &desc,
	})
	require.NoError(t, createErr)
	require.Equal(t, "hero_banner", createdPreset.Name)
	require.Equal(t, &desc, createdPreset.Description)

	// 3. Duplicate name returns error
	_, duplicateErr := presetManager.Create(ctx, CreatePresetInput{
		Name:              "hero_banner",
		ProcessingOptions: "rs:fill:800:400",
	})
	require.Error(t, duplicateErr)

	// 4. Get by ID
	retrievedPreset, getErr := presetManager.GetByID(ctx, createdPreset.ID)
	require.NoError(t, getErr)
	require.Equal(t, createdPreset.ID, retrievedPreset.ID)

	// 5. List presets
	presets, listErr := presetManager.List(ctx)
	require.NoError(t, listErr)
	require.NotEmpty(t, presets)

	// 6. Resolve options from memory cache
	options, resolveErr := presetManager.ResolveOptions("hero_banner")
	require.NoError(t, resolveErr)
	require.Equal(t, "rs:fill:1200:600/q:90", options)

	// 7. Update preset
	updatedOptions := "rs:fill:1400:700/q:95"
	updatedPreset, updateErr := presetManager.Update(ctx, createdPreset.ID, UpdatePresetInput{
		ProcessingOptions: &updatedOptions,
	})
	require.NoError(t, updateErr)
	require.Equal(t, updatedOptions, updatedPreset.ProcessingOptions)

	// 8. Delete preset
	deletedPreset, deleteErr := presetManager.Delete(ctx, createdPreset.ID)
	require.NoError(t, deleteErr)
	require.Equal(t, "hero_banner", deletedPreset.Name)

	// 9. Nonexistent Get returns error
	_, notFoundErr := presetManager.GetByID(ctx, uuid.New())
	require.Error(t, notFoundErr)

	// 10. Update database error on unique name conflict
	firstPreset, createFirstErr := presetManager.Create(ctx, CreatePresetInput{
		Name:              "preset_alpha",
		ProcessingOptions: "rs:fill:100:100",
	})
	require.NoError(t, createFirstErr)

	secondPreset, createSecondErr := presetManager.Create(ctx, CreatePresetInput{
		Name:              "preset_beta",
		ProcessingOptions: "rs:fill:200:200",
	})
	require.NoError(t, createSecondErr)

	conflictingName := "preset_alpha"
	_, updateConflictErr := presetManager.Update(ctx, secondPreset.ID, UpdatePresetInput{
		Name: &conflictingName,
	})
	require.Error(t, updateConflictErr)
	require.Contains(t, updateConflictErr.Error(), "failed to update preset")

	// 11. Delete database error on foreign key constraint violation
	_, createRefTableErr := kernel.DB().Exec(ctx, "CREATE TABLE image.preset_test_refs (preset_id UUID REFERENCES image.presets(id));")
	require.NoError(t, createRefTableErr)
	_, insertRefErr := kernel.DB().Exec(ctx, "INSERT INTO image.preset_test_refs (preset_id) VALUES ($1);", firstPreset.ID)
	require.NoError(t, insertRefErr)
	_, deleteConflictErr := presetManager.Delete(ctx, firstPreset.ID)
	require.Error(t, deleteConflictErr)
	require.Contains(t, deleteConflictErr.Error(), "failed to delete preset")
	_, dropRefTableErr := kernel.DB().Exec(ctx, "DROP TABLE image.preset_test_refs;")
	require.NoError(t, dropRefTableErr)
}
