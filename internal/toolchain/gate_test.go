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

func TestEvaluateRequirementsRejectsPresentButUnusableTools(t *testing.T) {
	t.Parallel()

	detected := Toolchain{
		Tools: map[string]Tool{
			"node": {
				Name:    "node",
				Present: true,
				Error:   "node version probe failed",
			},
			"npm": {
				Name:    "npm",
				Present: true,
				Error:   "npm shim could not be invoked",
			},
			"pnpm": {
				Name:    "pnpm",
				Present: true,
			},
			"git": {
				Name:    "git",
				Present: true,
				Error:   "git version probe failed",
			},
			"cargo": {
				Name:    "cargo",
				Present: true,
			},
		},
	}
	result := EvaluateRequirements(detected, Requirements{
		Node:            ">=20.0.0",
		PackageManagers: []string{"npm", "pnpm"},
		Tools:           []string{"git", "cargo"},
	})
	want := []string{
		"node was found but could not be used: node version probe failed",
		"required tool git could not be used: git version probe failed",
		"required tool cargo has no usable version",
		"no supported package manager is usable: npm (npm shim could not be invoked), pnpm (version unavailable)",
	}
	if result.Available {
		t.Fatal("EvaluateRequirements() unexpectedly available")
	}
	if !reflect.DeepEqual(result.Reasons, want) {
		t.Fatalf("Reasons = %#v, want %#v", result.Reasons, want)
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
