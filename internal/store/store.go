// Package store provides a small, versioned, atomic JSON file store.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	ErrInvalidVersion  = errors.New("store version must be positive")
	ErrVersionMismatch = errors.New("store schema version mismatch")
)

// Document is the on-disk schema shared by settings, presets, and history.
type Document[T any] struct {
	Version int `json:"version"`
	Data    T   `json:"data"`
}

// VersionError reports an on-disk schema version that this Store cannot read.
type VersionError struct {
	Path string
	Want int
	Got  int
}

func (err *VersionError) Error() string {
	return fmt.Sprintf(
		"%s: %v: expected version %d, found %d",
		err.Path,
		ErrVersionMismatch,
		err.Want,
		err.Got,
	)
}

func (err *VersionError) Unwrap() error {
	return ErrVersionMismatch
}

// Store reads and atomically replaces a single JSON document.
type Store[T any] struct {
	path         string
	version      int
	defaultValue T
	mu           sync.Mutex
}

// New creates a typed store. It performs no filesystem IO until Load, Save, or
// Update is called.
func New[T any](path string, version int, defaultValue T) (*Store[T], error) {
	if path == "" {
		return nil, errors.New("store path is empty")
	}
	if version <= 0 {
		return nil, ErrInvalidVersion
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("make store path absolute: %w", err)
	}
	return &Store[T]{
		path:         absolute,
		version:      version,
		defaultValue: defaultValue,
	}, nil
}

// Path returns the absolute backing-file path.
func (store *Store[T]) Path() string {
	return store.path
}

// Version returns the schema version written by this store.
func (store *Store[T]) Version() int {
	return store.version
}

// Load reads and strictly decodes the document. A missing file returns a deep
// copy of the supplied default value.
func (store *Store[T]) Load() (T, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.load()
}

func (store *Store[T]) load() (T, error) {
	var zero T
	content, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		value, cloneErr := cloneJSON(store.defaultValue)
		if cloneErr != nil {
			return zero, fmt.Errorf("clone default store value: %w", cloneErr)
		}
		return value, nil
	}
	if err != nil {
		return zero, fmt.Errorf("read store %q: %w", store.path, err)
	}

	var document Document[T]
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return zero, fmt.Errorf("decode store %q: %w", store.path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return zero, fmt.Errorf("decode store %q: unexpected trailing JSON value", store.path)
	}
	if document.Version != store.version {
		return zero, &VersionError{
			Path: store.path,
			Want: store.version,
			Got:  document.Version,
		}
	}
	return document.Data, nil
}

// LoadOrRecover reads the document, and when the stored copy cannot be used it
// preserves the original alongside the store and returns the default value.
//
// A version mismatch or a corrupt file is otherwise a dead end: nothing in the
// application can rewrite an unreadable document, so settings, presets, and
// history would stay permanently unreachable. Moving the file aside keeps the
// data recoverable by hand while letting the application start.
//
// The returned string is the path the unreadable document was preserved at, and
// is empty when the document loaded normally.
func (store *Store[T]) LoadOrRecover() (T, string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	value, err := store.load()
	if err == nil {
		return value, "", nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return value, "", err
	}

	backupPath := store.path + ".unreadable"
	if renameErr := os.Rename(store.path, backupPath); renameErr != nil {
		return value, "", fmt.Errorf("%w (and it could not be preserved: %v)", err, renameErr)
	}
	fallback, cloneErr := cloneJSON(store.defaultValue)
	if cloneErr != nil {
		return value, backupPath, fmt.Errorf("clone default store value: %w", cloneErr)
	}
	return fallback, backupPath, nil
}

// Save serializes the entire document before touching the filesystem, then
// writes, syncs, and renames a temporary file in the destination directory.
func (store *Store[T]) Save(value T) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.save(value)
}

func (store *Store[T]) save(value T) error {
	document := Document[T]{
		Version: store.version,
		Data:    value,
	}
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store %q: %w", store.path, err)
	}
	content = append(content, '\n')

	directory := filepath.Dir(store.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create store directory %q: %w", directory, err)
	}
	temp, err := os.CreateTemp(directory, "."+filepath.Base(store.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary store file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}

	if err := temp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("set temporary store permissions: %w", err)
	}
	if _, err := temp.Write(content); err != nil {
		cleanup()
		return fmt.Errorf("write temporary store file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary store file: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close temporary store file: %w", err)
	}
	if err := os.Rename(tempPath, store.path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace store %q atomically: %w", store.path, err)
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("sync store directory %q: %w", directory, err)
	}
	return nil
}

// Update holds the store lock across read, mutation, and atomic replacement.
func (store *Store[T]) Update(change func(*T) error) error {
	if change == nil {
		return errors.New("store update function is nil")
	}
	store.mu.Lock()
	defer store.mu.Unlock()

	value, err := store.load()
	if err != nil {
		return err
	}
	if err := change(&value); err != nil {
		return err
	}
	return store.save(value)
}

func cloneJSON[T any](source T) (T, error) {
	var clone T
	content, err := json.Marshal(source)
	if err != nil {
		return clone, err
	}
	if err := json.Unmarshal(content, &clone); err != nil {
		return clone, err
	}
	return clone, nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil && runtime.GOOS != "windows" {
		return errors.Join(syncErr, closeErr)
	}
	return closeErr
}
