package toolchain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	// ErrBinaryNotAllowed is returned when a caller asks Nodesmith to resolve a
	// binary that is not part of its fixed executable allowlist.
	ErrBinaryNotAllowed = errors.New("binary is not allowlisted")
	// ErrBinaryNotFound is returned when an allowlisted binary is not present on
	// the resolved PATH.
	ErrBinaryNotFound = errors.New("binary not found")
)

var allowedBinaries = map[string]struct{}{
	"bun":   {},
	"bunx":  {},
	"cargo": {},
	"code":  {},
	"gh":    {},
	"git":   {},
	"go":    {},
	"node":  {},
	"npm":   {},
	"npx":   {},
	"pnpm":  {},
	"pnpx":  {},
	"wails": {},
	"yarn":  {},
}

var detectedBinaries = []string{
	"node",
	"npm",
	"npx",
	"pnpm",
	"yarn",
	"bun",
	"git",
	"go",
	"cargo",
	"wails",
	"gh",
	"code",
}

// AllowedBinaries returns a sorted copy of the executable allowlist.
func AllowedBinaries() []string {
	names := make([]string, 0, len(allowedBinaries))
	for name := range allowedBinaries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DetectedBinaries returns the stable list of tools included in a toolchain
// scan. pnpx and bunx are resolvable helpers but are not reported separately.
func DetectedBinaries() []string {
	return append([]string(nil), detectedBinaries...)
}

// IsAllowed reports whether name is an allowlisted logical binary name. Paths,
// extensions, and alternate casing are deliberately rejected.
func IsAllowed(name string) bool {
	if name != strings.ToLower(name) {
		return false
	}
	_, ok := allowedBinaries[name]
	return ok
}

func validateBinaryName(name string) error {
	if !IsAllowed(name) {
		return fmt.Errorf("%w: %q", ErrBinaryNotAllowed, name)
	}
	return nil
}
