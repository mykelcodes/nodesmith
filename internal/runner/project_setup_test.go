package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"nodesmith/internal/planner"
)

func TestWriteProjectSetupConfiguresSelectedNodeTools(t *testing.T) {
	root := t.TempDir()
	writeSetupFixture(t, root, "package.json", `{
  "name": "demo",
  "scripts": { "test": "vitest" },
  "dependencies": { "left-pad": "1.3.0", "eslint": "old" },
  "devDependencies": { "prettier": "old" }
}
`)
	writeSetupFixture(t, root, "eslint.config.js", "export default []\n")
	step := planner.PlanStep{
		Kind: planner.StepKindProjectSetup,
		Dir:  root,
		Setup: &planner.ProjectSetup{
			Linting:    "oxlint",
			Formatting: "oxfmt",
		},
	}

	if err := writeProjectSetup(step); err != nil {
		t.Fatalf("writeProjectSetup() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "eslint.config.js")); !os.IsNotExist(err) {
		t.Fatalf("old ESLint config still exists, err = %v", err)
	}
	for _, name := range []string{".oxlintrc.json", ".oxfmtrc.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}

	packageJSON := readPackageFixture(t, root)
	scripts := packageJSON["scripts"].(map[string]any)
	if scripts["test"] != "vitest" || scripts["lint"] != "oxlint" || scripts["format:check"] != "oxfmt --check" {
		t.Fatalf("scripts = %#v", scripts)
	}
	dependencies := packageJSON["dependencies"].(map[string]any)
	if dependencies["left-pad"] != "1.3.0" {
		t.Fatalf("dependencies = %#v", dependencies)
	}
	if _, exists := dependencies["eslint"]; exists {
		t.Fatalf("stale ESLint dependency was not removed")
	}
	devDependencies := packageJSON["devDependencies"].(map[string]any)
	if devDependencies["oxlint"] != "^1.76.0" || devDependencies["oxfmt"] != "^0.61.0" {
		t.Fatalf("devDependencies = %#v", devDependencies)
	}
}

func TestWriteProjectSetupAddsFrameworkToolingPlugins(t *testing.T) {
	tests := []struct {
		name         string
		setup        planner.ProjectSetup
		wantPackages []string
		wantConfig   string
	}{
		{
			name:         "Astro",
			setup:        planner.ProjectSetup{RecipeID: "astro", Linting: "eslint", Formatting: "prettier"},
			wantPackages: []string{"eslint-plugin-astro", "prettier-plugin-astro"},
			wantConfig:   "astro.configs['flat/recommended']",
		},
		{
			name:         "Svelte template",
			setup:        planner.ProjectSetup{RecipeID: "tauri", Template: "svelte-ts", Linting: "eslint", Formatting: "prettier"},
			wantPackages: []string{"eslint-plugin-svelte", "prettier-plugin-svelte"},
			wantConfig:   "svelte.configs.recommended",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeSetupFixture(t, root, "package.json", `{"name":"demo"}`)
			if err := writeProjectSetup(planner.PlanStep{Dir: root, Setup: &test.setup}); err != nil {
				t.Fatal(err)
			}
			packageJSON := readPackageFixture(t, root)
			devDependencies := packageJSON["devDependencies"].(map[string]any)
			for _, name := range test.wantPackages {
				if _, exists := devDependencies[name]; !exists {
					t.Fatalf("devDependencies = %#v, missing %s", devDependencies, name)
				}
			}
			config, err := os.ReadFile(filepath.Join(root, "eslint.config.mjs"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(config), test.wantConfig) {
				t.Fatalf("ESLint config = %q, want %q", config, test.wantConfig)
			}
		})
	}
}

func TestWriteProjectSetupPreservesGeneratedESLintAndPrettierConfigs(t *testing.T) {
	root := t.TempDir()
	writeSetupFixture(t, root, "package.json", `{"name":"demo"}`)
	writeSetupFixture(t, root, "eslint.config.mjs", "export default ['framework-eslint'];\n")
	writeSetupFixture(t, root, ".prettierrc", `{"plugins":["framework-prettier"]}`)

	err := writeProjectSetup(planner.PlanStep{Dir: root, Setup: &planner.ProjectSetup{
		RecipeID: "next", Linting: "eslint", Formatting: "prettier",
	}})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"eslint.config.mjs": "export default ['framework-eslint'];\n",
		".prettierrc":       `{"plugins":["framework-prettier"]}`,
	} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != want {
			t.Fatalf("%s = %q, want preserved %q", name, content, want)
		}
	}
}

func TestWriteProjectSetupConfiguresExpoStyling(t *testing.T) {
	tests := []struct {
		name        string
		styling     string
		template    string
		entry       string
		wantImport  string
		wantFiles   []string
		wantPackage string
	}{
		{
			name: "uniwind default", styling: "uniwind", template: "default",
			entry: "src/app/_layout.tsx", wantImport: "import '../../global.css';",
			wantFiles: []string{"global.css", "metro.config.js"}, wantPackage: "uniwind",
		},
		{
			name: "nativewind blank typescript", styling: "nativewind", template: "blank-typescript",
			entry: "App.tsx", wantImport: "import './global.css';",
			wantFiles: []string{"global.css", "metro.config.js", "postcss.config.mjs"}, wantPackage: "nativewind",
		},
		{
			name: "unistyles", styling: "unistyles", template: "tabs",
			wantFiles: []string{"babel.config.js"}, wantPackage: "react-native-unistyles",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeSetupFixture(t, root, "package.json", `{"name":"demo"}`)
			if test.entry != "" {
				writeSetupFixture(t, root, test.entry, "export default function App() {}\n")
			}
			step := planner.PlanStep{
				Kind: planner.StepKindProjectSetup,
				Dir:  root,
				Setup: &planner.ProjectSetup{
					Linting: "biome", Formatting: "prettier",
					RecipeID: "expo", Template: test.template, Styling: test.styling,
				},
			}
			if err := writeProjectSetup(step); err != nil {
				t.Fatalf("writeProjectSetup() error = %v", err)
			}
			if err := writeProjectSetup(step); err != nil {
				t.Fatalf("idempotent writeProjectSetup() error = %v", err)
			}
			for _, name := range test.wantFiles {
				if _, err := os.Stat(filepath.Join(root, name)); err != nil {
					t.Fatalf("expected %s: %v", name, err)
				}
			}
			if test.entry != "" {
				content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(test.entry)))
				if err != nil {
					t.Fatal(err)
				}
				if count := strings.Count(string(content), test.wantImport); count != 1 {
					t.Fatalf("CSS import count = %d in %q", count, content)
				}
			}
			packageJSON := readPackageFixture(t, root)
			dependencies := packageJSON["dependencies"].(map[string]any)
			if _, exists := dependencies[test.wantPackage]; !exists {
				t.Fatalf("dependencies = %#v, missing %s", dependencies, test.wantPackage)
			}
		})
	}
}

func TestWriteProjectSetupRejectsSymlinkedPackageJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available")
	}
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "package.json")
	if err := os.WriteFile(target, []byte(`{"name":"outside"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "package.json")); err != nil {
		t.Fatal(err)
	}
	err := writeProjectSetup(planner.PlanStep{
		Dir:   root,
		Setup: &planner.ProjectSetup{Linting: "eslint", Formatting: "prettier"},
	})
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("error = %v, want regular-file rejection", err)
	}
}

func TestWriteProjectSetupRejectsSymlinkedExpoParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available")
	}
	root := t.TempDir()
	outside := t.TempDir()
	writeSetupFixture(t, root, "package.json", `{"name":"demo"}`)
	writeSetupFixture(t, outside, "app/_layout.tsx", "export default null\n")
	if err := os.Symlink(filepath.Join(outside, "app"), filepath.Join(root, "app")); err != nil {
		t.Fatal(err)
	}
	err := writeProjectSetup(planner.PlanStep{
		Dir: root,
		Setup: &planner.ProjectSetup{
			RecipeID: "expo", Template: "tabs", Linting: "eslint", Formatting: "prettier", Styling: "uniwind",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "symlinks") {
		t.Fatalf("error = %v, want symlink rejection", err)
	}
}

func writeSetupFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readPackageFixture(t *testing.T, root string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(content, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
