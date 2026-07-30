package project

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fileInfoWithMode struct {
	os.FileInfo
	mode os.FileMode
}

func (info fileInfoWithMode) Mode() os.FileMode {
	return info.mode
}

func statExecutable(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return fileInfoWithMode{
		FileInfo: info,
		mode:     info.Mode() | 0o111,
	}, nil
}

func TestSupportedEditorsReturnsCopy(t *testing.T) {
	t.Parallel()

	got := SupportedEditors()
	want := []string{"code", "cursor", "zed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportedEditors() = %#v, want %#v", got, want)
	}
	got[0] = "changed"
	if SupportedEditors()[0] != "code" {
		t.Fatal("SupportedEditors returned mutable package state")
	}
}

func TestEditorCommandKeepsHostileDirectoryAsOneArgument(t *testing.T) {
	t.Parallel()

	directory := `/tmp/project; touch should-not-exist $(also-not-run)`
	for _, editor := range []string{"code", "cursor", "zed"} {
		command, err := editorCommand(directory, editor)
		if err != nil {
			t.Fatalf("editorCommand(%q) error = %v", editor, err)
		}
		if command.name != editor {
			t.Fatalf("command name = %q, want %q", command.name, editor)
		}
		if !reflect.DeepEqual(command.args, []string{directory}) {
			t.Fatalf("command args = %#v", command.args)
		}
	}
}

func TestEditorCommandRejectsCommandInjectionAndPaths(t *testing.T) {
	t.Parallel()

	for _, editor := range []string{
		"",
		"CODE",
		"code --new-window",
		"code; touch pwned",
		"../code",
		"vim",
	} {
		_, err := editorCommand("/tmp/project", editor)
		if !errors.Is(err, ErrUnsupportedEditor) {
			t.Errorf("editorCommand(%q) error = %v, want ErrUnsupportedEditor", editor, err)
		}
	}
}

func TestEditorCommandAcceptsAbsoluteCustomExecutable(t *testing.T) {
	t.Parallel()

	executable := filepath.Join(t.TempDir(), "my editor")
	command, err := editorCommand("/tmp/project", executable)
	if err != nil {
		t.Fatal(err)
	}
	if command.name != executable || !reflect.DeepEqual(command.args, []string{"/tmp/project"}) {
		t.Fatalf("editorCommand() = %#v, want custom executable with one directory arg", command)
	}
}

func TestRevealCommandSelection(t *testing.T) {
	t.Parallel()

	directory := `/tmp/project; still-one-argument`
	tests := []struct {
		goos string
		want integrationCommand
	}{
		{
			goos: "darwin",
			want: integrationCommand{name: "open", args: []string{"-R", directory}},
		},
		{
			goos: "linux",
			want: integrationCommand{name: "xdg-open", args: []string{directory}},
		},
		{
			goos: "windows",
			want: integrationCommand{name: "explorer.exe", args: []string{directory}},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.goos, func(t *testing.T) {
			t.Parallel()
			got, err := revealCommandFor(test.goos, directory)
			if err != nil {
				t.Fatalf("revealCommandFor() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("revealCommandFor() = %#v, want %#v", got, test.want)
			}
		})
	}
	if _, err := revealCommandFor("plan9", directory); !errors.Is(
		err,
		ErrIntegrationUnavailable,
	) {
		t.Fatalf("unsupported reveal error = %v", err)
	}
}

func TestIntegrationLauncherResolvesKnownBinaryAndPreservesArgv(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	executable := filepath.Join(directory, "code")
	if err := os.WriteFile(executable, []byte("marker"), 0o700); err != nil {
		t.Fatal(err)
	}

	var gotExecutable string
	var gotArgs []string
	var gotPath string
	launcher := integrationLauncher{
		goos: "linux",
		resolvedPath: func() (string, error) {
			return directory, nil
		},
		start: func(path string, args []string, pathValue string) error {
			gotExecutable = path
			gotArgs = append([]string(nil), args...)
			gotPath = pathValue
			return nil
		},
		stat: statExecutable,
	}
	hostileDirectory := `/tmp/project; echo not-run`
	err := launcher.launch(integrationCommand{
		name: "code",
		args: []string{hostileDirectory},
	})
	if err != nil {
		t.Fatalf("launch() error = %v", err)
	}
	executable, _ = filepath.Abs(executable)
	if gotExecutable != executable {
		t.Fatalf("executable = %q, want %q", gotExecutable, executable)
	}
	if !reflect.DeepEqual(gotArgs, []string{hostileDirectory}) {
		t.Fatalf("args = %#v", gotArgs)
	}
	if gotPath != directory {
		t.Fatalf("PATH = %q, want %q", gotPath, directory)
	}
}

func TestIntegrationLauncherStartsAbsoluteCustomExecutableWithoutShell(t *testing.T) {
	t.Parallel()

	executable := filepath.Join(t.TempDir(), "editor with spaces")
	if err := os.WriteFile(executable, []byte("marker"), 0o700); err != nil {
		t.Fatal(err)
	}
	var gotExecutable string
	var gotArgs []string
	launcher := integrationLauncher{
		goos:         "linux",
		resolvedPath: func() (string, error) { return "/tools", nil },
		start: func(path string, args []string, _ string) error {
			gotExecutable = path
			gotArgs = append([]string(nil), args...)
			return nil
		},
		stat: statExecutable,
	}
	projectDirectory := `/tmp/project; still-not-shell`
	if err := launcher.launch(integrationCommand{
		name: executable,
		args: []string{projectDirectory},
	}); err != nil {
		t.Fatal(err)
	}
	if gotExecutable != executable || !reflect.DeepEqual(gotArgs, []string{projectDirectory}) {
		t.Fatalf("launch() executable=%q args=%#v", gotExecutable, gotArgs)
	}
}

func TestFindIntegrationExecutableResolvesWindowsShimToNativeEditor(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "Cursor")
	binDirectory := filepath.Join(directory, "resources", "app", "bin")
	if err := os.MkdirAll(binDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDirectory, "cursor.cmd")
	if err := os.WriteFile(shim, []byte("marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	native := filepath.Join(directory, "Cursor.exe")
	if err := os.WriteFile(native, []byte("native"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := findIntegrationExecutable("cursor", binDirectory, "windows", os.Stat)
	if err != nil {
		t.Fatalf("findIntegrationExecutable() error = %v", err)
	}
	native, _ = filepath.Abs(native)
	if got != native {
		t.Fatalf("findIntegrationExecutable() = %q, want %q", got, native)
	}
}

func TestFindIntegrationExecutableRejectsWindowsBatchShimWithoutNativeEditor(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "code.cmd"), []byte("marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := findIntegrationExecutable("code", directory, "windows", os.Stat); !errors.Is(
		err,
		ErrIntegrationUnavailable,
	) {
		t.Fatalf("findIntegrationExecutable() error = %v, want unavailable", err)
	}
}

func TestFindIntegrationExecutableRejectsExecutablePath(t *testing.T) {
	t.Parallel()

	_, err := findIntegrationExecutable("../code", t.TempDir(), "linux", os.Stat)
	if !errors.Is(err, ErrIntegrationUnavailable) {
		t.Fatalf("findIntegrationExecutable() error = %v", err)
	}
}

func TestCanonicalDirectory(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	got, err := canonicalDirectory(directory)
	if err != nil {
		t.Fatalf("canonicalDirectory() error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("canonicalDirectory() = %q, want absolute", got)
	}

	file := filepath.Join(directory, "file")
	if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", filepath.Join(directory, "missing"), file} {
		if _, err := canonicalDirectory(invalid); !errors.Is(err, ErrInvalidParent) {
			t.Errorf("canonicalDirectory(%q) error = %v, want ErrInvalidParent", invalid, err)
		}
	}
}
