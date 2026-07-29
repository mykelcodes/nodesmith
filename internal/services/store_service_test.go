package services

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreServiceSettingsPresetsAndHistory(t *testing.T) {
	service, err := NewStoreService(t.TempDir(), filepath.Join(t.TempDir(), "projects"))
	if err != nil {
		t.Fatalf("NewStoreService() error = %v", err)
	}
	fixedNow := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	settings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if settings.Editor != "code" || settings.Theme != "dark" {
		t.Fatalf("default settings = %#v", settings)
	}
	settings.Editor = "zed"
	settings.Theme = "system"
	settings.PathOverride = "/custom/bin"
	if err := service.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}
	reloadedSettings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() after save error = %v", err)
	}
	if reloadedSettings.PathOverride != "/custom/bin" {
		t.Fatalf("PathOverride = %q, want /custom/bin", reloadedSettings.PathOverride)
	}
	reloadedSettings.Editor = filepath.Join(t.TempDir(), "custom-editor")
	if err := service.SaveSettings(reloadedSettings); err != nil {
		t.Fatalf("SaveSettings(custom editor path) error = %v", err)
	}

	preset := Preset{
		Name: "Svelte default",
		Request: ScaffoldRequest{
			RecipeID:          "sveltekit",
			ProjectName:       "my-app",
			ParentDir:         "/projects",
			PackageManager:    "pnpm",
			InstallDeps:       true,
			GitInit:           true,
			MinimumReleaseAge: testMinutes(4320),
			Answers:           map[string]any{"typescript": true},
		},
	}
	if err := service.SavePreset(preset); err != nil {
		t.Fatalf("SavePreset() error = %v", err)
	}
	presets, err := service.ListPresets()
	if err != nil {
		t.Fatalf("ListPresets() error = %v", err)
	}
	if len(presets) != 1 || presets[0].ID == "" || presets[0].CreatedAt != fixedNow {
		t.Fatalf("presets = %#v", presets)
	}
	if presets[0].Request.MinimumReleaseAge == nil ||
		*presets[0].Request.MinimumReleaseAge != 4320 {
		t.Fatalf(
			"preset minimum release age = %v, want 4320",
			presets[0].Request.MinimumReleaseAge,
		)
	}
	if err := service.DeletePreset(presets[0].ID); err != nil {
		t.Fatalf("DeletePreset() error = %v", err)
	}
	presets, err = service.ListPresets()
	if err != nil {
		t.Fatalf("ListPresets() after delete error = %v", err)
	}
	if len(presets) != 0 {
		t.Fatalf("len(presets) = %d, want 0", len(presets))
	}

	entry := HistoryEntry{
		ID:         "job-1",
		RecipeID:   "sveltekit",
		RecipeName: "SvelteKit",
		ProjectDir: "/projects/my-app",
		State:      "running",
	}
	if err := service.recordHistory(entry); err != nil {
		t.Fatalf("recordHistory() error = %v", err)
	}
	if err := service.updateHistory("job-1", "success", 1250, ""); err != nil {
		t.Fatalf("updateHistory() error = %v", err)
	}
	history, err := service.ListHistory(10)
	if err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}
	if len(history) != 1 || history[0].State != "success" || history[0].DurationMS != 1250 {
		t.Fatalf("history = %#v", history)
	}
	if err := service.ClearHistory(); err != nil {
		t.Fatalf("ClearHistory() error = %v", err)
	}
	history, err = service.ListHistory(10)
	if err != nil {
		t.Fatalf("ListHistory() after clear error = %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("len(history) = %d, want 0", len(history))
	}
}

func TestStoreServiceRejectsInvalidSettingsAndMissingRecords(t *testing.T) {
	service, err := NewStoreService(t.TempDir(), "")
	if err != nil {
		t.Fatalf("NewStoreService() error = %v", err)
	}
	if err := service.SaveSettings(Settings{Theme: "sepia", Editor: "code"}); err == nil {
		t.Fatal("SaveSettings(invalid theme) error = nil")
	}
	if err := service.SaveSettings(Settings{Theme: "dark", Editor: "vim"}); err == nil {
		t.Fatal("SaveSettings(invalid editor) error = nil")
	}
	if err := service.SavePreset(Preset{}); err == nil {
		t.Fatal("SavePreset(empty) error = nil")
	}
	if err := service.DeletePreset("missing"); err == nil {
		t.Fatal("DeletePreset(missing) error = nil")
	}
	if err := service.updateHistory("missing", "failed", 0, "error"); err == nil {
		t.Fatal("updateHistory(missing) error = nil")
	}
}
