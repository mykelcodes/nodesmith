package toolchain

import (
	"errors"
	"reflect"
	"testing"
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

func TestValidateBinaryNameRejectsAnythingExceptLogicalAllowlistNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"Node", "node.exe", "/usr/bin/node", "../node", "", "sh"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := validateBinaryName(name); !errors.Is(err, ErrBinaryNotAllowed) {
				t.Fatalf("validateBinaryName(%q) error = %v, want ErrBinaryNotAllowed", name, err)
			}
		})
	}
}
