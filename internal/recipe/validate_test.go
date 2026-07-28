package recipe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidFixtures(t *testing.T) {
	t.Parallel()

	expected := map[string]string{
		"arg-depth.json":               "nesting depth exceeds 3",
		"bin-not-allowlist.json":       "is not in the allowlist",
		"duplicate-id.json":            "duplicate id",
		"empty-steps.json":             "must contain at least one step",
		"foreach-non-multiselect.json": "is not a multiselect",
		"malformed-expression.json":    "expression parse error",
		"malformed-json.json":          "EOF",
		"select-default.json":          "is not among options",
		"shadow-builtin.json":          "shadows a built-in",
		"unknown-foreach.json":         `unknown identifier "missing"`,
		"unknown-substitution.json":    `unknown identifier "missing"`,
		"unknown-visible-if.json":      `unknown identifier "missing"`,
		"unknown-when.json":            `unknown identifier "missing"`,
		"unsupported-schema.json":      "unsupported version 2",
	}

	entries, err := os.ReadDir("testdata/invalid")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(expected) {
		t.Fatalf("invalid fixture count = %d, expected map count = %d", len(entries), len(expected))
	}
	for _, entry := range entries {
		entry := entry
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			want, exists := expected[entry.Name()]
			if !exists {
				t.Fatalf("fixture %s has no expected error", entry.Name())
			}
			file, err := os.Open(filepath.Join("testdata/invalid", entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = file.Close()
			}()
			_, err = DecodeAndValidate(file)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("DecodeAndValidate() error = %v, want substring %q", err, want)
			}
		})
	}
}

func TestValidateAdditionalStructuralRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{name: "invalid id", mutate: func(m *Manifest) { m.ID = "Not Valid" }, want: "kebab-case"},
		{name: "empty name", mutate: func(m *Manifest) { m.Name = "" }, want: "name: must not be empty"},
		{name: "bad category", mutate: func(m *Manifest) { m.Category = "other" }, want: "unsupported value"},
		{name: "empty description", mutate: func(m *Manifest) { m.Description = "" }, want: "description: must not be empty"},
		{name: "bad docs url", mutate: func(m *Manifest) { m.DocsURL = "relative" }, want: "absolute HTTP"},
		{name: "missing tags", mutate: func(m *Manifest) { m.Tags = nil }, want: "tags: must be an array"},
		{name: "bad tag", mutate: func(m *Manifest) { m.Tags = []string{"Not Valid"} }, want: "kebab-case"},
		{name: "duplicate tag", mutate: func(m *Manifest) { m.Tags = []string{"test", "test"} }, want: "duplicate value"},
		{name: "empty icon", mutate: func(m *Manifest) { m.Icon = "" }, want: "icon: must not be empty"},
		{name: "bad date", mutate: func(m *Manifest) { m.VerifiedAt = "today" }, want: "YYYY-MM-DD"},
		{name: "bad install policy", mutate: func(m *Manifest) { m.InstallPolicy = "sometimes" }, want: "installPolicy: unsupported value"},
		{name: "empty node requirement", mutate: func(m *Manifest) { m.Requires.Node = "" }, want: "requires.node"},
		{name: "missing package managers", mutate: func(m *Manifest) { m.Requires.PackageManagers = nil }, want: "must be an array"},
		{name: "missing tools", mutate: func(m *Manifest) { m.Requires.Tools = nil }, want: "must be an array"},
		{name: "bad package manager", mutate: func(m *Manifest) { m.Requires.PackageManagers = []string{"deno"} }, want: "unsupported package manager"},
		{name: "duplicate package manager", mutate: func(m *Manifest) { m.Requires.PackageManagers = []string{"npm", "npm"} }, want: "duplicate value"},
		{name: "bad tool", mutate: func(m *Manifest) { m.Requires.Tools = []string{"bash"} }, want: "not in the allowlist"},
		{name: "duplicate tool", mutate: func(m *Manifest) { m.Requires.Tools = []string{"git", "git"} }, want: "duplicate value"},
		{name: "missing fields", mutate: func(m *Manifest) { m.Fields = nil }, want: "fields: must be an array"},
		{name: "bad field id", mutate: func(m *Manifest) { m.Fields[0].ID = "Bad" }, want: "kebab-case"},
		{name: "empty field label", mutate: func(m *Manifest) { m.Fields[0].Label = "" }, want: "label: must not be empty"},
		{name: "bad field type", mutate: func(m *Manifest) { m.Fields[0].Type = "date" }, want: "unsupported value"},
		{name: "select wrong default type", mutate: func(m *Manifest) { m.Fields[0].Default = true }, want: "must be a string"},
		{name: "empty option value", mutate: func(m *Manifest) { m.Fields[0].Options[0].Value = "" }, want: "value: must not be empty"},
		{name: "duplicate option", mutate: func(m *Manifest) { m.Fields[0].Options[1].Value = "a" }, want: "duplicate value"},
		{name: "empty option label", mutate: func(m *Manifest) { m.Fields[0].Options[0].Label = "" }, want: "label: must not be empty"},
		{name: "multiselect wrong default", mutate: func(m *Manifest) { m.Fields[1].Default = true }, want: "array of strings"},
		{name: "multiselect unavailable default", mutate: func(m *Manifest) { m.Fields[1].Default = []string{"bad"} }, want: "is not among options"},
		{name: "boolean wrong default", mutate: func(m *Manifest) { m.Fields[2].Default = "yes" }, want: "must be true or false"},
		{name: "empty step id", mutate: func(m *Manifest) { m.Steps[0].ID = "" }, want: "kebab-case"},
		{name: "empty step label", mutate: func(m *Manifest) { m.Steps[0].Label = "" }, want: "label: must not be empty"},
		{name: "bad cwd", mutate: func(m *Manifest) { m.Steps[0].CWD = "elsewhere" }, want: "must be"},
		{name: "invalid env key", mutate: func(m *Manifest) { m.Steps[0].Env["BAD=KEY"] = "value" }, want: "invalid environment variable name"},
		{name: "nul env value", mutate: func(m *Manifest) { m.Steps[0].Env["BAD"] = "value\x00" }, want: "value contains a NUL byte"},
		{name: "case-insensitive env duplicate", mutate: func(m *Manifest) { m.Steps[0].Env["ci"] = "0" }, want: "case-insensitive platforms"},
		{name: "managed path env", mutate: func(m *Manifest) { m.Steps[0].Env["Path"] = "/untrusted" }, want: "PATH is managed by Nodesmith"},
		{name: "missing ci", mutate: func(m *Manifest) { delete(m.Steps[0].Env, "CI") }, want: "env.CI"},
		{name: "missing args", mutate: func(m *Manifest) { m.Steps[0].Args = nil }, want: "args: must be an array"},
		{name: "includes scalar", mutate: func(m *Manifest) { m.Steps[0].When = `choice includes "a"` }, want: "requires a multiselect"},
		{name: "unknown bin variable", mutate: func(m *Manifest) { m.Steps[0].Bin = "${missing}" }, want: "unknown identifier"},
		{name: "unsafe dynamic bin", mutate: func(m *Manifest) { m.Steps[0].Bin = "${choice}" }, want: "not in the allowlist"},
		{name: "invalid variable syntax", mutate: func(m *Manifest) { m.Steps[0].Args = []ArgNode{Literal("${bad value}")} }, want: "invalid variable reference"},
		{name: "invalid union", mutate: func(m *Manifest) { m.Steps[0].Args = []ArgNode{{}} }, want: "exactly one variant"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			manifest := validManifest(t)
			test.mutate(&manifest)
			err := Validate(manifest)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func validManifest(t *testing.T) Manifest {
	t.Helper()
	manifest, err := Decode(strings.NewReader(fixtureText(t, "testdata/valid/basic.json")))
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}
