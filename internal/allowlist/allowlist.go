// Package allowlist owns the fixed set of executables Nodesmith is willing to
// name or run.
//
// This list is the most security-relevant constant in the project, so it has
// exactly one definition. It previously lived in both internal/recipe (which
// decides whether a manifest may declare a binary) and internal/toolchain
// (which decides whether a binary may be resolved and executed). The two copies
// agreed, but nothing enforced that: adding a tool to one and not the other
// yields either a recipe that validates and then fails at resolve time, or a
// resolvable binary no recipe is allowed to declare.
//
// The package deliberately holds no behaviour beyond membership so that both
// consumers can depend on it without either depending on the other.
package allowlist

import (
	"slices"
	"strings"
)

// binaries is the single source of truth. Everything else in the repo derives
// from it.
var binaries = map[string]struct{}{
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

// detected is the stable subset reported by a toolchain scan. pnpx and bunx are
// resolvable helpers that are not surfaced as separate tools.
var detected = []string{
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

// Names returns a sorted copy of the executable allowlist.
func Names() []string {
	names := make([]string, 0, len(binaries))
	for name := range binaries {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Detected returns a copy of the tools included in a toolchain scan.
func Detected() []string {
	return slices.Clone(detected)
}

// IsAllowed reports whether name is an allowlisted logical binary name. Paths,
// extensions, and alternate casing are deliberately rejected: the caller must
// present the bare lowercase name, and resolution to an absolute path happens
// afterwards.
func IsAllowed(name string) bool {
	if name != strings.ToLower(name) {
		return false
	}
	_, ok := binaries[name]
	return ok
}
