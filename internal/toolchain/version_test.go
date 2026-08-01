package toolchain

import "testing"

func TestParseToolVersionCapturedOutputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tool   string
		output string
		want   string
	}{
		{tool: "node", output: "v22.5.1\n", want: "22.5.1"},
		{tool: "npm", output: "10.8.2\n", want: "10.8.2"},
		{tool: "npx", output: "10.8.2\n", want: "10.8.2"},
		{tool: "pnpm", output: "9.6.0\n", want: "9.6.0"},
		{tool: "yarn", output: "1.22.22\n", want: "1.22.22"},
		{tool: "bun", output: "1.1.20\n", want: "1.1.20"},
		{
			tool:   "git",
			output: "git version 2.45.2 (Apple Git-156)\n",
			want:   "2.45.2",
		},
		{
			tool:   "go",
			output: "go version go1.25.0 darwin/arm64\n",
			want:   "1.25.0",
		},
		{
			tool:   "cargo",
			output: "cargo 1.80.1 (376290515 2024-07-16)\n",
			want:   "1.80.1",
		},
		{
			tool:   "gh",
			output: "gh version 2.54.0 (2024-08-01)\nhttps://github.com/cli/cli/releases/tag/v2.54.0\n",
			want:   "2.54.0",
		},
		{
			tool:   "code",
			output: "1.92.1\n123456789abcdef\narm64\n",
			want:   "1.92.1",
		},
		{
			tool:   "wails",
			output: "\x1b[0;92mWails CLI\x1b[0m \x1b[0;31mv2.13.0\x1b[0m\n",
			want:   "2.13.0",
		},
		{
			tool:   "npm",
			output: "npm warn config legacy-peer-deps\n10.8\n",
			want:   "10.8.0",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.tool+"_"+test.want, func(t *testing.T) {
			t.Parallel()
			got, err := ParseToolVersion(test.tool, test.output)
			if err != nil {
				t.Fatalf("ParseToolVersion() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ParseToolVersion() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseToolVersionRejectsUnparseableOutput(t *testing.T) {
	t.Parallel()

	if _, err := ParseToolVersion("node", "development build"); err == nil {
		t.Fatal("ParseToolVersion() accepted output with no version")
	}
}

func TestSemanticVersionCompare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		left  string
		right string
		want  int
	}{
		{left: "20.0.0", right: "20.0.0", want: 0},
		{left: "20.1.0", right: "20.0.9", want: 1},
		{left: "19.9.9", right: "20.0.0", want: -1},
		{left: "20.0.0-rc.1", right: "20.0.0", want: -1},
		{left: "20.0.0-rc.2", right: "20.0.0-rc.10", want: -1},
		{left: "20.0.0-alpha", right: "20.0.0-1", want: 1},
		{left: "20.0.0+one", right: "20.0.0+two", want: 0},
	}
	for _, test := range tests {
		test := test
		t.Run(test.left+"_"+test.right, func(t *testing.T) {
			t.Parallel()
			left, err := ParseSemanticVersion(test.left)
			if err != nil {
				t.Fatal(err)
			}
			right, err := ParseSemanticVersion(test.right)
			if err != nil {
				t.Fatal(err)
			}
			if got := left.Compare(right); got != test.want {
				t.Fatalf("Compare() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestSemanticVersionCompareIsATotalOrder(t *testing.T) {
	t.Parallel()

	// Semantic-version precedence, ascending.
	ascending := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
		"1.0.1",
		"1.1.0",
		"2.0.0",
	}
	parsed := make([]SemanticVersion, len(ascending))
	for index, value := range ascending {
		version, err := ParseSemanticVersion(value)
		if err != nil {
			t.Fatalf("ParseSemanticVersion(%q) error = %v", value, err)
		}
		parsed[index] = version
	}

	for leftIndex, left := range parsed {
		for rightIndex, right := range parsed {
			forward := left.Compare(right)
			if forward != -right.Compare(left) {
				t.Fatalf(
					"Compare(%q, %q) = %d and Compare(%q, %q) = %d are not antisymmetric",
					ascending[leftIndex], ascending[rightIndex], forward,
					ascending[rightIndex], ascending[leftIndex], right.Compare(left),
				)
			}
			want := 0
			switch {
			case leftIndex < rightIndex:
				want = -1
			case leftIndex > rightIndex:
				want = 1
			}
			if forward != want {
				t.Fatalf(
					"Compare(%q, %q) = %d, want %d",
					ascending[leftIndex], ascending[rightIndex], forward, want,
				)
			}
		}
	}
}

func TestParseSemanticVersionRejectsLeadingZeroPrerelease(t *testing.T) {
	t.Parallel()

	// "01" and "1" compare equal numerically but differ as strings, which is
	// why semantic versioning forbids the form outright.
	for _, value := range []string{"1.0.0-01", "1.0.0-beta.01", "1.0.0-00"} {
		if _, err := ParseSemanticVersion(value); err == nil {
			t.Errorf("ParseSemanticVersion(%q) error = nil, want a rejection", value)
		}
	}
	for _, value := range []string{"1.0.0-0", "1.0.0-0a", "1.0.0-alpha.0", "1.0.0+00.1"} {
		if _, err := ParseSemanticVersion(value); err != nil {
			t.Errorf("ParseSemanticVersion(%q) error = %v, want acceptance", value, err)
		}
	}
}

func TestSatisfiesRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version    string
		constraint string
		want       bool
		wantError  bool
	}{
		{version: "20.0.0", constraint: ">=20.0.0", want: true},
		{version: "18.20.4", constraint: ">=20.0.0", want: false},
		{version: "20.11.1", constraint: ">= 20.0.0 <21.0.0", want: true},
		{version: "21.0.0", constraint: ">=20.0.0, <21.0.0", want: false},
		{version: "20.0.0-rc.1", constraint: ">=20.0.0", want: false},
		{version: "20.0.0", constraint: "20.0.0", want: true},
		{version: "20.0.0", constraint: "", want: true},
		{version: "20.0.0", constraint: "^20.0.0", wantError: true},
		{version: "20.0.0", constraint: ">=20 || <21", wantError: true},
		{version: "invalid", constraint: ">=20.0.0", wantError: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.version+"_"+test.constraint, func(t *testing.T) {
			t.Parallel()
			got, err := SatisfiesRange(test.version, test.constraint)
			if test.wantError {
				if err == nil {
					t.Fatal("SatisfiesRange() returned no error")
				}
				return
			}
			if err != nil {
				t.Fatalf("SatisfiesRange() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("SatisfiesRange() = %t, want %t", got, test.want)
			}
		})
	}
}
