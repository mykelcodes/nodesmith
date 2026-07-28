package project

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Target is a canonical, checked project destination.
type Target struct {
	ParentDir  string `json:"parentDir"`
	ProjectDir string `json:"projectDir"`
	Exists     bool   `json:"exists"`
	Empty      bool   `json:"empty"`
}

// ResolveTarget validates a name and parent, canonicalizes the target path,
// rejects symlink escapes and non-empty collisions, and returns an absolute
// destination. It performs no write probe.
func ResolveTarget(
	parentDir string,
	projectName string,
	nameOptions NameOptions,
) (Target, error) {
	if err := ValidateName(projectName, nameOptions); err != nil {
		return Target{}, err
	}
	if parentDir == "" {
		return Target{}, fmt.Errorf("%w: path is empty", ErrInvalidParent)
	}

	absoluteParent, err := filepath.Abs(parentDir)
	if err != nil {
		return Target{}, fmt.Errorf("%w: make path absolute: %w", ErrInvalidParent, err)
	}
	parentInfo, err := os.Stat(absoluteParent)
	if err != nil {
		return Target{}, fmt.Errorf("%w: inspect %q: %w", ErrInvalidParent, absoluteParent, err)
	}
	if !parentInfo.IsDir() {
		return Target{}, fmt.Errorf("%w: %q is not a directory", ErrInvalidParent, absoluteParent)
	}
	canonicalParent, err := filepath.EvalSymlinks(absoluteParent)
	if err != nil {
		return Target{}, fmt.Errorf("%w: resolve %q: %w", ErrInvalidParent, absoluteParent, err)
	}
	canonicalParent, err = filepath.Abs(canonicalParent)
	if err != nil {
		return Target{}, fmt.Errorf("%w: canonicalize %q: %w", ErrInvalidParent, parentDir, err)
	}

	projectDir := filepath.Join(canonicalParent, projectName)
	if !isWithin(canonicalParent, projectDir) {
		return Target{}, fmt.Errorf("%w: %q", ErrPathTraversal, projectDir)
	}

	target := Target{
		ParentDir:  canonicalParent,
		ProjectDir: projectDir,
	}
	linkInfo, err := os.Lstat(projectDir)
	if errors.Is(err, os.ErrNotExist) {
		return target, nil
	}
	if err != nil {
		return Target{}, fmt.Errorf("inspect project target %q: %w", projectDir, err)
	}
	target.Exists = true

	checkedPath := projectDir
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		checkedPath, err = filepath.EvalSymlinks(projectDir)
		if err != nil {
			return Target{}, fmt.Errorf("%w: target symlink cannot be resolved: %w", ErrTargetExists, err)
		}
		checkedPath, err = filepath.Abs(checkedPath)
		if err != nil {
			return Target{}, fmt.Errorf("make target symlink absolute: %w", err)
		}
		if !isWithin(canonicalParent, checkedPath) {
			return Target{}, fmt.Errorf("%w: target symlink resolves to %q", ErrPathTraversal, checkedPath)
		}
		target.ProjectDir = checkedPath
	}

	info, err := os.Stat(checkedPath)
	if err != nil {
		return Target{}, fmt.Errorf("inspect project target %q: %w", checkedPath, err)
	}
	if !info.IsDir() {
		return Target{}, fmt.Errorf("%w: %q is not a directory", ErrTargetExists, checkedPath)
	}

	empty, err := directoryEmpty(checkedPath)
	if err != nil {
		return Target{}, fmt.Errorf("inspect project target contents: %w", err)
	}
	target.Empty = empty
	if !empty {
		return Target{}, fmt.Errorf("%w: %q", ErrTargetNotEmpty, checkedPath)
	}
	return target, nil
}

// ValidateTarget performs ResolveTarget and an actual create/remove write
// probe in the directory the generator will need to modify.
func ValidateTarget(
	parentDir string,
	projectName string,
	nameOptions NameOptions,
) (Target, error) {
	target, err := ResolveTarget(parentDir, projectName, nameOptions)
	if err != nil {
		return Target{}, err
	}
	probeDir := target.ParentDir
	if target.Exists {
		probeDir = target.ProjectDir
	}
	if err := CheckWritable(probeDir); err != nil {
		return Target{}, err
	}
	return target, nil
}

// CheckWritable verifies effective filesystem permissions with a temporary
// file and always attempts to remove the probe.
func CheckWritable(directory string) error {
	file, err := os.CreateTemp(directory, ".nodesmith-write-check-*")
	if err != nil {
		return fmt.Errorf("%w: create probe in %q: %w", ErrNotWritable, directory, err)
	}
	name := file.Name()
	closeErr := file.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return fmt.Errorf("%w: close probe in %q: %w", ErrNotWritable, directory, closeErr)
	}
	if removeErr != nil {
		return fmt.Errorf("%w: remove probe %q: %w", ErrNotWritable, name, removeErr)
	}
	return nil
}

func directoryEmpty(path string) (bool, error) {
	directory, err := os.Open(path)
	if err != nil {
		return false, err
	}

	_, readErr := directory.Readdirnames(1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return false, errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return false, closeErr
	}
	if errors.Is(readErr, io.EOF) {
		return true, nil
	}
	return false, nil
}

func isWithin(parent string, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil || relative == "." || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !startsWithParentTraversal(relative)
}

func startsWithParentTraversal(path string) bool {
	separator := string(filepath.Separator)
	return len(path) >= 3 && path[:3] == ".."+separator
}
