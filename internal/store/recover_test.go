package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeStoreFile writes raw bytes directly to a store's backing path, bypassing
// Save, so a test can stage a document this Store cannot read.
func writeStoreFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestVersionAccessorAndErrorMessage(t *testing.T) {
	t.Parallel()

	store, err := New(filepath.Join(t.TempDir(), "settings.json"), 3, testSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if store.Version() != 3 {
		t.Fatalf("Version() = %d, want 3", store.Version())
	}

	versionErr := &VersionError{Path: "/tmp/settings.json", Want: 3, Got: 1}
	message := versionErr.Error()
	for _, want := range []string{"/tmp/settings.json", "expected version 3", "found 1"} {
		if !strings.Contains(message, want) {
			t.Fatalf("Error() = %q, want it to contain %q", message, want)
		}
	}
	// Callers branch on the sentinel, not on the message.
	if !errors.Is(versionErr, ErrVersionMismatch) {
		t.Fatal("VersionError does not unwrap to ErrVersionMismatch")
	}
}

func TestLoadOrRecoverLeavesAHealthyDocumentAlone(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := New(path, 1, testSettings{Theme: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(testSettings{Theme: "dark", Recent: []string{"a"}}); err != nil {
		t.Fatal(err)
	}

	value, backup, err := store.LoadOrRecover()
	if err != nil {
		t.Fatalf("LoadOrRecover() error = %v", err)
	}
	if backup != "" {
		t.Fatalf("LoadOrRecover() backup = %q, want empty for a readable document", backup)
	}
	if value.Theme != "dark" {
		t.Fatalf("value = %#v, want the stored document", value)
	}
	if _, err := os.Stat(path + ".unreadable"); !os.IsNotExist(err) {
		t.Fatal("LoadOrRecover wrote a backup for a healthy document")
	}
}

// A missing file is not a failure: Load already falls back to defaults, so
// nothing is preserved and no error is reported.
func TestLoadOrRecoverTreatsAMissingFileAsDefaults(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := New(path, 1, testSettings{Theme: "default"})
	if err != nil {
		t.Fatal(err)
	}

	value, backup, err := store.LoadOrRecover()
	if err != nil {
		t.Fatalf("LoadOrRecover() error = %v", err)
	}
	if backup != "" {
		t.Fatalf("backup = %q, want empty", backup)
	}
	if value.Theme != "default" {
		t.Fatalf("value = %#v, want the default", value)
	}
}

// The case T-14 exists for. Before LoadOrRecover, any of these left settings,
// presets, and history permanently unreachable: nothing in the application could
// rewrite a document it refused to read.
func TestLoadOrRecoverPreservesUnreadableDocumentsAndFallsBackToDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "version from a future release",
			content: `{"version": 99, "data": {"theme": "dark", "recent": [], "aliases": {}}}`,
		},
		{
			name:    "version from an older release",
			content: `{"version": 1, "data": {"theme": "dark", "recent": [], "aliases": {}}}`,
		},
		{
			name:    "truncated file",
			content: `{"version": 2, "data": {"theme": "da`,
		},
		{
			name:    "not JSON at all",
			content: "\x00\x01 not json",
		},
		{
			name:    "empty file",
			content: "",
		},
		{
			// Strict decoding rejects unknown fields, so a document written by a
			// newer build is unreadable even at a matching version.
			name:    "unknown field",
			content: `{"version": 2, "data": {"theme": "dark", "recent": [], "aliases": {}, "future": 1}}`,
		},
		{
			name:    "trailing JSON value",
			content: `{"version": 2, "data": {"theme": "dark", "recent": [], "aliases": {}}} {"extra": true}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "settings.json")
			writeStoreFile(t, path, test.content)
			store, err := New(path, 2, testSettings{Theme: "default"})
			if err != nil {
				t.Fatal(err)
			}

			value, backup, err := store.LoadOrRecover()
			if err != nil {
				t.Fatalf("LoadOrRecover() error = %v, want a recovered fallback", err)
			}
			if value.Theme != "default" {
				t.Fatalf("value = %#v, want the default value", value)
			}
			if backup != path+".unreadable" {
				t.Fatalf("backup = %q, want %q", backup, path+".unreadable")
			}

			// The original bytes must survive verbatim: recovery is meant to be
			// undoable by hand, not a silent delete.
			preserved, err := os.ReadFile(backup)
			if err != nil {
				t.Fatalf("read preserved document: %v", err)
			}
			if string(preserved) != test.content {
				t.Fatalf("preserved = %q, want the original bytes %q", preserved, test.content)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("the unreadable document was left in place as well as preserved")
			}
		})
	}
}

// Recovery has to actually unblock the store, not just report a fallback.
func TestLoadOrRecoverLeavesTheStoreWritable(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	writeStoreFile(t, path, `{"version": 99, "data": {}}`)
	store, err := New(path, 2, testSettings{Theme: "default"})
	if err != nil {
		t.Fatal(err)
	}

	if _, backup, err := store.LoadOrRecover(); err != nil || backup == "" {
		t.Fatalf("LoadOrRecover() = %q, %v", backup, err)
	}
	if err := store.Save(testSettings{Theme: "light"}); err != nil {
		t.Fatalf("Save() after recovery error = %v", err)
	}
	value, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after recovery error = %v", err)
	}
	if value.Theme != "light" {
		t.Fatalf("value = %#v, want the document written after recovery", value)
	}

	// A second recovery pass has nothing left to preserve.
	if _, backup, err := store.LoadOrRecover(); err != nil || backup != "" {
		t.Fatalf("second LoadOrRecover() = %q, %v, want no further recovery", backup, err)
	}
}

// When the document cannot be moved aside, the original load failure has to
// survive rather than be replaced by the rename error: the caller still needs to
// know why the document was unusable.
func TestLoadOrRecoverReportsBothFailuresWhenItCannotPreserve(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not prevent rename on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "settings.json")
	writeStoreFile(t, path, `{"version": 99, "data": {}}`)
	store, err := New(path, 2, testSettings{Theme: "default"})
	if err != nil {
		t.Fatal(err)
	}

	// Removing write permission on the parent blocks the rename.
	if err := os.Chmod(directory, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(directory, 0o700)
	})

	_, backup, err := store.LoadOrRecover()
	if err == nil {
		t.Fatal("LoadOrRecover() error = nil, want the failure reported")
	}
	if backup != "" {
		t.Fatalf("backup = %q, want empty when nothing was preserved", backup)
	}
	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("error = %v, want it to still unwrap to ErrVersionMismatch", err)
	}
	if !strings.Contains(err.Error(), "could not be preserved") {
		t.Fatalf("error = %v, want it to explain that preservation failed", err)
	}
}

type unclonable struct {
	Content any `json:"content"`
}

// A default value that cannot be cloned is a programming error, not a user
// state. It must surface rather than hand out an aliased or zero value.
func TestCloneFailuresSurface(t *testing.T) {
	t.Parallel()

	t.Run("load of a missing file", func(t *testing.T) {
		t.Parallel()
		store, err := New(
			filepath.Join(t.TempDir(), "value.json"),
			1,
			unclonable{Content: make(chan int)},
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(); err == nil ||
			!strings.Contains(err.Error(), "clone default store value") {
			t.Fatalf("Load() error = %v, want a clone failure", err)
		}
	})

	t.Run("recovery fallback", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "value.json")
		writeStoreFile(t, path, `{"version": 99, "data": {"content": null}}`)
		store, err := New(path, 1, unclonable{Content: make(chan int)})
		if err != nil {
			t.Fatal(err)
		}

		_, backup, err := store.LoadOrRecover()
		if err == nil || !strings.Contains(err.Error(), "clone default store value") {
			t.Fatalf("LoadOrRecover() error = %v, want a clone failure", err)
		}
		// The document was already moved aside, so the path must still be
		// reported even though the fallback could not be produced.
		if backup != path+".unreadable" {
			t.Fatalf("backup = %q, want the preserved path reported anyway", backup)
		}
	})
}

func TestSaveReportsAnUnusableStoreDirectory(t *testing.T) {
	t.Parallel()

	// A regular file where a directory component belongs.
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New(filepath.Join(blocker, "settings.json"), 1, testSettings{})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Save(testSettings{Theme: "dark"}); err == nil ||
		!strings.Contains(err.Error(), "create store directory") {
		t.Fatalf("Save() error = %v, want a directory-creation failure", err)
	}
}

func TestUpdateRejectsANilChangeFunction(t *testing.T) {
	t.Parallel()

	store, err := New(filepath.Join(t.TempDir(), "settings.json"), 1, testSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(nil); err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("Update(nil) error = %v, want a validation error", err)
	}
}

// Update must not run the change function, and must not write, when the current
// document cannot be read. Otherwise a mutation would be applied to defaults and
// silently overwrite an unreadable-but-recoverable file.
func TestUpdateDoesNotRunOrWriteWhenTheDocumentIsUnreadable(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	original := `{"version": 99, "data": {"theme": "dark", "recent": [], "aliases": {}}}`
	writeStoreFile(t, path, original)
	store, err := New(path, 2, testSettings{})
	if err != nil {
		t.Fatal(err)
	}

	called := false
	err = store.Update(func(value *testSettings) error {
		called = true
		value.Theme = "light"
		return nil
	})
	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("Update() error = %v, want ErrVersionMismatch", err)
	}
	if called {
		t.Fatal("Update ran the change function against an unreadable document")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Fatalf("Update rewrote an unreadable document\nbefore: %s\nafter:  %s", original, after)
	}
}

// The store directory exists but is not writable, so MkdirAll succeeds and the
// temporary file cannot be created. Nothing may be left behind.
func TestSaveReportsAnUnwritableStoreDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not prevent file creation on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	t.Parallel()

	directory := t.TempDir()
	store, err := New(filepath.Join(directory, "settings.json"), 1, testSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(directory, 0o700)
	})

	if err := store.Save(testSettings{Theme: "dark"}); err == nil ||
		!strings.Contains(err.Error(), "create temporary store file") {
		t.Fatalf("Save() error = %v, want a temporary-file failure", err)
	}

	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("directory contains %#v, want no partial write left behind", entries)
	}
}
