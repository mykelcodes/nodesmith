package allowlist

import (
	"reflect"
	"slices"
	"testing"
)

func TestNames(t *testing.T) {
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
	got := Names()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %#v, want %#v", got, want)
	}

	got[0] = "changed"
	if Names()[0] != "bun" {
		t.Fatal("Names returned mutable package state")
	}
}

func TestIsAllowed(t *testing.T) {
	t.Parallel()

	for _, name := range Names() {
		if !IsAllowed(name) {
			t.Fatalf("IsAllowed(%q) = false, want true", name)
		}
	}

	// Only the bare lowercase name is accepted. Paths, extensions, and alternate
	// casing are rejected here so that resolution to an absolute path is the
	// only thing that can turn a name into something executable.
	for _, name := range []string{
		"Node",
		"NODE",
		"node.exe",
		"/usr/bin/node",
		`C:\Windows\System32\node.exe`,
		"../node",
		"node ",
		"",
		"sh",
		"bash",
		"powershell",
		"curl",
	} {
		if IsAllowed(name) {
			t.Fatalf("IsAllowed(%q) = true, want false", name)
		}
	}
}

func TestDetected(t *testing.T) {
	t.Parallel()

	got := Detected()
	if len(got) == 0 {
		t.Fatal("Detected() is empty")
	}

	// A scan may only report tools the resolver is allowed to run.
	for _, name := range got {
		if !IsAllowed(name) {
			t.Fatalf("detected binary %q is not allowlisted", name)
		}
	}

	// pnpx and bunx resolve but are not surfaced as separate tools.
	for _, helper := range []string{"pnpx", "bunx"} {
		if slices.Contains(got, helper) {
			t.Fatalf("Detected() includes the helper %q, which is not reported separately", helper)
		}
	}

	got[0] = "changed"
	if Detected()[0] == "changed" {
		t.Fatal("Detected returned mutable package state")
	}
}
