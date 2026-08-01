package toolchain

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveInPathFindsExecutableAndSkipsNonExecutable(t *testing.T) {
	t.Parallel()

	first := t.TempDir()
	second := t.TempDir()
	if err := os.WriteFile(filepath.Join(first, "node"), []byte("not executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(second, "node")
	if err := os.WriteFile(want, []byte("executable marker"), 0o700); err != nil {
		t.Fatal(err)
	}
	stat := func(path string) (os.FileInfo, error) {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		mode := info.Mode() &^ 0o111
		if path == want {
			mode |= 0o111
		}
		return fileInfoWithMode{
			FileInfo: info,
			mode:     mode,
		}, nil
	}

	got, err := resolveInPath(
		"node",
		first+string(os.PathListSeparator)+second,
		"linux",
		stat,
	)
	if err != nil {
		t.Fatalf("resolveInPath() error = %v", err)
	}
	want, _ = filepath.Abs(want)
	if got != want {
		t.Fatalf("resolveInPath() = %q, want %q", got, want)
	}
}

type fileInfoWithMode struct {
	os.FileInfo
	mode os.FileMode
}

func (info fileInfoWithMode) Mode() os.FileMode {
	return info.mode
}

func TestResolveInPathFindsWindowsCommandAndPowerShellShims(t *testing.T) {
	t.Parallel()

	tests := []string{"npm.cmd", "pnpm.ps1"}
	for _, filename := range tests {
		filename := filename
		t.Run(filename, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			want := filepath.Join(directory, filename)
			if err := os.WriteFile(want, []byte("shim"), 0o600); err != nil {
				t.Fatal(err)
			}
			name := filename[:len(filename)-len(filepath.Ext(filename))]
			got, err := resolveInPath(name, directory, "windows", os.Stat)
			if err != nil {
				t.Fatalf("resolveInPath() error = %v", err)
			}
			want, _ = filepath.Abs(want)
			if got != want {
				t.Fatalf("resolveInPath() = %q, want %q", got, want)
			}
		})
	}
}

func TestResolveInPathPrefersWindowsNativeExecutable(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for _, filename := range []string{"npm.exe", "npm.cmd"} {
		if err := os.WriteFile(filepath.Join(directory, filename), []byte(filename), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := resolveInPath("npm", directory, "windows", os.Stat)
	if err != nil {
		t.Fatalf("resolveInPath() error = %v", err)
	}
	if filepath.Base(got) != "npm.exe" {
		t.Fatalf("resolveInPath() chose %q, want npm.exe", got)
	}
}

func TestResolveInPathPrefersWindowsCommandShimOverPOSIXShim(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for _, filename := range []string{"npm", "npm.cmd"} {
		if err := os.WriteFile(filepath.Join(directory, filename), []byte(filename), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := resolveInPath("npm", directory, "windows", os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "npm.cmd" {
		t.Fatalf("resolveInPath() chose %q, want npm.cmd", got)
	}
}

func TestResolveCommandTranslatesWindowsNodeShimWithoutShell(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	nodePath := filepath.Join(directory, "node.exe")
	shimPath := filepath.Join(directory, "npm.cmd")
	scriptPath := filepath.Join(directory, "node_modules", "npm", "bin", "npm-cli.js")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		nodePath:   "native node",
		shimPath:   "batch shim",
		scriptPath: "npm cli",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	paths := NewPathResolver()
	if err := paths.SetOverride(directory); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(paths)
	resolver.goos = "windows"

	binary, prefix, err := resolver.ResolveCommand("npm")
	if err != nil {
		t.Fatal(err)
	}
	absoluteNode, _ := filepath.Abs(nodePath)
	absoluteScript, _ := filepath.Abs(scriptPath)
	if binary != absoluteNode || !reflect.DeepEqual(prefix, []string{absoluteScript}) {
		t.Fatalf(
			"ResolveCommand() = %q %#v, want node %q with script %q",
			binary,
			prefix,
			absoluteNode,
			absoluteScript,
		)
	}
}

func TestResolveCommandPrefersNodeAdjacentToSelectedWindowsShim(t *testing.T) {
	t.Parallel()

	earlierDirectory := t.TempDir()
	shimDirectory := t.TempDir()
	earlierNode := filepath.Join(earlierDirectory, "node.exe")
	adjacentNode := filepath.Join(shimDirectory, "node.exe")
	shimPath := filepath.Join(shimDirectory, "npm.cmd")
	scriptPath := filepath.Join(shimDirectory, "node_modules", "npm", "bin", "npm-cli.js")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		earlierNode:  "different node installation",
		adjacentNode: "node paired with npm shim",
		shimPath:     "batch shim",
		scriptPath:   "npm cli",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	paths := NewPathResolver()
	pathValue := earlierDirectory + string(os.PathListSeparator) + shimDirectory
	if err := paths.SetOverride(pathValue); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(paths)
	resolver.goos = "windows"

	binary, prefix, err := resolver.ResolveCommand("npm")
	if err != nil {
		t.Fatal(err)
	}
	absoluteNode, _ := filepath.Abs(adjacentNode)
	absoluteScript, _ := filepath.Abs(scriptPath)
	if binary != absoluteNode || !reflect.DeepEqual(prefix, []string{absoluteScript}) {
		t.Fatalf(
			"ResolveCommand() = %q %#v, want adjacent node %q with script %q",
			binary,
			prefix,
			absoluteNode,
			absoluteScript,
		)
	}
}

func TestResolveCommandFallsBackToNodeOnPathForGlobalWindowsShim(t *testing.T) {
	t.Parallel()

	nodeDirectory := t.TempDir()
	shimDirectory := t.TempDir()
	nodePath := filepath.Join(nodeDirectory, "node.exe")
	shimPath := filepath.Join(shimDirectory, "node_modules", "pnpm", "bin", "pnpm.cjs")
	if err := os.MkdirAll(filepath.Dir(shimPath), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		nodePath:                                 "native node",
		filepath.Join(shimDirectory, "pnpm.cmd"): "global package shim",
		shimPath:                                 "pnpm cli",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	paths := NewPathResolver()
	pathValue := nodeDirectory + string(os.PathListSeparator) + shimDirectory
	if err := paths.SetOverride(pathValue); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(paths)
	resolver.goos = "windows"

	binary, prefix, err := resolver.ResolveCommand("pnpm")
	if err != nil {
		t.Fatal(err)
	}
	absoluteNode, _ := filepath.Abs(nodePath)
	absoluteScript, _ := filepath.Abs(shimPath)
	if binary != absoluteNode || !reflect.DeepEqual(prefix, []string{absoluteScript}) {
		t.Fatalf(
			"ResolveCommand() = %q %#v, want PATH node %q with script %q",
			binary,
			prefix,
			absoluteNode,
			absoluteScript,
		)
	}
}

func TestResolveCommandRejectsUnknownWindowsBatchShim(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "code.cmd"), []byte("shim"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths := NewPathResolver()
	if err := paths.SetOverride(directory); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(paths)
	resolver.goos = "windows"
	if _, _, err := resolver.ResolveCommand("code"); !errors.Is(err, ErrBinaryNotFound) {
		t.Fatalf("ResolveCommand(code.cmd) error = %v, want ErrBinaryNotFound", err)
	}
}

func TestResolveInPathMissing(t *testing.T) {
	t.Parallel()

	_, err := resolveInPath("node", t.TempDir(), "linux", os.Stat)
	if !errors.Is(err, ErrBinaryNotFound) {
		t.Fatalf("resolveInPath() error = %v, want ErrBinaryNotFound", err)
	}
}

// PATH replacement moved to internal/environ, which is now the single
// implementation shared by the runner, the detector, and editor launch.
// See internal/environ/environ_test.go.
