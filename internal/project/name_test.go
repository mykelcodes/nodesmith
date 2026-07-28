package project

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateProjectNameHostileAndPortableNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		allowLeadingDot bool
		valid           bool
	}{
		{name: "app", valid: true},
		{name: "a b", valid: true},
		{name: "café-项目", valid: true},
		{name: "package.name", valid: true},
		{name: ".hidden", valid: false},
		{name: ".hidden", allowLeadingDot: true, valid: true},
		{name: "..", valid: false},
		{name: ".", allowLeadingDot: true, valid: false},
		{name: "con", valid: false},
		{name: "CON.txt", valid: false},
		{name: "CONOUT$", valid: false},
		{name: "lpt9.log", valid: false},
		{name: "COM¹", valid: false},
		{name: "com0", valid: true},
		{name: "a/b", valid: false},
		{name: `a\b`, valid: false},
		{name: `a:b`, valid: false},
		{name: `a"b`, valid: false},
		{name: " leading", valid: false},
		{name: "trailing ", valid: false},
		{name: "trailing.", valid: false},
		{name: "line\nbreak", valid: false},
		{name: strings.Repeat("a", 300), valid: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateProjectName(test.name, test.allowLeadingDot)
			if test.valid && err != nil {
				t.Fatalf("ValidateProjectName(%q) error = %v", test.name, err)
			}
			if !test.valid && !errors.Is(err, ErrInvalidName) {
				t.Fatalf(
					"ValidateProjectName(%q) error = %v, want ErrInvalidName",
					test.name,
					err,
				)
			}
		})
	}
}

func TestValidateNameCustomLimitCannotExceedPortableMaximum(t *testing.T) {
	t.Parallel()

	if err := ValidateName("123456", NameOptions{MaxBytes: 5}); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("ValidateName() error = %v, want custom length failure", err)
	}
	if err := ValidateName(
		strings.Repeat("a", 256),
		NameOptions{MaxBytes: 1_000},
	); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("ValidateName() error = %v, want portable length failure", err)
	}
}

func TestValidateNameRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()

	if err := ValidateName(string([]byte{0xff}), NameOptions{}); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("ValidateName() error = %v, want ErrInvalidName", err)
	}
}
