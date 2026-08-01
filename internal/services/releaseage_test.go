package services

import (
	"strings"
	"testing"

	"nodesmith/internal/recipe"
)

func testMinutes(value int) *int {
	return &value
}

func cooldownManifest(id string, minutes *int) recipe.Manifest {
	return recipe.Manifest{ID: id, MinimumReleaseAge: minutes}
}

func TestResolveMinimumReleaseAgePrecedence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		requestMinutes *int
		settings       Settings
		manifest       recipe.Manifest
		wantMinute     *int
		wantSource     string
	}{
		{
			name:       "nothing configured anywhere",
			settings:   Settings{},
			manifest:   cooldownManifest("vite", nil),
			wantSource: ReleaseAgeSourceUnset,
		},
		{
			name:       "global applies when the recipe is silent",
			settings:   Settings{MinimumReleaseAge: testMinutes(1440)},
			manifest:   cooldownManifest("vite", nil),
			wantMinute: testMinutes(1440),
			wantSource: ReleaseAgeSourceGlobal,
		},
		{
			name:       "recipe beats global",
			settings:   Settings{MinimumReleaseAge: testMinutes(1440)},
			manifest:   cooldownManifest("vite", testMinutes(4320)),
			wantMinute: testMinutes(4320),
			wantSource: ReleaseAgeSourceRecipe,
		},
		{
			name:       "recipe zero beats a non-zero global",
			settings:   Settings{MinimumReleaseAge: testMinutes(1440)},
			manifest:   cooldownManifest("vite", testMinutes(0)),
			wantMinute: testMinutes(0),
			wantSource: ReleaseAgeSourceRecipe,
		},
		{
			name:           "catalogue configuration beats the recipe",
			requestMinutes: testMinutes(10080),
			settings:       Settings{MinimumReleaseAge: testMinutes(1440)},
			manifest:       cooldownManifest("vite", testMinutes(4320)),
			wantMinute:     testMinutes(10080),
			wantSource:     ReleaseAgeSourceRequest,
		},
		{
			name:           "configured zero disables a recipe cooldown",
			requestMinutes: testMinutes(0),
			settings:       Settings{MinimumReleaseAge: testMinutes(1440)},
			manifest:       cooldownManifest("vite", testMinutes(4320)),
			wantMinute:     testMinutes(0),
			wantSource:     ReleaseAgeSourceRequest,
		},
	}

	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := resolveMinimumReleaseAge(test.requestMinutes, test.settings, test.manifest)
			if got.Source != test.wantSource {
				t.Fatalf("source = %q, want %q", got.Source, test.wantSource)
			}
			switch {
			case test.wantMinute == nil && got.Minutes != nil:
				t.Fatalf("minutes = %d, want unset", *got.Minutes)
			case test.wantMinute != nil && got.Minutes == nil:
				t.Fatalf("minutes = unset, want %d", *test.wantMinute)
			case test.wantMinute != nil && *got.Minutes != *test.wantMinute:
				t.Fatalf("minutes = %d, want %d", *got.Minutes, *test.wantMinute)
			}
		})
	}
}

func TestResolveMinimumReleaseAgeDoesNotAliasStoredState(t *testing.T) {
	t.Parallel()

	manifest := cooldownManifest("vite", testMinutes(4320))
	resolved := resolveMinimumReleaseAge(nil, Settings{}, manifest)
	*resolved.Minutes = 1
	if *manifest.MinimumReleaseAge != 4320 {
		t.Fatalf("manifest cooldown mutated through the resolution: %d", *manifest.MinimumReleaseAge)
	}

	configured := testMinutes(10080)
	resolved = resolveMinimumReleaseAge(configured, Settings{}, manifest)
	*resolved.Minutes = 1
	if *configured != 10080 {
		t.Fatalf("request cooldown mutated through the resolution: %d", *configured)
	}
}

func TestSaveSettingsRejectsOutOfRangeCooldowns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		mutate   func(*Settings)
		wantPart string
	}{
		{
			name:     "negative global",
			mutate:   func(s *Settings) { s.MinimumReleaseAge = testMinutes(-5) },
			wantPart: "minimum release age: must not be negative",
		},
		{
			name:     "global beyond a year",
			mutate:   func(s *Settings) { s.MinimumReleaseAge = testMinutes(recipe.MaxMinimumReleaseAge + 1) },
			wantPart: "minimum release age: must be at most",
		},
	}

	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service, err := NewStoreService(nil, t.TempDir(), t.TempDir())
			if err != nil {
				t.Fatalf("NewStoreService(nil, ) error = %v", err)
			}
			settings, err := service.GetSettings()
			if err != nil {
				t.Fatalf("GetSettings() error = %v", err)
			}
			test.mutate(&settings)
			err = service.SaveSettings(settings)
			if err == nil || !strings.Contains(err.Error(), test.wantPart) {
				t.Fatalf("SaveSettings() error = %v, want substring %q", err, test.wantPart)
			}
		})
	}
}

func TestPresetRejectsOutOfRangeCooldown(t *testing.T) {
	t.Parallel()

	service, err := NewStoreService(nil, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("NewStoreService(nil, ) error = %v", err)
	}
	err = service.SavePreset(Preset{
		Name: "Invalid cooldown",
		Request: ScaffoldRequest{
			RecipeID:          "vite",
			MinimumReleaseAge: testMinutes(-1),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "must not be negative") {
		t.Fatalf("SavePreset() error = %v, want invalid cooldown", err)
	}
}

func TestPlanRejectsOutOfRangeConfiguredCooldown(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)
	_, err := service.Plan(ScaffoldRequest{
		RecipeID:          "service-test",
		ProjectName:       "demo",
		ParentDir:         t.TempDir(),
		PackageManager:    "npm",
		MinimumReleaseAge: testMinutes(recipe.MaxMinimumReleaseAge + 1),
		Answers:           map[string]any{},
	})
	if err == nil || !strings.Contains(err.Error(), "must be at most") {
		t.Fatalf("Plan() error = %v, want invalid configured cooldown", err)
	}
}

func TestPlanAppliesTheGlobalAndConfiguredCooldown(t *testing.T) {
	service, storeService := newScaffoldServiceTestHarness(t)
	request := ScaffoldRequest{
		RecipeID:       "service-test",
		ProjectName:    "demo",
		ParentDir:      t.TempDir(),
		PackageManager: "npm",
		Answers:        map[string]any{},
	}

	plan, err := service.Plan(request)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if _, present := plan.Steps[0].Env["npm_config_min_release_age"]; present {
		t.Fatal("plan carries a cooldown before one is configured")
	}

	settings, err := storeService.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	settings.MinimumReleaseAge = testMinutes(4320)
	if err := storeService.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}
	plan, err = service.Plan(request)
	if err != nil {
		t.Fatalf("Plan() after the global cooldown error = %v", err)
	}
	if got := plan.Steps[0].Env["npm_config_min_release_age"]; got != "3" {
		t.Fatalf("npm_config_min_release_age = %q, want %q from the global setting", got, "3")
	}

	// A value selected during catalogue configuration must win over the global
	// value and remain explicit when it is zero.
	request.MinimumReleaseAge = testMinutes(0)
	plan, err = service.Plan(request)
	if err != nil {
		t.Fatalf("Plan() after the configured cooldown error = %v", err)
	}
	if got := plan.Steps[0].Env["npm_config_min_release_age"]; got != "0" {
		t.Fatalf("npm_config_min_release_age = %q, want %q from the configured request", got, "0")
	}
}

func TestStartRejectsAPlanReviewedBeforeTheCooldownChanged(t *testing.T) {
	service, storeService := newScaffoldServiceTestHarness(t)
	request := ScaffoldRequest{
		RecipeID:       "service-test",
		ProjectName:    "demo",
		ParentDir:      t.TempDir(),
		PackageManager: "npm",
		Answers:        map[string]any{},
	}
	if _, err := service.Plan(request); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	settings, err := storeService.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	settings.MinimumReleaseAge = testMinutes(1440)
	if err := storeService.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	if _, err := service.Start(request); err == nil ||
		!strings.Contains(err.Error(), "changed after review") {
		t.Fatalf("Start() after a cooldown change error = %v, want a re-review", err)
	}
}

func TestSaveSettingsRoundTripsTheGlobalCooldownAndClearsLegacyOverrides(t *testing.T) {
	t.Parallel()

	service, err := NewStoreService(nil, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("NewStoreService(nil, ) error = %v", err)
	}
	settings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if settings.MinimumReleaseAge != nil {
		t.Fatalf("default global cooldown = %d, want unset", *settings.MinimumReleaseAge)
	}

	settings.MinimumReleaseAge = testMinutes(1440)
	// Version-one settings can contain these values. Saving migrates them out
	// because catalogue configuration now owns recipe-specific choices.
	settings.RecipeMinimumReleaseAge = map[string]int{"vite": 0, "astro": 10080}
	if err := service.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}
	reloaded, err := service.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() after save error = %v", err)
	}
	if reloaded.MinimumReleaseAge == nil || *reloaded.MinimumReleaseAge != 1440 {
		t.Fatalf("global cooldown = %v, want 1440", reloaded.MinimumReleaseAge)
	}
	if len(reloaded.RecipeMinimumReleaseAge) != 0 {
		t.Fatalf("legacy recipe overrides = %#v, want cleared", reloaded.RecipeMinimumReleaseAge)
	}

	// Clearing the global default returns settings to "nothing configured".
	reloaded.MinimumReleaseAge = nil
	if err := service.SaveSettings(reloaded); err != nil {
		t.Fatalf("SaveSettings(cleared) error = %v", err)
	}
	cleared, err := service.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() after clearing error = %v", err)
	}
	if cleared.MinimumReleaseAge != nil || len(cleared.RecipeMinimumReleaseAge) != 0 {
		t.Fatalf("cleared settings = %#v, want no cooldown configuration", cleared)
	}
}
