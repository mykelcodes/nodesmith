package recipe

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestDecodeAndValidateValidFixture(t *testing.T) {
	t.Parallel()

	file, err := os.Open("testdata/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close fixture: %v", err)
		}
	})

	manifest, err := DecodeAndValidate(file)
	if err != nil {
		t.Fatalf("DecodeAndValidate() error = %v", err)
	}
	if manifest.ID != "basic" {
		t.Fatalf("manifest.ID = %q, want basic", manifest.ID)
	}
	if got := manifest.Fields[0].Default; got != "a" {
		t.Fatalf("select default = %#v, want a", got)
	}
}

func TestDecodeRejectsStrictJSONViolations(t *testing.T) {
	t.Parallel()

	valid := fixtureText(t, "testdata/valid/basic.json")
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "unknown root field",
			raw:  strings.Replace(valid, `"schemaVersion": 1,`, `"schemaVersion": 1, "surprise": true,`, 1),
			want: `unknown field "surprise"`,
		},
		{
			name: "duplicate object key",
			raw:  strings.Replace(valid, `"id": "basic",`, `"id": "first", "id": "basic",`, 1),
			want: `duplicate JSON object key "id"`,
		},
		{
			name: "unknown nested field",
			raw:  strings.Replace(valid, `"node": ">=20.0.0",`, `"node": ">=20.0.0", "surprise": true,`, 1),
			want: `unknown field "surprise"`,
		},
		{
			name: "trailing json",
			raw:  valid + `{}`,
			want: "unexpected data after JSON value",
		},
		{
			name: "non object manifest",
			raw:  `[]`,
			want: "cannot unmarshal array",
		},
		{
			name: "unknown conditional property",
			raw: strings.Replace(
				valid,
				`{"if": "flag", "then": ["--flag"], "else": []}`,
				`{"if": "flag", "then": ["--flag"], "else": [], "surprise": true}`,
				1,
			),
			want: `unknown field "surprise"`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(strings.NewReader(test.raw))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Decode() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestDecodeRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()

	_, err := DecodeBytes([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'})
	if err == nil || !strings.Contains(err.Error(), "not valid UTF-8") {
		t.Fatalf("DecodeBytes(invalid UTF-8) error = %v", err)
	}
}

func TestArgNodeRejectsInvalidUnionShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want string
	}{
		{raw: ``, want: "unexpected end"},
		{raw: `3`, want: "must be a string"},
		{raw: `{}`, want: "exactly one"},
		{raw: `{"if":"flag","forEach":"items","then":[],"args":[]}`, want: "exactly one"},
		{raw: `{"if":"flag"}`, want: `requires "then"`},
		{raw: `{"forEach":"items"}`, want: `requires "args"`},
	}
	for _, test := range tests {
		var node ArgNode
		err := json.Unmarshal([]byte(test.raw), &node)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("Unmarshal(%q) error = %v, want substring %q", test.raw, err, test.want)
		}
	}

	if _, err := json.Marshal(ArgNode{}); err == nil {
		t.Fatal("Marshal(ArgNode{}) unexpectedly succeeded")
	}
	if _, err := json.Marshal(ArgNode{Literal: ptr("x"), Conditional: &ConditionalArg{}}); err == nil {
		t.Fatal("Marshal(arg with two variants) unexpectedly succeeded")
	}
}

func fixtureText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func ptr(value string) *string {
	return &value
}
