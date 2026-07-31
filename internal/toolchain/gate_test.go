package toolchain

import (
	"reflect"
	"testing"
)

func TestEvaluateRequirements(t *testing.T) {
	t.Parallel()

	toolchain := Toolchain{
		Tools: map[string]Tool{
			"node": {Name: "node", Present: true, Version: "20.11.1"},
			"npm":  {Name: "npm", Present: true, Version: "10.2.4"},
			"git":  {Name: "git", Present: true, Version: "2.45.2"},
		},
	}
	result := EvaluateRequirements(toolchain, Requirements{
		Node:            ">=20.0.0",
		PackageManagers: []string{"pnpm", "npm"},
		Tools:           []string{"git"},
	})
	if !result.Available {
		t.Fatalf("EvaluateRequirements() = %#v, want available", result)
	}
	if result.PackageManager != "npm" {
		t.Fatalf("PackageManager = %q, want npm", result.PackageManager)
	}
	if len(result.Reasons) != 0 {
		t.Fatalf("Reasons = %#v", result.Reasons)
	}
}

func TestEvaluateRequirementsReportsEveryBlockingReason(t *testing.T) {
	t.Parallel()

	toolchain := Toolchain{
		Tools: map[string]Tool{
			"node": {Name: "node", Present: true, Version: "18.20.4"},
		},
	}
	result := EvaluateRequirements(toolchain, Requirements{
		Node:            ">=20.0.0",
		PackageManagers: []string{"pnpm", "npm"},
		Tools:           []string{"git", "cargo"},
	})
	want := []string{
		"node 18.20.4 does not satisfy required version >=20.0.0",
		"required tool git was not found",
		"required tool cargo was not found",
		"no supported package manager is usable: pnpm (not found), npm (not found)",
	}
	if result.Available {
		t.Fatal("EvaluateRequirements() unexpectedly available")
	}
	if !reflect.DeepEqual(result.Reasons, want) {
		t.Fatalf("Reasons = %#v, want %#v", result.Reasons, want)
	}
}

func TestEvaluateRequirementsRejectsUninvocableTools(t *testing.T) {
	t.Parallel()

	// Detection reports a tool that resolved on PATH but cannot be invoked —
	// a Windows package-manager shim with no JavaScript entrypoint — as not
	// present, carrying the reason.
	detected := Toolchain{
		Tools: map[string]Tool{
			"node":  {Name: "node", Present: true, Version: "22.1.0"},
			"npm":   {Name: "npm", Error: "npm shim could not be invoked"},
			"pnpm":  {Name: "pnpm", Error: "pnpm shim could not be invoked"},
			"git":   {Name: "git", Error: "git shim could not be invoked"},
			"cargo": {Name: "cargo", Present: true, Version: "1.80.0"},
		},
	}
	result := EvaluateRequirements(detected, Requirements{
		Node:            ">=20.0.0",
		PackageManagers: []string{"npm", "pnpm"},
		Tools:           []string{"git", "cargo"},
	})
	want := []string{
		"required tool git could not be used: git shim could not be invoked",
		"no supported package manager is usable: " +
			"npm (npm shim could not be invoked), pnpm (pnpm shim could not be invoked)",
	}
	if result.Available {
		t.Fatal("EvaluateRequirements() unexpectedly available")
	}
	if !reflect.DeepEqual(result.Reasons, want) {
		t.Fatalf("Reasons = %#v, want %#v", result.Reasons, want)
	}
}

func TestEvaluateRequirementsAllowsPresentToolsWithUnprobedVersion(t *testing.T) {
	t.Parallel()

	// A slow first run leaves a tool present with no version: corepack
	// downloads the package manager on first invocation, and on-access virus
	// scanning adds seconds to a cold launch. requires.tools and
	// requires.packageManagers are presence checks, so these must still be
	// satisfied rather than silently disabling the recipe.
	detected := Toolchain{
		Tools: map[string]Tool{
			"node":  {Name: "node", Present: true, Version: "22.1.0"},
			"pnpm":  {Name: "pnpm", Present: true, Error: "pnpm version probe failed"},
			"git":   {Name: "git", Present: true},
			"cargo": {Name: "cargo", Present: true, Error: "cargo version probe failed"},
		},
	}
	result := EvaluateRequirements(detected, Requirements{
		Node:            ">=20.0.0",
		PackageManagers: []string{"pnpm"},
		Tools:           []string{"git", "cargo"},
	})
	if !result.Available {
		t.Fatalf("EvaluateRequirements() = %#v, want available", result)
	}
	if result.PackageManager != "pnpm" {
		t.Fatalf("PackageManager = %q, want %q", result.PackageManager, "pnpm")
	}
}

func TestEvaluateRequirementsMissingNode(t *testing.T) {
	t.Parallel()

	result := EvaluateRequirements(Toolchain{Tools: map[string]Tool{}}, Requirements{
		Node: ">=20.0.0",
	})
	if result.Available || len(result.Reasons) != 1 {
		t.Fatalf("EvaluateRequirements() = %#v", result)
	}
}
