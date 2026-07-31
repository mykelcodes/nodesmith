package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeCommandResolver struct {
	prefix map[string][]string
	err    error
}

func (resolver fakeCommandResolver) ResolveCommand(
	name string,
) (string, []string, error) {
	if resolver.err != nil {
		return "", nil, resolver.err
	}
	return "/tools/" + name, append([]string(nil), resolver.prefix[name]...), nil
}

func TestParseSmokeSpecsStrictAndNormalised(t *testing.T) {
	t.Parallel()

	specs, err := parseSmokeSpecs(`[
		{
			"label":"Build",
			"bin":"npm",
			"args":["run","build"],
			"env":{"CI":"0","EXTRA":"yes"}
		}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 {
		t.Fatalf("spec count = %d, want 1", len(specs))
	}
	if specs[0].ID != "smoke-1" || specs[0].CWD != "projectDir" {
		t.Fatalf("normalised spec = %#v", specs[0])
	}

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "unknown field", raw: `[{"label":"x","bin":"npm","wat":true}]`, want: "unknown field"},
		{name: "trailing value", raw: `[] {}`, want: "unexpected data"},
		{name: "missing label", raw: `[{"bin":"npm"}]`, want: "label is required"},
		{name: "missing binary", raw: `[{"label":"x","bin":""}]`, want: "bin is required"},
		{
			name: "invalid cwd",
			raw:  `[{"label":"x","bin":"npm","cwd":"somewhere"}]`,
			want: "cwd must be",
		},
		{
			name: "duplicate id",
			raw:  `[{"id":"same","label":"x","bin":"npm"},{"id":"same","label":"y","bin":"npm"}]`,
			want: "duplicate",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSmokeSpecs(test.raw)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parseSmokeSpecs() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestBuildSmokePlanUsesNativeResolutionAndExactArgv(t *testing.T) {
	t.Parallel()

	specs, err := parseSmokeSpecs(`[
		{
			"id":"build",
			"label":"Build project",
			"bin":"npm",
			"args":["run","build","value with spaces"],
			"env":{"CI":"0","CUSTOM":"value"}
		}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := buildSmokePlan(
		"sample",
		"/tmp/project",
		specs,
		fakeCommandResolver{prefix: map[string][]string{"npm": {"/tools/npm-cli.js"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(plan.Steps))
	}
	step := plan.Steps[0]
	wantArgs := []string{"/tools/npm-cli.js", "run", "build", "value with spaces"}
	if step.Bin != "/tools/npm" || !reflect.DeepEqual(step.Args, wantArgs) {
		t.Fatalf("resolved command = %q %#v, want /tools/npm %#v", step.Bin, step.Args, wantArgs)
	}
	if step.Env["CI"] != "true" || step.Env["CUSTOM"] != "value" {
		t.Fatalf("environment = %#v, want enforced CI=true and CUSTOM=value", step.Env)
	}
	if !strings.Contains(step.Display, `"value with spaces"`) {
		t.Fatalf("display = %q, want unsplit spaced argument", step.Display)
	}
}

func TestBuildSmokePlanPropagatesResolutionError(t *testing.T) {
	t.Parallel()

	specs, err := parseSmokeSpecs(`[{"label":"Build","bin":"npm"}]`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = buildSmokePlan(
		"sample",
		"/tmp/project",
		specs,
		fakeCommandResolver{err: errors.New("not found")},
	)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("buildSmokePlan() error = %v, want resolution failure", err)
	}
}
