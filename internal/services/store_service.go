package services

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"nodesmith/internal/project"
	jsonstore "nodesmith/internal/store"
)

const storeSchemaVersion = 1

// StoreService persists settings, presets, and project history as atomic JSON.
type StoreService struct {
	settings *jsonstore.Store[Settings]
	presets  *jsonstore.Store[[]Preset]
	history  *jsonstore.Store[[]HistoryEntry]
	now      func() time.Time
}

// NewStoreService constructs the three local JSON stores.
func NewStoreService(configDir string, defaultParentDir string) (*StoreService, error) {
	if strings.TrimSpace(configDir) == "" {
		return nil, errors.New("configure local storage: config directory is empty")
	}
	settings, err := jsonstore.New(
		filepath.Join(configDir, "settings.json"),
		storeSchemaVersion,
		Settings{
			DefaultParentDir: defaultParentDir,
			Editor:           "code",
			Theme:            "dark",
			OpenAfterCreate:  true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("configure settings store: %w", err)
	}
	presets, err := jsonstore.New(
		filepath.Join(configDir, "presets.json"),
		storeSchemaVersion,
		[]Preset{},
	)
	if err != nil {
		return nil, fmt.Errorf("configure presets store: %w", err)
	}
	history, err := jsonstore.New(
		filepath.Join(configDir, "history.json"),
		storeSchemaVersion,
		[]HistoryEntry{},
	)
	if err != nil {
		return nil, fmt.Errorf("configure history store: %w", err)
	}
	return &StoreService{
		settings: settings,
		presets:  presets,
		history:  history,
		now:      time.Now,
	}, nil
}

// GetSettings returns local preferences or their first-run defaults.
func (service *StoreService) GetSettings() (Settings, error) {
	settings, err := service.settings.Load()
	if err != nil {
		return Settings{}, fmt.Errorf("load settings: %w", err)
	}
	return settings, nil
}

// SaveSettings atomically replaces local preferences.
func (service *StoreService) SaveSettings(settings Settings) error {
	switch settings.Theme {
	case "dark", "light", "system":
	default:
		return fmt.Errorf("save settings: theme %q is not supported", settings.Theme)
	}
	if err := project.ValidateEditor(settings.Editor); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	if strings.IndexByte(settings.PathOverride, 0) >= 0 {
		return errors.New("save settings: PATH override contains a NUL byte")
	}
	if err := validateMinimumReleaseAgeSettings(settings); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	// Per-recipe choices moved to scaffold requests and presets. Clearing the
	// legacy field migrates version-one settings the next time they are saved.
	settings.RecipeMinimumReleaseAge = nil
	if err := service.settings.Save(settings); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

// ListPresets returns presets ordered by most-recent update, then name.
func (service *StoreService) ListPresets() ([]Preset, error) {
	presets, err := service.presets.Load()
	if err != nil {
		return nil, fmt.Errorf("load presets: %w", err)
	}
	sort.SliceStable(presets, func(left, right int) bool {
		if presets[left].UpdatedAt.Equal(presets[right].UpdatedAt) {
			return strings.ToLower(presets[left].Name) < strings.ToLower(presets[right].Name)
		}
		return presets[left].UpdatedAt.After(presets[right].UpdatedAt)
	})
	return cloneSlice(presets), nil
}

// SavePreset creates or updates a named preset.
func (service *StoreService) SavePreset(preset Preset) error {
	preset.Name = strings.TrimSpace(preset.Name)
	if preset.Name == "" {
		return errors.New("save preset: name is required")
	}
	if preset.Request.RecipeID == "" {
		return errors.New("save preset: recipe is required")
	}
	if err := validateMinimumReleaseAgeRequest(preset.Request); err != nil {
		return fmt.Errorf("save preset: %w", err)
	}
	now := service.now().UTC()
	if preset.ID == "" {
		id, err := randomID()
		if err != nil {
			return fmt.Errorf("save preset: %w", err)
		}
		preset.ID = id
	}
	if preset.CreatedAt.IsZero() {
		preset.CreatedAt = now
	}
	preset.UpdatedAt = now
	if preset.Request.Answers == nil {
		preset.Request.Answers = map[string]any{}
	}

	if err := service.presets.Update(func(presets *[]Preset) error {
		for index := range *presets {
			if (*presets)[index].ID == preset.ID {
				(*presets)[index] = preset
				return nil
			}
		}
		*presets = append(*presets, preset)
		return nil
	}); err != nil {
		return fmt.Errorf("save preset: %w", err)
	}
	return nil
}

// DeletePreset removes a preset by id.
func (service *StoreService) DeletePreset(id string) error {
	if id == "" {
		return errors.New("delete preset: id is required")
	}
	found := false
	if err := service.presets.Update(func(presets *[]Preset) error {
		for index := range *presets {
			if (*presets)[index].ID != id {
				continue
			}
			*presets = slices.Delete(*presets, index, index+1)
			found = true
			break
		}
		return nil
	}); err != nil {
		return fmt.Errorf("delete preset: %w", err)
	}
	if !found {
		return fmt.Errorf("delete preset: preset %q was not found", id)
	}
	return nil
}

// ListHistory returns newest entries first. A non-positive limit returns all.
func (service *StoreService) ListHistory(limit int) ([]HistoryEntry, error) {
	history, err := service.history.Load()
	if err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}
	sort.SliceStable(history, func(left, right int) bool {
		return history[left].CreatedAt.After(history[right].CreatedAt)
	})
	if limit > 0 && len(history) > limit {
		history = history[:limit]
	}
	return cloneSlice(history), nil
}

// ClearHistory removes all project history.
func (service *StoreService) ClearHistory() error {
	if err := service.history.Save([]HistoryEntry{}); err != nil {
		return fmt.Errorf("clear history: %w", err)
	}
	return nil
}

func (service *StoreService) recordHistory(entry HistoryEntry) error {
	if entry.ID == "" {
		return errors.New("record history: id is required")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = service.now().UTC()
	}
	if err := service.history.Update(func(history *[]HistoryEntry) error {
		for index := range *history {
			if (*history)[index].ID == entry.ID {
				(*history)[index] = entry
				return nil
			}
		}
		*history = append(*history, entry)
		return nil
	}); err != nil {
		return fmt.Errorf("record history: %w", err)
	}
	return nil
}

func (service *StoreService) updateHistory(
	id string,
	state string,
	durationMS int64,
	message string,
) error {
	found := false
	if err := service.history.Update(func(history *[]HistoryEntry) error {
		for index := range *history {
			if (*history)[index].ID != id {
				continue
			}
			(*history)[index].State = state
			(*history)[index].DurationMS = durationMS
			(*history)[index].Error = message
			found = true
			break
		}
		return nil
	}); err != nil {
		return fmt.Errorf("update history: %w", err)
	}
	if !found {
		return fmt.Errorf("update history: entry %q was not found", id)
	}
	return nil
}
