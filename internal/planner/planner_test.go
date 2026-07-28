package planner

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"nodesmith/internal/recipe"
)

var updateGoldens = flag.Bool("update", false, "update bundled recipe golden plans")

type fakeResolver struct {
	paths map[string]string
	err   error
	calls []string
}

type fakeCommandResolver struct {
	fakeResolver
	prefix []string
}

func (resolver *fakeCommandResolver) ResolveCommand(name string) (string, []string, error) {
	path, err := resolver.Resolve(name)
	return path, append([]string(nil), resolver.prefix...), err
}

func (resolver *fakeResolver) Resolve(name string) (string, error) {
	resolver.calls = append(resolver.calls, name)
	if resolver.err != nil {
		return "", resolver.err
	}
	if path, exists := resolver.paths[name]; exists {
		return path, nil
	}
	return "/tools/" + name, nil
}

func TestResolveBundledRecipeGoldens(t *testing.T) {
	registry, _, err := recipe.Load(os.DirFS("../../recipes"), "")
	if err != nil {
		t.Fatalf("load bundled recipes: %v", err)
	}
	manifests := registry.List()
	if len(manifests) != 14 {
		t.Fatalf("bundled recipe count = %d, want 14", len(manifests))
	}

	for _, manifest := range manifests {
		manifest := manifest
		t.Run(manifest.ID, func(t *testing.T) {
			packageManager := ""
			if len(manifest.Requires.PackageManagers) > 0 {
				packageManager = manifest.Requires.PackageManagers[0]
			}
			plan, err := Resolve(manifest, ScaffoldRequest{
				RecipeID:       manifest.ID,
				ProjectName:    manifest.ID + "-app",
				ParentDir:      ".",
				PackageManager: packageManager,
				InstallDeps:    true,
				GitInit:        true,
				Answers:        map[string]any{},
			}, &fakeResolver{})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if len(plan.Steps) == 0 {
				t.Fatal("resolved plan contains no steps")
			}
			got, err := json.MarshalIndent(plan, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')

			path := "testdata/golden/" + manifest.ID + ".json"
			if *updateGoldens {
				if err := os.WriteFile(path, got, 0o600); err != nil {
					t.Fatalf("update golden: %v", err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Fatalf(
					"resolved %s plan changed\n--- got ---\n%s\n--- want ---\n%s",
					manifest.Name,
					got,
					want,
				)
			}
		})
	}
}

func TestResolveNeverSplitsOrReinterpretsSubstitutedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		projectName string
	}{
		{name: "spaces", projectName: "my app"},
		{name: "double quotes", projectName: `my "quoted" app`},
		{name: "single quotes", projectName: "my 'quoted' app"},
		{name: "backticks", projectName: "`touch should-not-run`"},
		{name: "command substitution", projectName: "$(touch should-not-run)"},
		{name: "semicolons", projectName: "demo; touch should-not-run"},
		{name: "unicode", projectName: "café-项目-🚀"},
		{name: "very long", projectName: strings.Repeat("long-name-", 512)},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			plan, err := Resolve(loadViteManifest(t), ScaffoldRequest{
				RecipeID:       "vite",
				ProjectName:    test.projectName,
				ParentDir:      "/work space",
				PackageManager: "npm",
				InstallDeps:    false,
				GitInit:        false,
				Answers:        map[string]any{"template": "vanilla-ts"},
			}, &fakeResolver{})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if len(plan.Steps) != 1 {
				t.Fatalf("step count = %d, want only scaffold", len(plan.Steps))
			}
			wantArgs := []string{
				"--yes",
				"create-vite@latest",
				test.projectName,
				"--template",
				"vanilla-ts",
				"--no-interactive",
				"--no-immediate",
			}
			if len(plan.Steps[0].Args) != len(wantArgs) {
				t.Fatalf(
					"argv element count = %d, want %d: %#v",
					len(plan.Steps[0].Args),
					len(wantArgs),
					plan.Steps[0].Args,
				)
			}
			if !reflect.DeepEqual(plan.Steps[0].Args, wantArgs) {
				t.Fatalf("Args = %#v, want exact argv %#v", plan.Steps[0].Args, wantArgs)
			}
		})
	}
}

func TestResolvePrependsNativeResolverArguments(t *testing.T) {
	t.Parallel()

	resolver := &fakeCommandResolver{prefix: []string{"/tools/npm-cli.js"}}
	plan, err := Resolve(loadViteManifest(t), ScaffoldRequest{
		RecipeID:       "vite",
		ProjectName:    "demo",
		ParentDir:      ".",
		PackageManager: "npm",
		Answers:        map[string]any{},
	}, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Args[0] != "/tools/npm-cli.js" {
		t.Fatalf("resolved steps = %#v, want fixed native argv prefix", plan.Steps)
	}
}

func TestResolveNestJSDoesNotApplyHiddenStrictDefaultToJavaScript(t *testing.T) {
	t.Parallel()

	plan, err := Resolve(loadBundledManifest(t, "nestjs"), ScaffoldRequest{
		RecipeID:       "nestjs",
		ProjectName:    "api",
		ParentDir:      ".",
		PackageManager: "npm",
		Answers:        map[string]any{"language": "javascript"},
	}, &fakeResolver{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("step count = %d, want scaffold only", len(plan.Steps))
	}
	if slices.Contains(plan.Steps[0].Args, "--strict") {
		t.Fatalf("JavaScript scaffold args contain --strict: %#v", plan.Steps[0].Args)
	}
}

func TestResolveWarnsWhenInstallPolicyIsRequired(t *testing.T) {
	t.Parallel()

	manifest := loadBundledManifest(t, "electron")
	plan, err := Resolve(manifest, ScaffoldRequest{
		RecipeID:       manifest.ID,
		ProjectName:    "api",
		ParentDir:      ".",
		PackageManager: "npm",
		InstallDeps:    false,
		Answers:        map[string]any{},
	}, &fakeResolver{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) != 1 || !strings.Contains(plan.Warnings[0], "cannot be disabled") {
		t.Fatalf("warnings = %#v, want required-install warning", plan.Warnings)
	}
}

func TestResolveExpandsConditionsAndForEach(t *testing.T) {
	t.Parallel()

	manifest := complexManifest()
	resolver := &fakeResolver{}
	plan, err := Resolve(manifest, ScaffoldRequest{
		RecipeID:       manifest.ID,
		ProjectName:    "demo",
		ParentDir:      "/projects",
		PackageManager: "npm",
		InstallDeps:    false,
		GitInit:        true,
		Answers: map[string]any{
			"flavor": "full",
			"addons": []any{"lint", "test"},
			"label":  "two words",
			"count":  json.Number("2.5"),
		},
	}, resolver)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(plan.Steps))
	}
	wantArgs := []string{
		"create",
		"--enabled",
		"--full",
		"--add",
		"lint",
		"--add",
		"test",
		"name=two words",
		"2.5",
	}
	if !reflect.DeepEqual(plan.Steps[0].Args, wantArgs) {
		t.Fatalf("Args = %#v, want %#v", plan.Steps[0].Args, wantArgs)
	}
	if strings.Join(resolver.calls, ",") != "npx" {
		t.Fatalf("resolver calls = %v, want only npx for included step", resolver.calls)
	}
	if plan.Steps[0].Env["A"] != "first" || plan.Steps[0].Env["Z"] != "last" || plan.Steps[0].Env["CI"] != "1" {
		t.Fatalf("Env = %#v, want merged recipe env plus CI=1", plan.Steps[0].Env)
	}
}

func TestExpandArgNodesEmptyCollectionsAndMissingElseEmitNoArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		nodes  []recipe.ArgNode
		values map[string]any
	}{
		{
			name: "empty forEach",
			nodes: []recipe.ArgNode{{
				Iteration: &recipe.ForEachArg{
					Field: "addons",
					Args: []recipe.ArgNode{
						recipe.Literal("--add"),
						recipe.Literal("${item}"),
					},
				},
			}},
			values: map[string]any{"addons": []string{}},
		},
		{
			name: "false condition with missing else",
			nodes: []recipe.ArgNode{{
				Conditional: &recipe.ConditionalArg{
					If:   "enabled",
					Then: []recipe.ArgNode{recipe.Literal("--enabled")},
				},
			}},
			values: map[string]any{"enabled": false},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			args, err := expandArgNodes(test.nodes, test.values, 0)
			if err != nil {
				t.Fatalf("expandArgNodes() error = %v", err)
			}
			if len(args) != 0 {
				t.Fatalf("expandArgNodes() = %#v, want no argv elements", args)
			}
		})
	}
}

func TestPlannerUnknownIdentifiersReturnErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "substitution",
			run: func() error {
				_, err := expandArgNodes(
					[]recipe.ArgNode{recipe.Literal("${missing}")},
					map[string]any{},
					0,
				)
				return err
			},
		},
		{
			name: "condition",
			run: func() error {
				_, err := evaluateWhen("missing", map[string]any{})
				return err
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.run()
			if err == nil || !strings.Contains(err.Error(), `unknown identifier "missing"`) {
				t.Fatalf("planner error = %v, want unknown identifier", err)
			}
		})
	}
}

func TestResolveHashIsStable(t *testing.T) {
	t.Parallel()

	manifestA := complexManifest()
	manifestB := complexManifest()
	manifestA.Steps[0].Env = map[string]string{"CI": "1", "A": "first", "Z": "last"}
	manifestB.Steps[0].Env = map[string]string{"Z": "last", "CI": "1", "A": "first"}
	request := ScaffoldRequest{
		RecipeID:       manifestA.ID,
		ProjectName:    "demo",
		ParentDir:      ".",
		PackageManager: "npm",
		GitInit:        true,
		Answers:        map[string]any{},
	}
	first, err := Resolve(manifestA, request, &fakeResolver{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Resolve(manifestB, request, &fakeResolver{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash || len(first.Hash) != 64 {
		t.Fatalf("hashes are not stable across env map order: %q %q", first.Hash, second.Hash)
	}
	for iteration := 0; iteration < 100; iteration++ {
		resolved, err := Resolve(manifestA, request, &fakeResolver{})
		if err != nil {
			t.Fatalf("Resolve() iteration %d error = %v", iteration, err)
		}
		if resolved.Hash != first.Hash {
			t.Fatalf(
				"Resolve() iteration %d hash = %q, want %q",
				iteration,
				resolved.Hash,
				first.Hash,
			)
		}
	}
}

func TestResolveAllowsRecipeWithoutPackageManagers(t *testing.T) {
	t.Parallel()

	manifest := complexManifest()
	manifest.Requires.PackageManagers = []string{}
	manifest.Steps = []recipe.Step{
		{
			ID:    "build",
			Label: "Build",
			Bin:   "go",
			CWD:   "projectDir",
			Env:   map[string]string{"CI": "1"},
			Args:  []recipe.ArgNode{recipe.Literal("version")},
		},
	}
	plan, err := Resolve(manifest, ScaffoldRequest{
		RecipeID:       manifest.ID,
		ProjectName:    "demo",
		ParentDir:      ".",
		PackageManager: "",
		Answers:        map[string]any{},
	}, &fakeResolver{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Bin != "/tools/go" {
		t.Fatalf("Plan steps = %#v, want one Go step", plan.Steps)
	}
}

func TestResolveRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	baseRequest := ScaffoldRequest{
		RecipeID:       "vite",
		ProjectName:    "demo",
		ParentDir:      ".",
		PackageManager: "npm",
		Answers:        map[string]any{},
	}
	tests := []struct {
		name     string
		manifest func() recipe.Manifest
		request  func() ScaffoldRequest
		resolver BinaryResolver
		want     string
	}{
		{
			name:     "nil resolver",
			manifest: func() recipe.Manifest { return loadViteManifest(t) },
			request:  func() ScaffoldRequest { return baseRequest },
			resolver: nil,
			want:     "resolver is nil",
		},
		{
			name:     "recipe mismatch",
			manifest: func() recipe.Manifest { return loadViteManifest(t) },
			request: func() ScaffoldRequest {
				value := baseRequest
				value.RecipeID = "other"
				return value
			},
			resolver: &fakeResolver{},
			want:     "recipe id mismatch",
		},
		{
			name:     "unsupported package manager",
			manifest: func() recipe.Manifest { return loadViteManifest(t) },
			request: func() ScaffoldRequest {
				value := baseRequest
				value.PackageManager = "deno"
				return value
			},
			resolver: &fakeResolver{},
			want:     "not supported",
		},
		{
			name: "invalid recipe",
			manifest: func() recipe.Manifest {
				value := loadViteManifest(t)
				value.Steps = nil
				return value
			},
			request:  func() ScaffoldRequest { return baseRequest },
			resolver: &fakeResolver{},
			want:     "recipe \"vite\" is invalid",
		},
		{
			name:     "unknown answer",
			manifest: func() recipe.Manifest { return loadViteManifest(t) },
			request: func() ScaffoldRequest {
				value := baseRequest
				value.Answers = map[string]any{"missing": true}
				return value
			},
			resolver: &fakeResolver{},
			want:     "field does not exist",
		},
		{
			name:     "wrong select type",
			manifest: func() recipe.Manifest { return loadViteManifest(t) },
			request: func() ScaffoldRequest {
				value := baseRequest
				value.Answers = map[string]any{"template": true}
				return value
			},
			resolver: &fakeResolver{},
			want:     "must be a string",
		},
		{
			name:     "unavailable select",
			manifest: func() recipe.Manifest { return loadViteManifest(t) },
			request: func() ScaffoldRequest {
				value := baseRequest
				value.Answers = map[string]any{"template": "missing"}
				return value
			},
			resolver: &fakeResolver{},
			want:     "not among",
		},
		{
			name:     "resolver error",
			manifest: func() recipe.Manifest { return loadViteManifest(t) },
			request:  func() ScaffoldRequest { return baseRequest },
			resolver: &fakeResolver{err: errors.New("not found")},
			want:     "not found",
		},
		{
			name:     "empty resolved path",
			manifest: func() recipe.Manifest { return loadViteManifest(t) },
			request:  func() ScaffoldRequest { return baseRequest },
			resolver: &fakeResolver{paths: map[string]string{"npx": ""}},
			want:     "empty path",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Resolve(test.manifest(), test.request(), test.resolver)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Resolve() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestAnswerValidation(t *testing.T) {
	t.Parallel()

	fields := []recipe.Field{
		{ID: "multi", Type: recipe.FieldMultiselect, Options: []recipe.Option{{Value: "a"}, {Value: "b"}}},
		{ID: "bool", Type: recipe.FieldBoolean},
		{ID: "text", Type: recipe.FieldText},
		{ID: "number", Type: recipe.FieldNumber},
	}
	tests := []struct {
		name  string
		field recipe.Field
		value any
		want  string
	}{
		{name: "multi wrong type", field: fields[0], value: true, want: "array of strings"},
		{name: "multi wrong item", field: fields[0], value: []any{"a", true}, want: "array of strings"},
		{name: "multi unavailable", field: fields[0], value: []string{"x"}, want: "not among"},
		{name: "multi duplicate", field: fields[0], value: []string{"a", "a"}, want: "duplicate"},
		{name: "bool wrong", field: fields[1], value: "yes", want: "true or false"},
		{name: "text wrong", field: fields[2], value: 1, want: "must be a string"},
		{name: "number wrong", field: fields[3], value: "1", want: "must be numeric"},
		{name: "unknown type", field: recipe.Field{Type: "date"}, value: "now", want: "unsupported field type"},
	}
	for _, test := range tests {
		_, err := validateAnswer(test.field, test.value)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: validateAnswer() error = %v, want substring %q", test.name, err, test.want)
		}
	}
}

func TestStringValueScalarTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   any
		want    string
		wantErr string
	}{
		{name: "string", value: "value", want: "value"},
		{name: "bool", value: true, want: "true"},
		{name: "json number", value: json.Number("1.25"), want: "1.25"},
		{name: "invalid json number", value: json.Number("invalid"), wantErr: "invalid JSON number"},
		{name: "float64", value: float64(1.25), want: "1.25"},
		{name: "float32", value: float32(1.25), want: "1.25"},
		{name: "int", value: int(-1), want: "-1"},
		{name: "int8", value: int8(-8), want: "-8"},
		{name: "int16", value: int16(-16), want: "-16"},
		{name: "int32", value: int32(-32), want: "-32"},
		{name: "int64", value: int64(-64), want: "-64"},
		{name: "uint", value: uint(1), want: "1"},
		{name: "uint8", value: uint8(8), want: "8"},
		{name: "uint16", value: uint16(16), want: "16"},
		{name: "uint32", value: uint32(32), want: "32"},
		{name: "uint64", value: uint64(64), want: "64"},
		{name: "unsupported", value: []string{"one"}, wantErr: "cannot be substituted"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := stringValue(test.value)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("stringValue() error = %v, want substring %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("stringValue() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("stringValue() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPlannerDefensiveHelpers(t *testing.T) {
	t.Parallel()

	if got := quoteForDisplay(""); got != "''" {
		t.Fatalf("quoteForDisplay(empty) = %q", got)
	}
	if got := quoteForDisplay("safe-value"); got != "safe-value" {
		t.Fatalf("quoteForDisplay(safe) = %q", got)
	}
	if _, err := substitute("${missing}", map[string]any{}); err == nil {
		t.Fatal("substitute unknown unexpectedly succeeded")
	}
	if _, err := substitute("${broken", map[string]any{}); err == nil {
		t.Fatal("substitute unterminated unexpectedly succeeded")
	}
	if _, err := substitute("${items}", map[string]any{"items": []string{"a"}}); err == nil {
		t.Fatal("substitute slice unexpectedly succeeded")
	}
	if _, err := resolveDirectory("bad", "", ""); err == nil {
		t.Fatal("resolveDirectory(bad) unexpectedly succeeded")
	}
	if _, err := expandArgNodes([]recipe.ArgNode{{}}, map[string]any{}, 0); err == nil {
		t.Fatal("expandArgNodes(empty union) unexpectedly succeeded")
	}
}

func loadViteManifest(t *testing.T) recipe.Manifest {
	return loadBundledManifest(t, "vite")
}

func loadBundledManifest(t *testing.T, id string) recipe.Manifest {
	t.Helper()
	file, err := os.Open("../../recipes/" + id + ".json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = file.Close()
	}()
	manifest, err := recipe.DecodeAndValidate(file)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func complexManifest() recipe.Manifest {
	return recipe.Manifest{
		SchemaVersion: 1,
		ID:            "complex",
		Name:          "Complex",
		Category:      "tooling",
		Description:   "Planner test recipe.",
		DocsURL:       "https://example.com/complex",
		Tags:          []string{"test"},
		Icon:          "complex",
		VerifiedAt:    "2026-07-28",
		Requires: recipe.Requirements{
			Node:            ">=20.0.0",
			PackageManagers: []string{"npm"},
			Tools:           []string{},
		},
		Fields: []recipe.Field{
			{
				ID:      "flavor",
				Label:   "Flavor",
				Type:    recipe.FieldSelect,
				Default: "lite",
				Options: []recipe.Option{{Value: "lite", Label: "Lite"}, {Value: "full", Label: "Full"}},
			},
			{
				ID:      "addons",
				Label:   "Addons",
				Type:    recipe.FieldMultiselect,
				Default: []string{"lint"},
				Options: []recipe.Option{{Value: "lint", Label: "Lint"}, {Value: "test", Label: "Test"}},
			},
			{ID: "enabled", Label: "Enabled", Type: recipe.FieldBoolean, Default: true},
			{ID: "label", Label: "Label", Type: recipe.FieldText, Default: "default"},
			{ID: "count", Label: "Count", Type: recipe.FieldNumber, Default: json.Number("1")},
		},
		Steps: []recipe.Step{
			{
				ID:    "create",
				Label: "Create",
				Bin:   "npx",
				CWD:   "projectDir",
				Env:   map[string]string{"Z": "last", "CI": "1", "A": "first"},
				Args: []recipe.ArgNode{
					recipe.Literal("create"),
					{
						Conditional: &recipe.ConditionalArg{
							If:   "enabled",
							Then: []recipe.ArgNode{recipe.Literal("--enabled")},
							Else: []recipe.ArgNode{recipe.Literal("--disabled")},
						},
					},
					{
						Conditional: &recipe.ConditionalArg{
							If:   `flavor == "full"`,
							Then: []recipe.ArgNode{recipe.Literal("--full")},
							Else: []recipe.ArgNode{recipe.Literal("--lite")},
						},
					},
					{
						Iteration: &recipe.ForEachArg{
							Field: "addons",
							Args:  []recipe.ArgNode{recipe.Literal("--add"), recipe.Literal("${item}")},
						},
					},
					recipe.Literal("name=${label}"),
					recipe.Literal("${count}"),
				},
				When: "gitInit",
			},
			{
				ID:    "skipped",
				Label: "Skipped",
				Bin:   "git",
				CWD:   "parentDir",
				Env:   map[string]string{"CI": "1"},
				Args:  []recipe.ArgNode{recipe.Literal("status")},
				When:  "installDeps",
			},
		},
	}
}
