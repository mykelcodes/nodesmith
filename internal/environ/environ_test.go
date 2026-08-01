package environ

import (
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		base      []string
		overrides map[string]string
		goos      string
		want      []string
	}{
		{
			name:      "no overrides returns base untouched",
			base:      []string{"A=1", "PATH=/old"},
			overrides: nil,
			goos:      "linux",
			want:      []string{"A=1", "PATH=/old"},
		},
		{
			name:      "replaces matching key and appends overrides sorted",
			base:      []string{"PATH=/old", "UNCHANGED=yes"},
			overrides: map[string]string{"PATH": "/new", "CI": "1"},
			goos:      "linux",
			want:      []string{"UNCHANGED=yes", "CI=1", "PATH=/new"},
		},
		{
			name:      "entries without a separator have no key and survive",
			base:      []string{"PATH=/old", "MALFORMED"},
			overrides: map[string]string{"PATH": "/new"},
			goos:      "linux",
			want:      []string{"MALFORMED", "PATH=/new"},
		},
		{
			name:      "unix matching is case sensitive",
			base:      []string{"Path=/keep", "A=1"},
			overrides: map[string]string{"PATH": "/new"},
			goos:      "linux",
			want:      []string{"Path=/keep", "A=1", "PATH=/new"},
		},
		{
			name:      "windows matching is case insensitive and deduplicates",
			base:      []string{"Path=old", "A=1", "PATH=duplicate"},
			overrides: map[string]string{"PATH": `C:\new`},
			goos:      "windows",
			want:      []string{"A=1", `PATH=C:\new`},
		},
		{
			name:      "windows folds the override key too",
			base:      []string{"PATH=old", "A=1"},
			overrides: map[string]string{"Path": `C:\new`},
			goos:      "windows",
			want:      []string{"A=1", `Path=C:\new`},
		},
		{
			name:      "missing key is appended",
			base:      []string{"A=1"},
			overrides: map[string]string{"PATH": "/new"},
			goos:      "darwin",
			want:      []string{"A=1", "PATH=/new"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := Merge(test.base, test.overrides, test.goos)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Merge() = %#v, want %#v", got, test.want)
			}
		})
	}
}

// Overrides are appended in sorted key order so that two runs of the same plan
// build byte-identical environments.
func TestMergeOrdersOverridesDeterministically(t *testing.T) {
	t.Parallel()

	overrides := map[string]string{"Z": "26", "A": "1", "M": "13", "B": "2"}
	want := []string{"KEEP=yes", "A=1", "B=2", "M=13", "Z=26"}
	for range 32 {
		got := Merge([]string{"KEEP=yes"}, overrides, "linux")
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Merge() = %#v, want %#v", got, want)
		}
	}
}

func TestMergeDoesNotAliasBase(t *testing.T) {
	t.Parallel()

	base := []string{"A=1", "PATH=/old"}
	got := Merge(base, map[string]string{"PATH": "/new"}, "linux")
	got[0] = "MUTATED=1"
	if base[0] != "A=1" {
		t.Fatalf("Merge aliased its base slice: base[0] = %q", base[0])
	}
}

func TestWithPATH(t *testing.T) {
	t.Parallel()

	got := WithPATH([]string{"A=1", "PATH=/old", "B=2"}, "/new", "linux")
	want := []string{"A=1", "B=2", "PATH=/new"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WithPATH() = %#v, want %#v", got, want)
	}
}
