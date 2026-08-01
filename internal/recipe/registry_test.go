package recipe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadRegistryOverridesAndOrdersRecipes(t *testing.T) {
	t.Parallel()

	basic := fixtureText(t, "testdata/valid/basic.json")
	alpha := strings.Replace(strings.Replace(basic, `"id": "basic"`, `"id": "alpha"`, 1), `"name": "Basic"`, `"name": "Alpha embedded"`, 1)
	zulu := strings.Replace(strings.Replace(basic, `"id": "basic"`, `"id": "zulu"`, 1), `"name": "Basic"`, `"name": "Zulu"`, 1)
	embedded := fstest.MapFS{
		"alpha.json":         {Data: []byte(alpha)},
		"zulu.json":          {Data: []byte(zulu)},
		"recipe.schema.json": {Data: []byte(`{"not":"a recipe"}`)},
	}

	userDir := t.TempDir()
	override := strings.Replace(alpha, `"name": "Alpha embedded"`, `"name": "Alpha user"`, 1)
	if err := os.WriteFile(filepath.Join(userDir, "override.json"), []byte(override), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "broken.json"), []byte(`{"bad":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "notes.txt"), []byte(`ignored`), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, report, err := Load(embedded, userDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if registry.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", registry.Len())
	}
	if strings.Join(report.Overrides, ",") != "alpha" {
		t.Fatalf("Overrides = %v, want [alpha]", report.Overrides)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "broken.json") {
		t.Fatalf("Warnings = %v, want broken.json warning", report.Warnings)
	}
	list, err := registry.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 || list[0].ID != "alpha" || list[1].ID != "zulu" {
		t.Fatalf("List() ids = %v, want alpha,zulu", []string{list[0].ID, list[1].ID})
	}
	if list[0].Name != "Alpha user" {
		t.Fatalf("overridden name = %q, want Alpha user", list[0].Name)
	}

	list[0].Name = "mutated"
	got, err := registry.Get("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Alpha user" {
		t.Fatalf("registry leaked mutable manifest: name = %q", got.Name)
	}
}

func TestBundledCatalogueLoadsAllRecipes(t *testing.T) {
	t.Parallel()

	registry, report, err := Load(os.DirFS("../../recipes"), "")
	if err != nil {
		t.Fatalf("Load() bundled catalogue error = %v", err)
	}
	if registry.Len() != 14 {
		t.Fatalf("bundled catalogue length = %d, want 14", registry.Len())
	}
	if len(report.Warnings) != 0 || len(report.Overrides) != 0 {
		t.Fatalf("bundled catalogue report = %#v, want empty report", report)
	}
	wails, err := registry.Get("wails")
	if err != nil {
		t.Fatal(err)
	}
	if wails.Name != "Wails v2" {
		t.Fatalf("Wails recipe name = %q, want Wails v2", wails.Name)
	}
	if wails.InstallPolicy != InstallRequired {
		t.Fatalf("Wails install policy = %q, want required", wails.InstallPolicy)
	}
	electron, err := registry.Get("electron")
	if err != nil {
		t.Fatal(err)
	}
	if electron.InstallPolicy != InstallRequired {
		t.Fatalf("Electron install policy = %q, want required", electron.InstallPolicy)
	}
	hono, err := registry.Get("hono")
	if err != nil {
		t.Fatal(err)
	}
	if hono.InstallPolicy != InstallRequired {
		t.Fatalf("Hono install policy = %q, want required", hono.InstallPolicy)
	}
	vite, err := registry.Get("vite")
	if err != nil {
		t.Fatal(err)
	}
	if vite.InstallPolicy != "" {
		t.Fatalf("Vite install policy = %q, want backwards-compatible optional default", vite.InstallPolicy)
	}
}

func TestLoadRegistryFailureModes(t *testing.T) {
	t.Parallel()

	basic := fixtureText(t, "testdata/valid/basic.json")
	duplicate := strings.Replace(basic, `"name": "Basic"`, `"name": "Duplicate"`, 1)
	tests := []struct {
		name     string
		embedded fstest.MapFS
		want     string
	}{
		{
			name: "invalid embedded",
			embedded: fstest.MapFS{
				"bad.json": {Data: []byte(`{"schemaVersion":1}`)},
			},
			want: "bad.json",
		},
		{
			name: "duplicate embedded id",
			embedded: fstest.MapFS{
				"a.json": {Data: []byte(basic)},
				"b.json": {Data: []byte(duplicate)},
			},
			want: "duplicate id",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := Load(test.embedded, "")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want substring %q", err, test.want)
			}
		})
	}

	if _, _, err := Load(nil, ""); err == nil {
		t.Fatal("Load(nil) unexpectedly succeeded")
	}
	registry, report, err := Load(fstest.MapFS{"basic.json": {Data: []byte(basic)}}, filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("Load() missing user dir error = %v", err)
	}
	if registry.Len() != 1 || len(report.Warnings) != 0 {
		t.Fatalf("missing user dir result = len %d warnings %v", registry.Len(), report.Warnings)
	}
	if _, err := registry.Get("missing"); err == nil {
		t.Fatal("Get(missing) unexpectedly succeeded")
	}
	if _, err := (*Registry)(nil).Get("missing"); err == nil {
		t.Fatal("nil Registry.Get unexpectedly succeeded")
	}
	nilList, nilErr := (*Registry)(nil).List()
	if nilErr != nil {
		t.Fatalf("nil Registry.List() error = %v", nilErr)
	}
	if (*Registry)(nil).Len() != 0 || len(nilList) != 0 {
		t.Fatal("nil registry accessors returned non-empty values")
	}
}

func TestGetReturnsAnIsolatedCopy(t *testing.T) {
	t.Parallel()

	registry, _, err := Load(os.DirFS("../../recipes"), "")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	manifests, err := registry.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(manifests) == 0 {
		t.Fatal("List() returned no manifests")
	}
	id := manifests[0].ID

	first, err := registry.Get(id)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", id, err)
	}
	if len(first.Tags) == 0 || len(first.Steps) == 0 {
		t.Fatalf("recipe %q has nothing shared to mutate", id)
	}
	originalTag := first.Tags[0]
	originalCI := first.Steps[0].Env["CI"]

	// A caller that mutates its copy must not be able to reach the registry's
	// stored manifest through the slices and maps inside it.
	first.Tags[0] = "mutated"
	first.Steps[0].Env["CI"] = "mutated"

	second, err := registry.Get(id)
	if err != nil {
		t.Fatalf("second Get(%q) error = %v", id, err)
	}
	if second.Tags[0] != originalTag {
		t.Fatalf("Tags[0] = %q, want %q: Get returned an aliased manifest", second.Tags[0], originalTag)
	}
	if second.Steps[0].Env["CI"] != originalCI {
		t.Fatalf(
			"Steps[0].Env[CI] = %q, want %q: Get returned an aliased manifest",
			second.Steps[0].Env["CI"],
			originalCI,
		)
	}
}

func TestDuplicateUserRecipeIsWarning(t *testing.T) {
	t.Parallel()

	basic := fixtureText(t, "testdata/valid/basic.json")
	userDir := t.TempDir()
	for _, name := range []string{"a.json", "b.json"} {
		if err := os.WriteFile(filepath.Join(userDir, name), []byte(basic), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	registry, report, err := Load(fstest.MapFS{}, userDir)
	if err != nil {
		t.Fatal(err)
	}
	if registry.Len() != 1 || len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "duplicate id") {
		t.Fatalf("duplicate user result = len %d warnings %v", registry.Len(), report.Warnings)
	}
}
