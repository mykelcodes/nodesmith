package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"nodesmith/internal/planner"
)

func TestRenderProjectConfigPreservesExistingSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		config  planner.ProjectConfig
		want    string
	}{
		{
			name:    "pnpm workspace yaml",
			content: "packages:\n  - apps/*\nminimumReleaseAge: 60\n",
			config: planner.ProjectConfig{
				Format: planner.ConfigFormatYAML,
				Key:    "minimumReleaseAge",
				Value:  "4320",
			},
			want: "packages:\n  - apps/*\nminimumReleaseAge: 4320\n",
		},
		{
			name:    "npm properties",
			content: "save-exact=true\nmin-release-age=1\n",
			config: planner.ProjectConfig{
				Format: planner.ConfigFormatProperties,
				Key:    "min-release-age",
				Value:  "3",
			},
			want: "save-exact=true\nmin-release-age=3\n",
		},
		{
			name:    "bun install section",
			content: "[install]\nexact = true\n\n[test]\ncoverage = true\n",
			config: planner.ProjectConfig{
				Format:  planner.ConfigFormatTOML,
				Section: "install",
				Key:     "minimumReleaseAge",
				Value:   "259200",
			},
			want: "[install]\nexact = true\nminimumReleaseAge = 259200\n\n[test]\ncoverage = true\n",
		},
		{
			name:    "new bun install section",
			content: "logLevel = \"debug\"\n",
			config: planner.ProjectConfig{
				Format:  planner.ConfigFormatTOML,
				Section: "install",
				Key:     "minimumReleaseAge",
				Value:   "0",
			},
			want: "logLevel = \"debug\"\n\n[install]\nminimumReleaseAge = 0\n",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := renderProjectConfig(test.content, test.config)
			if err != nil {
				t.Fatalf("renderProjectConfig() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("rendered config\n--- got ---\n%s--- want ---\n%s", got, test.want)
			}
		})
	}
}

func TestWriteProjectConfigCreatesAndUpdatesTheGeneratedProjectFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	path := filepath.Join(projectDir, "pnpm-workspace.yaml")
	if err := os.WriteFile(path, []byte("packages:\n  - packages/*\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	step := planner.PlanStep{
		Kind: planner.StepKindProjectConfig,
		Dir:  projectDir,
		Config: &planner.ProjectConfig{
			Path:   "pnpm-workspace.yaml",
			Format: planner.ConfigFormatYAML,
			Key:    "minimumReleaseAge",
			Value:  "10080",
		},
	}
	if err := writeProjectConfig(step); err != nil {
		t.Fatalf("writeProjectConfig() error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "packages:\n  - packages/*") ||
		!strings.Contains(string(content), "minimumReleaseAge: 10080") {
		t.Fatalf("pnpm-workspace.yaml = %q, want existing content and release age", content)
	}
	// Windows does not expose or preserve POSIX permission bits.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o640 {
			t.Fatalf("file mode = %o, want existing 640", got)
		}
	}
}
