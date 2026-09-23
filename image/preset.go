package image

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// Standard preset errors.
var (
	ErrPresetNotFound = errors.New("preset not found")
	ErrPresetExists   = errors.New("preset with this name already exists")
)

// PresetManager manages named transformation presets stored in image.presets.
type PresetManager struct {
	kernel  *core.Kernel
	rwMutex sync.RWMutex
	cache   map[string]Preset
}

// NewPresetManager initializes a preset manager instance.
func NewPresetManager(kernel *core.Kernel) *PresetManager {
	return &PresetManager{
		kernel: kernel,
		cache:  make(map[string]Preset),
	}
}

// Load populates the in-memory preset cache from PostgreSQL.
func (presetManager *PresetManager) Load(ctx context.Context) error {
	const selectSQLStatement = `
		SELECT id, name, processing_options, description, created_at, updated_at
		FROM image.presets;
	`

	rows, queryErr := presetManager.kernel.DB().Query(ctx, selectSQLStatement)
	if queryErr != nil {
		return fmt.Errorf("failed to query presets: %w", queryErr)
	}
	defer rows.Close()

	freshCache := make(map[string]Preset)
	for rows.Next() {
		var preset Preset
		var descriptionNullString sql.NullString
		_ = rows.Scan(
			&preset.ID,
			&preset.Name,
			&preset.ProcessingOptions,
			&descriptionNullString,
			&preset.CreatedAt,
			&preset.UpdatedAt,
		)
		if descriptionNullString.Valid {
			descString := descriptionNullString.String
			preset.Description = &descString
		}
		freshCache[preset.Name] = preset
	}

	presetManager.rwMutex.Lock()
	presetManager.cache = freshCache
	presetManager.rwMutex.Unlock()

	return nil
}

// ResolveOptions returns the expanded processing options for a named preset.
func (presetManager *PresetManager) ResolveOptions(name string) (string, error) {
	presetManager.rwMutex.RLock()
	preset, found := presetManager.cache[name]
	presetManager.rwMutex.RUnlock()

	if found {
		return preset.ProcessingOptions, nil
	}

	return "", fmt.Errorf("%w: %q", ErrPresetNotFound, name)
}

// List returns all presets ordered by name.
func (presetManager *PresetManager) List(ctx context.Context) ([]Preset, error) {
	const selectSQLStatement = `
		SELECT id, name, processing_options, description, created_at, updated_at
		FROM image.presets
		ORDER BY name ASC;
	`

	rows, queryErr := presetManager.kernel.DB().Query(ctx, selectSQLStatement)
	if queryErr != nil {
		return nil, fmt.Errorf("failed to list presets: %w", queryErr)
	}
	defer rows.Close()

	presets := make([]Preset, 0)
	for rows.Next() {
		var preset Preset
		var descriptionNullString sql.NullString
		_ = rows.Scan(
			&preset.ID,
			&preset.Name,
			&preset.ProcessingOptions,
			&descriptionNullString,
			&preset.CreatedAt,
			&preset.UpdatedAt,
		)
		if descriptionNullString.Valid {
			descString := descriptionNullString.String
			preset.Description = &descString
		}
		presets = append(presets, preset)
	}

	return presets, nil
}

// GetByID queries a single preset by its UUID.
func (presetManager *PresetManager) GetByID(ctx context.Context, presetID uuid.UUID) (*Preset, error) {
	const selectSQLStatement = `
		SELECT id, name, processing_options, description, created_at, updated_at
		FROM image.presets
		WHERE id = $1;
	`

	var preset Preset
	var descriptionNullString sql.NullString
	queryErr := presetManager.kernel.DB().QueryRow(ctx, selectSQLStatement, presetID).Scan(
		&preset.ID,
		&preset.Name,
		&preset.ProcessingOptions,
		&descriptionNullString,
		&preset.CreatedAt,
		&preset.UpdatedAt,
	)
	if queryErr != nil {
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return nil, ErrPresetNotFound
		}
		return nil, queryErr
	}
	if descriptionNullString.Valid {
		descString := descriptionNullString.String
		preset.Description = &descString
	}

	return &preset, nil
}

// Create inserts a new preset and updates the cache.
func (presetManager *PresetManager) Create(ctx context.Context, createPresetInput CreatePresetInput) (*Preset, error) {
	if createPresetInput.Name == "" {
		return nil, errors.New("preset name cannot be empty")
	}
	if createPresetInput.ProcessingOptions == "" {
		return nil, errors.New("processing_options cannot be empty")
	}

	const insertSQLStatement = `
		INSERT INTO image.presets (name, processing_options, description, created_at, updated_at)
		VALUES ($1, $2, $3, clock_timestamp(), clock_timestamp())
		RETURNING id, name, processing_options, description, created_at, updated_at;
	`

	var preset Preset
	var descriptionNullString sql.NullString
	queryErr := presetManager.kernel.DB().QueryRow(
		ctx,
		insertSQLStatement,
		createPresetInput.Name,
		createPresetInput.ProcessingOptions,
		createPresetInput.Description,
	).Scan(
		&preset.ID,
		&preset.Name,
		&preset.ProcessingOptions,
		&descriptionNullString,
		&preset.CreatedAt,
		&preset.UpdatedAt,
	)
	if queryErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrPresetExists, queryErr)
	}
	if descriptionNullString.Valid {
		descString := descriptionNullString.String
		preset.Description = &descString
	}

	presetManager.rwMutex.Lock()
	presetManager.cache[preset.Name] = preset
	presetManager.rwMutex.Unlock()

	return &preset, nil
}

// Update modifies an existing preset.
func (presetManager *PresetManager) Update(ctx context.Context, presetID uuid.UUID, updatePresetInput UpdatePresetInput) (*Preset, error) {
	existingPreset, getErr := presetManager.GetByID(ctx, presetID)
	if getErr != nil {
		return nil, getErr
	}

	newName := existingPreset.Name
	if updatePresetInput.Name != nil && *updatePresetInput.Name != "" {
		newName = *updatePresetInput.Name
	}

	newOptions := existingPreset.ProcessingOptions
	if updatePresetInput.ProcessingOptions != nil && *updatePresetInput.ProcessingOptions != "" {
		newOptions = *updatePresetInput.ProcessingOptions
	}

	newDescription := existingPreset.Description
	if updatePresetInput.Description != nil {
		newDescription = updatePresetInput.Description
	}

	const updateSQLStatement = `
		UPDATE image.presets
		SET name = $1, processing_options = $2, description = $3, updated_at = clock_timestamp()
		WHERE id = $4
		RETURNING id, name, processing_options, description, created_at, updated_at;
	`

	var updatedPreset Preset
	var descriptionNullString sql.NullString
	queryErr := presetManager.kernel.DB().QueryRow(
		ctx,
		updateSQLStatement,
		newName,
		newOptions,
		newDescription,
		presetID,
	).Scan(
		&updatedPreset.ID,
		&updatedPreset.Name,
		&updatedPreset.ProcessingOptions,
		&descriptionNullString,
		&updatedPreset.CreatedAt,
		&updatedPreset.UpdatedAt,
	)
	if queryErr != nil {
		return nil, fmt.Errorf("failed to update preset: %w", queryErr)
	}
	if descriptionNullString.Valid {
		descString := descriptionNullString.String
		updatedPreset.Description = &descString
	}

	presetManager.rwMutex.Lock()
	delete(presetManager.cache, existingPreset.Name)
	presetManager.cache[updatedPreset.Name] = updatedPreset
	presetManager.rwMutex.Unlock()

	return &updatedPreset, nil
}

// Delete removes a preset by ID.
func (presetManager *PresetManager) Delete(ctx context.Context, presetID uuid.UUID) (string, error) {
	existingPreset, getErr := presetManager.GetByID(ctx, presetID)
	if getErr != nil {
		return "", getErr
	}

	const deleteSQLStatement = `
		DELETE FROM image.presets
		WHERE id = $1;
	`

	_, execErr := presetManager.kernel.DB().Exec(ctx, deleteSQLStatement, presetID)
	if execErr != nil {
		return "", fmt.Errorf("failed to delete preset: %w", execErr)
	}

	presetManager.rwMutex.Lock()
	delete(presetManager.cache, existingPreset.Name)
	presetManager.rwMutex.Unlock()

	return existingPreset.Name, nil
}
