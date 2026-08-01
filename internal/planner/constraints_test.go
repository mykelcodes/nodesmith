package planner

import (
	"encoding/json"
	"strings"
	"testing"

	"nodesmith/internal/recipe"
)

// constrainedManifest builds a manifest through the real decoder so these tests
// exercise decoding and manifest validation, not just the planner.
func constrainedManifest(t *testing.T, fieldsJSON string) recipe.Manifest {
	t.Helper()
	raw := `{
		"schemaVersion": 1,
		"id": "constrained",
		"name": "Constrained",
		"category": "tooling",
		"description": "A recipe used to exercise field value constraints in tests.",
		"docsUrl": "https://example.invalid/docs",
		"tags": ["tooling"],
		"icon": "vite",
		"verifiedAt": "2026-07-28",
		"requires": {"node": ">=20.19.0", "packageManagers": ["npm"], "tools": []},
		"fields": ` + fieldsJSON + `,
		"steps": [{
			"id": "scaffold",
			"label": "Scaffold project",
			"bin": "npx",
			"cwd": "parentDir",
			"env": {"CI": "1"},
			"args": ["--yes", "create-thing", "${projectName}"]
		}]
	}`
	manifest, err := recipe.DecodeAndValidate(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("decode constrained manifest: %v", err)
	}
	return manifest
}

func resolveWithAnswers(
	t *testing.T,
	manifest recipe.Manifest,
	answers map[string]any,
) (Plan, error) {
	t.Helper()
	return Resolve(manifest, ScaffoldRequest{
		RecipeID:       manifest.ID,
		ProjectName:    "app",
		ParentDir:      ".",
		PackageManager: "npm",
		InstallDeps:    false,
		Answers:        answers,
	}, &fakeResolver{})
}

func TestResolveEnforcesTextConstraints(t *testing.T) {
	t.Parallel()

	manifest := constrainedManifest(t, `[{
		"id": "scope",
		"label": "Package scope",
		"type": "text",
		"default": "acme",
		"pattern": "^[a-z][a-z0-9-]*$",
		"minLength": 2,
		"maxLength": 12
	}]`)

	tests := []struct {
		name    string
		answer  string
		wantErr string
	}{
		{name: "valid", answer: "my-scope"},
		{name: "too short", answer: "a", wantErr: "at least 2 characters"},
		{name: "too long", answer: "aaaaaaaaaaaaaaa", wantErr: "at most 12 characters"},
		{name: "fails pattern", answer: "Bad Scope", wantErr: "does not match"},
		// The realistic failure this guards: a value the generator reads as a
		// flag rather than as a value.
		{name: "leading dash reads as a flag", answer: "-rf", wantErr: "does not match"},
		{name: "empty string", answer: "", wantErr: "at least 2 characters"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveWithAnswers(t, manifest, map[string]any{"scope": test.answer})
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("Resolve() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Resolve() error = %v, want substring %q", err, test.wantErr)
			}
			// Errors are field-scoped so the form can attach them to an input.
			if err != nil && !strings.Contains(err.Error(), "answers.scope") {
				t.Fatalf("Resolve() error = %v, want it scoped to answers.scope", err)
			}
		})
	}
}

func TestResolveEnforcesNumberBounds(t *testing.T) {
	t.Parallel()

	manifest := constrainedManifest(t, `[{
		"id": "port",
		"label": "Dev server port",
		"type": "number",
		"default": 3000,
		"min": 1024,
		"max": 65535
	}]`)

	if _, err := resolveWithAnswers(t, manifest, map[string]any{
		"port": json.Number("8080"),
	}); err != nil {
		t.Fatalf("Resolve() error = %v, want an in-range port accepted", err)
	}
	for _, test := range []struct {
		value   string
		wantErr string
	}{
		{value: "80", wantErr: "at least 1024"},
		{value: "70000", wantErr: "at most 65535"},
	} {
		_, err := resolveWithAnswers(t, manifest, map[string]any{"port": json.Number(test.value)})
		if err == nil || !strings.Contains(err.Error(), test.wantErr) {
			t.Fatalf("Resolve(port=%s) error = %v, want substring %q", test.value, err, test.wantErr)
		}
	}
}

func TestResolveRequiresAnAnswerForRequiredFields(t *testing.T) {
	t.Parallel()

	manifest := constrainedManifest(t, `[{
		"id": "scope",
		"label": "Package scope",
		"type": "text",
		"default": "",
		"required": true
	}]`)

	// Omitting the answer must not silently fall back to the default, which is
	// the whole point of declaring the field required.
	_, err := resolveWithAnswers(t, manifest, map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "requires an answer") {
		t.Fatalf("Resolve() error = %v, want a required-field error", err)
	}
	if !strings.Contains(err.Error(), "Package scope") {
		t.Fatalf("Resolve() error = %v, want the field label named", err)
	}
	if _, err := resolveWithAnswers(t, manifest, map[string]any{"scope": "acme"}); err != nil {
		t.Fatalf("Resolve() error = %v, want an answered required field accepted", err)
	}
}

// A required field the user never saw must not block the plan.
func TestResolveSkipsRequiredFieldsHiddenByVisibleIf(t *testing.T) {
	t.Parallel()

	manifest := constrainedManifest(t, `[
		{"id": "custom", "label": "Use a custom scope", "type": "boolean", "default": false},
		{
			"id": "scope",
			"label": "Package scope",
			"type": "text",
			"default": "",
			"required": true,
			"visibleIf": "custom == true"
		}
	]`)

	if _, err := resolveWithAnswers(t, manifest, map[string]any{"custom": false}); err != nil {
		t.Fatalf("Resolve() error = %v, want a hidden required field ignored", err)
	}
	_, err := resolveWithAnswers(t, manifest, map[string]any{"custom": true})
	if err == nil || !strings.Contains(err.Error(), "requires an answer") {
		t.Fatalf("Resolve() error = %v, want the visible required field enforced", err)
	}
}

// Constraints are additive and optional, so a manifest that declares none keeps
// behaving exactly as before.
func TestResolveLeavesUnconstrainedFieldsUnchanged(t *testing.T) {
	t.Parallel()

	manifest := constrainedManifest(t, `[{
		"id": "scope",
		"label": "Package scope",
		"type": "text",
		"default": "acme"
	}]`)

	for _, answer := range []string{"", "-rf", "Anything At All", strings.Repeat("x", 4096)} {
		if _, err := resolveWithAnswers(t, manifest, map[string]any{"scope": answer}); err != nil {
			t.Fatalf("Resolve(scope=%q) error = %v, want no constraint applied", answer, err)
		}
	}
}
