package recipe

import (
	"encoding/json"
	"strings"
	"testing"
)

func intPtr(value int) *int           { return &value }
func floatPtr(value float64) *float64 { return &value }

func TestValidateFieldConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		field   Field
		wantErr string
	}{
		{
			name:  "no constraints is valid",
			field: Field{ID: "name", Label: "Name", Type: FieldText, Default: "value"},
		},
		{
			name: "text constraints satisfied by the default",
			field: Field{
				ID: "name", Label: "Name", Type: FieldText, Default: "abc",
				Pattern: "^[a-z]+$", MinLength: intPtr(1), MaxLength: intPtr(8),
			},
		},
		{
			name: "pattern on a number field is rejected",
			field: Field{
				ID: "port", Label: "Port", Type: FieldNumber,
				Default: json.Number("3000"), Pattern: "^[0-9]+$",
			},
			wantErr: "apply only to text fields",
		},
		{
			name: "min on a text field is rejected",
			field: Field{
				ID: "name", Label: "Name", Type: FieldText, Default: "a", Min: floatPtr(1),
			},
			wantErr: "apply only to number fields",
		},
		{
			name: "an uncompilable pattern is rejected at load time",
			field: Field{
				ID: "name", Label: "Name", Type: FieldText, Default: "a", Pattern: "([a-z",
			},
			wantErr: "compile pattern",
		},
		{
			name: "minLength above maxLength is rejected",
			field: Field{
				ID: "name", Label: "Name", Type: FieldText, Default: "abc",
				MinLength: intPtr(5), MaxLength: intPtr(2),
			},
			wantErr: "exceeds maxLength",
		},
		{
			name: "min above max is rejected",
			field: Field{
				ID: "port", Label: "Port", Type: FieldNumber,
				Default: json.Number("1"), Min: floatPtr(10), Max: floatPtr(1),
			},
			wantErr: "exceeds max",
		},
		{
			name: "negative minLength is rejected",
			field: Field{
				ID: "name", Label: "Name", Type: FieldText, Default: "a", MinLength: intPtr(-1),
			},
			wantErr: "must not be negative",
		},
		// A default that violates its own constraint would be substituted into
		// argv whenever the field is left unanswered.
		{
			name: "a default violating its own pattern is rejected",
			field: Field{
				ID: "name", Label: "Name", Type: FieldText, Default: "NOPE", Pattern: "^[a-z]+$",
			},
			wantErr: "does not match the required pattern",
		},
		{
			name: "a default below min is rejected",
			field: Field{
				ID: "port", Label: "Port", Type: FieldNumber,
				Default: json.Number("80"), Min: floatPtr(1024),
			},
			wantErr: "must be at least 1024",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateFieldConstraints("fields[0]", test.field)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validateFieldConstraints() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validateFieldConstraints() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestCheckFieldConstraints(t *testing.T) {
	t.Parallel()

	text := Field{
		ID: "name", Label: "Name", Type: FieldText,
		Pattern: "^[a-z-]+$", MinLength: intPtr(2), MaxLength: intPtr(6),
	}
	number := Field{
		ID: "port", Label: "Port", Type: FieldNumber, Min: floatPtr(1024), Max: floatPtr(65535),
	}

	tests := []struct {
		name    string
		field   Field
		value   any
		wantErr string
	}{
		{name: "text within bounds", field: text, value: "my-app"},
		{name: "text too short", field: text, value: "a", wantErr: "at least 2 characters"},
		{name: "text too long", field: text, value: "way-too-long", wantErr: "at most 6 characters"},
		{name: "text failing the pattern", field: text, value: "AB", wantErr: "does not match"},
		{name: "number within bounds", field: number, value: json.Number("3000")},
		{name: "number below min", field: number, value: json.Number("80"), wantErr: "at least 1024"},
		{
			name: "number above max", field: number,
			value: json.Number("70000"), wantErr: "at most 65535",
		},
		// Type mismatches are the caller's job to report, so they are not
		// double-reported here.
		{name: "wrong type is ignored", field: text, value: 42},
		{name: "unconstrained field accepts anything", field: Field{Type: FieldText}, value: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := CheckFieldConstraints(test.field, test.value)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("CheckFieldConstraints() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("CheckFieldConstraints() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

// Length limits are expressed in characters, so a limit of 3 must accept three
// multi-byte characters rather than counting their bytes.
func TestCheckFieldConstraintsCountsRunesNotBytes(t *testing.T) {
	t.Parallel()

	field := Field{ID: "name", Label: "Name", Type: FieldText, MaxLength: intPtr(3)}
	if err := CheckFieldConstraints(field, "日本語"); err != nil {
		t.Fatalf("CheckFieldConstraints() error = %v, want three runes accepted", err)
	}
	if err := CheckFieldConstraints(field, "日本語だ"); err == nil {
		t.Fatal("CheckFieldConstraints() error = nil, want four runes rejected")
	}
}

func TestCompilePatternCachesAndRejectsBadExpressions(t *testing.T) {
	t.Parallel()

	first, err := CompilePattern(`^\d{2,4}$`)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CompilePattern(`^\d{2,4}$`)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("CompilePattern returned a fresh regexp for a cached pattern")
	}
	if _, err := CompilePattern("(unclosed"); err == nil {
		t.Fatal("CompilePattern() error = nil, want a compile failure")
	}
}
