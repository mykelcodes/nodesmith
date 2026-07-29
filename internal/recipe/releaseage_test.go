package recipe

import (
	"strings"
	"testing"
)

func testMinutes(value int) *int {
	return &value
}

func TestMinimumReleaseAgeIsOptionalAndAcceptsZero(t *testing.T) {
	t.Parallel()

	manifest := validManifest(t)
	if manifest.MinimumReleaseAge != nil {
		t.Fatalf("fixture minimumReleaseAge = %d, want unset", *manifest.MinimumReleaseAge)
	}
	if err := Validate(manifest); err != nil {
		t.Fatalf("Validate() with no cooldown error = %v", err)
	}

	// Zero is a deliberate "no cooldown for this recipe", not a missing value.
	manifest.MinimumReleaseAge = testMinutes(0)
	if err := Validate(manifest); err != nil {
		t.Fatalf("Validate() with a zero cooldown error = %v", err)
	}

	manifest.MinimumReleaseAge = testMinutes(MaxMinimumReleaseAge)
	if err := Validate(manifest); err != nil {
		t.Fatalf("Validate() at the cooldown ceiling error = %v", err)
	}
}

func TestDecodeReadsMinimumReleaseAge(t *testing.T) {
	t.Parallel()

	source := strings.Replace(
		fixtureText(t, "testdata/valid/basic.json"),
		`"schemaVersion": 1,`,
		`"schemaVersion": 1,
  "minimumReleaseAge": 4320,`,
		1,
	)
	manifest, err := DecodeAndValidate(strings.NewReader(source))
	if err != nil {
		t.Fatalf("DecodeAndValidate() error = %v", err)
	}
	if manifest.MinimumReleaseAge == nil || *manifest.MinimumReleaseAge != 4320 {
		t.Fatalf("minimumReleaseAge = %v, want 4320", manifest.MinimumReleaseAge)
	}
}

func TestValidateMinimumReleaseAgeRange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		minutes int
		want    string
	}{
		{name: "zero", minutes: 0},
		{name: "one day", minutes: 1440},
		{name: "ceiling", minutes: MaxMinimumReleaseAge},
		{name: "negative", minutes: -1, want: "must not be negative"},
		{name: "over the ceiling", minutes: MaxMinimumReleaseAge + 1, want: "must be at most"},
	}
	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateMinimumReleaseAge(test.minutes)
			if test.want == "" {
				if err != nil {
					t.Fatalf("ValidateMinimumReleaseAge(%d) error = %v, want nil", test.minutes, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateMinimumReleaseAge(%d) error = %v, want substring %q", test.minutes, err, test.want)
			}
		})
	}
}
