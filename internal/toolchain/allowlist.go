package toolchain

import (
	"errors"

	"nodesmith/internal/allowlist"
)

var (
	// ErrBinaryNotAllowed is returned when a caller asks Nodesmith to resolve a
	// binary that is not part of its fixed executable allowlist.
	ErrBinaryNotAllowed = errors.New("binary is not allowlisted")
	// ErrBinaryNotFound is returned when an allowlisted binary is not present on
	// the resolved PATH.
	ErrBinaryNotFound = errors.New("binary not found")
)

// AllowedBinaries returns a sorted copy of the executable allowlist. The list
// itself is owned by internal/allowlist so that recipe validation and binary
// resolution cannot drift apart.
func AllowedBinaries() []string {
	return allowlist.Names()
}

// DetectedBinaries returns the stable list of tools included in a toolchain
// scan.
func DetectedBinaries() []string {
	return allowlist.Detected()
}

// IsAllowed reports whether name is an allowlisted logical binary name.
func IsAllowed(name string) bool {
	return allowlist.IsAllowed(name)
}
