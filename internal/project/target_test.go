package project

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveAndValidateNewTarget(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	target, err := ResolveTarget(parent, "my app", NameOptions{})
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}
	if !filepath.IsAbs(target.ParentDir) || !filepath.IsAbs(target.ProjectDir) {
		t.Fatalf("target paths are not absolute: %#v", target)
	}
	if target.Exists || target.Empty {
		t.Fatalf("new target state = %#v", target)
	}
	if filepath.Base(target.ProjectDir) != "my app" {
		t.Fatalf("ProjectDir = %q", target.ProjectDir)
	}

	validated, err := ValidateTarget(parent, "my app", NameOptions{})
	if err != nil {
		t.Fatalf("ValidateTarget() error = %v", err)
	}
	if validated != target {
		t.Fatalf("ValidateTarget() = %#v, want %#v", validated, target)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("write probe left entries behind: %#v", entries)
	}
}

func TestResolveTargetAllowsExistingEmptyDirectory(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	projectDir := filepath.Join(parent, "empty")
	if err := os.Mkdir(projectDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target, err := ValidateTarget(parent, "empty", NameOptions{})
	if err != nil {
		t.Fatalf("ValidateTarget() error = %v", err)
	}
	if !target.Exists || !target.Empty {
		t.Fatalf("existing empty target = %#v", target)
	}
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("write probe left entries behind: %#v", entries)
	}
}

func TestResolveTargetRejectsCollision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		makeEntry func(string) error
		want      error
	}{
		{
			name: "non-empty directory",
			makeEntry: func(path string) error {
				if err := os.Mkdir(path, 0o700); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(path, "package.json"), []byte("{}"), 0o600)
			},
			want: ErrTargetNotEmpty,
		},
		{
			name: "file",
			makeEntry: func(path string) error {
				return os.WriteFile(path, []byte("occupied"), 0o600)
			},
			want: ErrTargetExists,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			if err := test.makeEntry(filepath.Join(parent, "app")); err != nil {
				t.Fatal(err)
			}
			_, err := ResolveTarget(parent, "app", NameOptions{})
			if !errors.Is(err, test.want) {
				t.Fatalf("ResolveTarget() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestResolveTargetRejectsInvalidParents(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	file := filepath.Join(parent, "file")
	if err := os.WriteFile(file, []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", filepath.Join(parent, "missing"), file} {
		_, err := ResolveTarget(invalid, "app", NameOptions{})
		if !errors.Is(err, ErrInvalidParent) {
			t.Errorf("ResolveTarget(%q) error = %v, want ErrInvalidParent", invalid, err)
		}
	}
}

func TestResolveTargetRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows commonly requires elevated privileges")
	}

	parent := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(parent, "app")); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveTarget(parent, "app", NameOptions{})
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("ResolveTarget() error = %v, want ErrPathTraversal", err)
	}
}

func TestResolveTargetAllowsSafeInternalSymlinkToEmptyDirectory(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows commonly requires elevated privileges")
	}

	parent := t.TempDir()
	realTarget := filepath.Join(parent, "real")
	if err := os.Mkdir(realTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realTarget, filepath.Join(parent, "app")); err != nil {
		t.Fatal(err)
	}
	target, err := ResolveTarget(parent, "app", NameOptions{})
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}
	canonicalTarget, err := filepath.EvalSymlinks(realTarget)
	if err != nil {
		t.Fatal(err)
	}
	if target.ProjectDir != canonicalTarget || !target.Exists || !target.Empty {
		t.Fatalf("ResolveTarget() = %#v", target)
	}
}

func TestResolveTargetValidatesNameBeforeJoining(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	for _, name := range []string{"..", "../escape", `..\escape`} {
		_, err := ResolveTarget(parent, name, NameOptions{})
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("ResolveTarget(%q) error = %v, want ErrInvalidName", name, err)
		}
	}
}

func TestCheckWritableRejectsNonDirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckWritable(path); !errors.Is(err, ErrNotWritable) {
		t.Fatalf("CheckWritable() error = %v, want ErrNotWritable", err)
	}
}

func TestIsWithin(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(string(filepath.Separator), "tmp", "parent")
	tests := []struct {
		child string
		want  bool
	}{
		{child: filepath.Join(parent, "app"), want: true},
		{child: parent, want: false},
		{child: filepath.Dir(parent), want: false},
		{child: parent + "-sibling", want: false},
		{child: filepath.Join(parent, strings.Repeat("a", 10)), want: true},
	}
	for _, test := range tests {
		if got := isWithin(parent, test.child); got != test.want {
			t.Errorf("isWithin(%q, %q) = %t, want %t", parent, test.child, got, test.want)
		}
	}
}
