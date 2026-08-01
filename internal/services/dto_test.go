package services

import (
	"fmt"
	"reflect"
	"testing"

	"nodesmith/internal/planner"
	"nodesmith/internal/recipe"
	"nodesmith/internal/toolchain"
)

// The frontend bridge parser rejects null where it expects an array, so every
// slice a service returns must encode as [] even when it is empty.
func TestDTOsEncodeEmptySlicesAsArrays(t *testing.T) {
	manifest := recipe.Manifest{ID: "bare", Name: "Bare"}
	detected := toolchain.Toolchain{Tools: map[string]toolchain.Tool{}}
	available, reasons, manager := recipeAvailability(manifest, detected, nil)
	if !available {
		t.Fatalf("recipeAvailability() available = false, reasons = %v", reasons)
	}

	summary := RecipeSummary{
		Tags:                  cloneSlice(manifest.Tags),
		Available:             available,
		UnavailableReasons:    reasons,
		DefaultPackageManager: manager,
	}
	assertNoNullArray(t, "RecipeSummary", summary)

	full, err := recipeDTO(manifest)
	if err != nil {
		t.Fatalf("recipeDTO() error = %v", err)
	}
	full.UnavailableReasons = reasons
	assertNoNullArray(t, "Recipe", full)

	assertNoNullArray(t, "Plan", planDTO(planner.Plan{Steps: []planner.PlanStep{{ID: "one"}}}))
	assertNoNullArray(t, "Toolchain", toolchainDTO(toolchain.Toolchain{}, ""))
	assertNoNullArray(t, "ReloadResult", ReloadResult{
		Warnings:  cloneSlice[string](nil),
		Overrides: cloneSlice[string](nil),
	})
}

func assertNoNullArray(t *testing.T, name string, value any) {
	t.Helper()
	assertNoNilSlice(t, name, reflect.ValueOf(value))
}

// assertNoNilSlice walks value and fails on any nil slice, which json.Marshal
// would encode as null. Nil maps are allowed: the bridge parser reads a null
// object as {}.
func assertNoNilSlice(t *testing.T, path string, value reflect.Value) {
	t.Helper()
	switch value.Kind() {
	case reflect.Slice:
		if value.IsNil() {
			t.Fatalf("%s is a nil slice, want an empty slice so it encodes as []", path)
		}
		for index := range value.Len() {
			assertNoNilSlice(t, fmt.Sprintf("%s[%d]", path, index), value.Index(index))
		}
	case reflect.Struct:
		for index := range value.NumField() {
			field := value.Type().Field(index)
			assertNoNilSlice(t, path+"."+field.Name, value.Field(index))
		}
	case reflect.Interface, reflect.Pointer:
		if !value.IsNil() {
			assertNoNilSlice(t, path, value.Elem())
		}
	}
}
