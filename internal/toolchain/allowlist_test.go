package toolchain

import (
	"errors"
	"reflect"
	"testing"

	"nodesmith/internal/allowlist"
)

func TestAllowedBinaries(t *testing.T) {
	t.Parallel()

	want := []string{
		"bun",
		"bunx",
		"cargo",
		"code",
		"gh",
		"git",
		"go",
		"node",
		"npm",
		"npx",
		"pnpm",
		"pnpx",
		"wails",
		"yarn",
	}
	got := AllowedBinaries()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllowedBinaries() = %#v, want %#v", got, want)
	}

	got[0] = "changed"
	if AllowedBinaries()[0] != "bun" {
		t.Fatal("AllowedBinaries returned mutable package state")
	}
}

func TestResolveRejectsAnythingExceptLogicalAllowlistNames(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(NewPathResolver())
	for _, name := range []string{"Node", "node.exe", "/usr/bin/node", "../node", "", "sh"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := resolver.Resolve(name); !errors.Is(err, ErrBinaryNotAllowed) {
				t.Fatalf("Resolve(%q) error = %v, want ErrBinaryNotAllowed", name, err)
			}
		})
	}
}

// The allowlist has one owner. A recipe that may declare a binary must be a
// binary the resolver may run, and vice versa: drift between the two is the
// failure this guards.
func TestAllowlistIsSharedWithRecipeValidation(t *testing.T) {
	t.Parallel()

	for _, name := range AllowedBinaries() {
		if !allowlist.IsAllowed(name) {
			t.Fatalf("toolchain allows %q but the shared allowlist does not", name)
		}
	}
	for _, name := range allowlist.Names() {
		if !IsAllowed(name) {
			t.Fatalf("the shared allowlist allows %q but toolchain does not", name)
		}
	}
	for _, name := range DetectedBinaries() {
		if !IsAllowed(name) {
			t.Fatalf("detected binary %q is not allowlisted", name)
		}
	}
}
