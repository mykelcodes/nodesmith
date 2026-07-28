package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

type testSettings struct {
	Theme   string            `json:"theme"`
	Recent  []string          `json:"recent"`
	Aliases map[string]string `json:"aliases"`
}

func TestNewValidatesAndCanonicalizesPath(t *testing.T) {
	t.Parallel()

	if _, err := New[testSettings]("", 1, testSettings{}); err == nil {
		t.Fatal("New() accepted an empty path")
	}
	if _, err := New("settings.json", 0, testSettings{}); !errors.Is(err, ErrInvalidVersion) {
		t.Fatalf("New() error = %v, want ErrInvalidVersion", err)
	}
	store, err := New("settings.json", 1, testSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(store.Path()) {
		t.Fatalf("Path() = %q, want absolute", store.Path())
	}
}

func TestLoadMissingReturnsDeepCopyOfDefault(t *testing.T) {
	t.Parallel()

	defaults := testSettings{
		Theme:   "system",
		Recent:  []string{"one"},
		Aliases: map[string]string{"code": "cursor"},
	}
	store, err := New(filepath.Join(t.TempDir(), "settings.json"), 1, defaults)
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	first.Recent[0] = "mutated"
	first.Aliases["code"] = "mutated"
	second, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if second.Recent[0] != "one" || second.Aliases["code"] != "cursor" {
		t.Fatalf("default value was aliased: %#v", second)
	}
}

func TestSaveRoundTripVersionAndPermissions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	store, err := New(path, 3, testSettings{})
	if err != nil {
		t.Fatal(err)
	}
	want := testSettings{
		Theme:   "dark",
		Recent:  []string{"/tmp/one", "/tmp/two"},
		Aliases: map[string]string{"code": "zed"},
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if fmt.Sprintf("%#v", got) != fmt.Sprintf("%#v", want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document Document[testSettings]
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("saved JSON does not parse: %v\n%s", err, raw)
	}
	if document.Version != 3 {
		t.Fatalf("version = %d, want 3", document.Version)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if gotMode := info.Mode().Perm(); gotMode != 0o600 {
			t.Fatalf("file mode = %o, want 600", gotMode)
		}
	}
}

func TestFailedEncodingPreservesPreviousDocument(t *testing.T) {
	t.Parallel()

	type value struct {
		Content any `json:"content"`
	}
	path := filepath.Join(t.TempDir(), "history.json")
	store, err := New(path, 1, value{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(value{Content: "safe"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(value{Content: make(chan int)}); err == nil {
		t.Fatal("Save() accepted a channel")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("failed Save() changed file\nbefore: %s\nafter: %s", before, after)
	}
}

func TestOrphanTemporaryFileDoesNotAffectLastCommittedDocument(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "presets.json")
	store, err := New(path, 1, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save([]string{"committed"}); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(directory, ".presets.json.tmp-interrupted")
	if err := os.WriteFile(orphan, []byte(`{"version":1,"data":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 1 || got[0] != "committed" {
		t.Fatalf("Load() = %#v", got)
	}
}

func TestLoadRejectsWrongVersionUnknownFieldsAndTrailingJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		wantVersion bool
	}{
		{
			name:        "wrong version",
			content:     `{"version":2,"data":{"theme":"dark","recent":[],"aliases":{}}}`,
			wantVersion: true,
		},
		{
			name:    "unknown field",
			content: `{"version":1,"data":{"theme":"dark","recent":[],"aliases":{},"extra":true}}`,
		},
		{
			name:    "trailing value",
			content: `{"version":1,"data":{"theme":"dark","recent":[],"aliases":{}}} {}`,
		},
		{
			name:    "malformed",
			content: `{"version":1,"data":`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			store, err := New(path, 1, testSettings{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = store.Load()
			if err == nil {
				t.Fatal("Load() returned no error")
			}
			if test.wantVersion {
				if !errors.Is(err, ErrVersionMismatch) {
					t.Fatalf("Load() error = %v, want ErrVersionMismatch", err)
				}
				var versionErr *VersionError
				if !errors.As(err, &versionErr) || versionErr.Got != 2 || versionErr.Want != 1 {
					t.Fatalf("VersionError = %#v", versionErr)
				}
			}
		})
	}
}

func TestUpdateIsAtomicAcrossGoroutines(t *testing.T) {
	t.Parallel()

	type counter struct {
		Value int `json:"value"`
	}
	store, err := New(filepath.Join(t.TempDir(), "history.json"), 1, counter{})
	if err != nil {
		t.Fatal(err)
	}
	const updates = 50
	var wait sync.WaitGroup
	errorsSeen := make(chan error, updates)
	for range updates {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsSeen <- store.Update(func(value *counter) error {
				value.Value++
				return nil
			})
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Errorf("Update() error = %v", err)
		}
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != updates {
		t.Fatalf("counter = %d, want %d", got.Value, updates)
	}
}

func TestUpdateErrorDoesNotWrite(t *testing.T) {
	t.Parallel()

	store, err := New(filepath.Join(t.TempDir(), "settings.json"), 1, testSettings{
		Theme: "system",
	})
	if err != nil {
		t.Fatal(err)
	}
	changeErr := errors.New("reject change")
	err = store.Update(func(value *testSettings) error {
		value.Theme = "dark"
		return changeErr
	})
	if !errors.Is(err, changeErr) {
		t.Fatalf("Update() error = %v", err)
	}
	if _, statErr := os.Stat(store.Path()); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed Update() created a file: %v", statErr)
	}
}

func TestConcurrentAtomicReplacementAlwaysLeavesParseableJSON(t *testing.T) {
	t.Parallel()

	type sequence struct {
		Value int `json:"value"`
	}
	path := filepath.Join(t.TempDir(), "history.json")
	store, err := New(path, 1, sequence{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(sequence{}); err != nil {
		t.Fatal(err)
	}

	const writes = 40
	done := make(chan struct{})
	readErrors := make(chan error, 1)
	go func() {
		defer close(done)
		for index := 0; index < writes*4; index++ {
			content, err := os.ReadFile(path)
			if err != nil {
				readErrors <- err
				return
			}
			var document Document[sequence]
			if err := json.Unmarshal(content, &document); err != nil {
				readErrors <- err
				return
			}
		}
	}()
	for index := 1; index <= writes; index++ {
		if err := store.Save(sequence{Value: index}); err != nil {
			t.Fatal(err)
		}
	}
	<-done
	select {
	case err := <-readErrors:
		t.Fatalf("reader observed a partial document: %v", err)
	default:
	}
}
