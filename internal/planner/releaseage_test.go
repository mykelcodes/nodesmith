package planner

import (
	"reflect"
	"strings"
	"testing"

	"nodesmith/internal/recipe"
)

func minutes(value int) *int {
	return &value
}

// releaseAgeManifest builds a two-step recipe: a scaffold step run by npx and a
// step run by whichever package manager the request selects.
func releaseAgeManifest(packageManagers ...string) recipe.Manifest {
	return recipe.Manifest{
		SchemaVersion: 1,
		ID:            "cooldown",
		Name:          "Cooldown",
		Category:      "tooling",
		Description:   "Release-age planner test recipe.",
		DocsURL:       "https://example.com/cooldown",
		Tags:          []string{"test"},
		Icon:          "cooldown",
		VerifiedAt:    "2026-07-28",
		Requires: recipe.Requirements{
			Node:            ">=20.0.0",
			PackageManagers: packageManagers,
			Tools:           []string{},
		},
		Fields: []recipe.Field{},
		Steps: []recipe.Step{
			{
				ID:    "scaffold",
				Label: "Scaffold",
				Bin:   "npx",
				CWD:   "parentDir",
				Env:   map[string]string{"CI": "1"},
				Args:  []recipe.ArgNode{recipe.Literal("create-thing")},
			},
			{
				ID:    "install",
				Label: "Install",
				Bin:   "${packageManager}",
				CWD:   "projectDir",
				Env:   map[string]string{"CI": "1"},
				Args:  []recipe.ArgNode{recipe.Literal("install")},
			},
			{
				ID:    "git-init",
				Label: "Initialise repository",
				Bin:   "git",
				CWD:   "projectDir",
				Env:   map[string]string{"CI": "1"},
				Args:  []recipe.ArgNode{recipe.Literal("init")},
			},
		},
	}
}

func releaseAgePlan(t *testing.T, packageManager string, cooldown *int) Plan {
	t.Helper()
	manifest := releaseAgeManifest("npm", "pnpm", "yarn", "bun")
	plan, err := Resolve(manifest, ScaffoldRequest{
		RecipeID:          manifest.ID,
		ProjectName:       "app",
		ParentDir:         ".",
		PackageManager:    packageManager,
		InstallDeps:       true,
		GitInit:           true,
		MinimumReleaseAge: cooldown,
		Answers:           map[string]any{},
	}, &fakeResolver{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	return plan
}

func stepByID(t *testing.T, plan Plan, id string) PlanStep {
	t.Helper()
	for _, step := range plan.Steps {
		if step.ID == id {
			return step
		}
	}
	t.Fatalf("plan has no step %q", id)
	return PlanStep{}
}

func releaseAgeConfigStep(t *testing.T, plan Plan) PlanStep {
	t.Helper()
	step := stepByID(t, plan, "minimum-release-age-config")
	if step.Kind != StepKindProjectConfig || step.Config == nil {
		t.Fatalf("release-age config step = %#v, want project configuration", step)
	}
	return step
}

func TestResolveOmitsReleaseAgeEnvironmentWhenUnset(t *testing.T) {
	t.Parallel()

	plan := releaseAgePlan(t, "pnpm", nil)
	for _, step := range plan.Steps {
		for key := range step.Env {
			if strings.Contains(strings.ToLower(key), "release_age") ||
				strings.Contains(strings.ToLower(key), "age_gate") {
				t.Fatalf("step %q env contains %q, want no cooldown variables", step.ID, key)
			}
		}
	}
	if len(plan.Warnings) != 1 || plan.Warnings[0] != pnpmBlockedBuildsWarning {
		t.Fatalf("warnings = %#v, want only pnpm build-approval warning", plan.Warnings)
	}
	for _, step := range plan.Steps {
		if step.Config != nil {
			t.Fatalf("step %q unexpectedly writes project configuration", step.ID)
		}
	}
}

func TestResolveInjectsPnpmCooldownAndNpmDayFallback(t *testing.T) {
	t.Parallel()

	plan := releaseAgePlan(t, "pnpm", minutes(4320))

	// npx runs npm, and pnpm is the selected manager the generator delegates
	// to, so the step carries both. The npm-style pnpm 10 fallback is dropped
	// because npm warns about config keys it does not know.
	scaffold := stepByID(t, plan, "scaffold")
	want := map[string]string{
		"CI":                              "true",
		"npm_config_min_release_age":      "3",
		"pnpm_config_minimum_release_age": "4320",
	}
	if !reflect.DeepEqual(scaffold.Env, want) {
		t.Fatalf("scaffold env = %#v, want %#v", scaffold.Env, want)
	}

	install := stepByID(t, plan, "install")
	wantInstall := map[string]string{
		"CI":                              "true",
		"pnpm_config_minimum_release_age": "4320",
		"npm_config_minimum_release_age":  "4320",
	}
	if !reflect.DeepEqual(install.Env, wantInstall) {
		t.Fatalf("install env = %#v, want %#v", install.Env, wantInstall)
	}

	config := releaseAgeConfigStep(t, plan)
	if config.Config.Path != "pnpm-workspace.yaml" ||
		config.Config.Key != "minimumReleaseAge" ||
		config.Config.Value != "4320" {
		t.Fatalf("pnpm config = %#v, want minimumReleaseAge 4320", config.Config)
	}
	if plan.Steps[1].ID != config.ID || plan.Steps[2].ID != "install" {
		t.Fatalf("step order = %#v, want config immediately before install", plan.Steps)
	}
}

func TestResolveLeavesNonPackageManagerStepsAlone(t *testing.T) {
	t.Parallel()

	plan := releaseAgePlan(t, "yarn", minutes(1440))
	git := stepByID(t, plan, "git-init")
	// git never reaches the registry itself, but yarn is still the selected
	// manager, so only the yarn gate rides along.
	want := map[string]string{"CI": "true", "YARN_NPM_MINIMAL_AGE_GATE": "1440"}
	if !reflect.DeepEqual(git.Env, want) {
		t.Fatalf("git env = %#v, want %#v", git.Env, want)
	}
	config := releaseAgeConfigStep(t, plan)
	if config.Config.Path != ".yarnrc.yml" || config.Config.Value != `"1d"` {
		t.Fatalf("yarn config = %#v, want a one-day npmMinimalAgeGate", config.Config)
	}
}

func TestResolveRoundsNpmCooldownUpToWholeDays(t *testing.T) {
	t.Parallel()

	plan := releaseAgePlan(t, "npm", minutes(90))
	scaffold := stepByID(t, plan, "scaffold")
	if got := scaffold.Env["npm_config_min_release_age"]; got != "1" {
		t.Fatalf("npm_config_min_release_age = %q, want %q", got, "1")
	}
	if len(plan.Warnings) != 1 || !strings.Contains(plan.Warnings[0], "whole days") {
		t.Fatalf("warnings = %#v, want a single rounding warning", plan.Warnings)
	}
	config := releaseAgeConfigStep(t, plan)
	if config.Config.Path != ".npmrc" || config.Config.Value != "1" {
		t.Fatalf("npm config = %#v, want min-release-age=1", config.Config)
	}
}

func TestResolveWritesBunfigBeforeInstall(t *testing.T) {
	t.Parallel()

	plan := releaseAgePlan(t, "bun", minutes(1440))
	install := stepByID(t, plan, "install")
	if len(install.Env) != 1 || install.Env["CI"] != "true" {
		t.Fatalf("bun install env = %#v, want only CI", install.Env)
	}
	// The npx step still gets npm's gate even though bun cannot take one.
	scaffold := stepByID(t, plan, "scaffold")
	if got := scaffold.Env["npm_config_min_release_age"]; got != "1" {
		t.Fatalf("npm_config_min_release_age = %q, want %q", got, "1")
	}
	config := releaseAgeConfigStep(t, plan)
	if config.Config.Path != "bunfig.toml" ||
		config.Config.Section != "install" ||
		config.Config.Value != "86400" {
		t.Fatalf("bun config = %#v, want install.minimumReleaseAge=86400", config.Config)
	}
	if plan.Steps[1].ID != config.ID || plan.Steps[2].ID != "install" {
		t.Fatalf("step order = %#v, want bunfig before install", plan.Steps)
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none once bunfig is written", plan.Warnings)
	}
}

func TestResolveInjectsExplicitZeroToDisableDefaultCooldowns(t *testing.T) {
	t.Parallel()

	// pnpm 11 applies a one-day cooldown by default, so an explicit zero has to
	// travel to the process rather than being dropped as "no value".
	plan := releaseAgePlan(t, "pnpm", minutes(0))
	install := stepByID(t, plan, "install")
	if got := install.Env["pnpm_config_minimum_release_age"]; got != "0" {
		t.Fatalf("pnpm_config_minimum_release_age = %q, want %q", got, "0")
	}
	if got := stepByID(t, plan, "scaffold").Env["npm_config_min_release_age"]; got != "0" {
		t.Fatalf("npm_config_min_release_age = %q, want %q", got, "0")
	}
	if got := releaseAgeConfigStep(t, plan).Config.Value; got != "0" {
		t.Fatalf("pnpm-workspace minimumReleaseAge = %q, want %q", got, "0")
	}
	if len(plan.Warnings) != 1 || plan.Warnings[0] != pnpmBlockedBuildsWarning {
		t.Fatalf("warnings = %#v, want only pnpm build-approval warning", plan.Warnings)
	}
}

func TestResolveCooldownChangesThePlanHash(t *testing.T) {
	t.Parallel()

	without := releaseAgePlan(t, "pnpm", nil)
	with := releaseAgePlan(t, "pnpm", minutes(1440))
	if without.Hash == with.Hash {
		t.Fatal("plan hash ignored the minimum release age; a settings change would slip past review")
	}
}

func TestResolveCooldownOverridesRecipeSuppliedEnvironment(t *testing.T) {
	t.Parallel()

	manifest := releaseAgeManifest("pnpm")
	manifest.Steps[1].Env["pnpm_config_minimum_release_age"] = "0"
	plan, err := Resolve(manifest, ScaffoldRequest{
		RecipeID:          manifest.ID,
		ProjectName:       "app",
		ParentDir:         ".",
		PackageManager:    "pnpm",
		InstallDeps:       true,
		GitInit:           true,
		MinimumReleaseAge: minutes(1440),
		Answers:           map[string]any{},
	}, &fakeResolver{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := stepByID(t, plan, "install").Env["pnpm_config_minimum_release_age"]; got != "1440" {
		t.Fatalf("pnpm_config_minimum_release_age = %q, want the resolved %q", got, "1440")
	}
}

func TestCeilDays(t *testing.T) {
	t.Parallel()

	cases := []struct {
		minutes int
		want    int
	}{
		{minutes: 0, want: 0},
		{minutes: 1, want: 1},
		{minutes: 1440, want: 1},
		{minutes: 1441, want: 2},
		{minutes: 4320, want: 3},
	}
	for _, testCase := range cases {
		if got := ceilDays(testCase.minutes); got != testCase.want {
			t.Fatalf("ceilDays(%d) = %d, want %d", testCase.minutes, got, testCase.want)
		}
	}
}
