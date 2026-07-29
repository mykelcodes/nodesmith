package planner

import (
	"reflect"
	"testing"
)

func TestConfigurePnpmInstallUsesNonStrictBuildApproval(t *testing.T) {
	t.Parallel()

	args := []string{"install", "--frozen-lockfile"}
	got, changed := configurePnpmInstall("pnpm", args)
	want := []string{"install", "--frozen-lockfile", pnpmNonStrictBuildsArgument}
	if !changed || !reflect.DeepEqual(got, want) {
		t.Fatalf("configurePnpmInstall() = %#v, %v, want %#v, true", got, changed, want)
	}
	if !reflect.DeepEqual(args, []string{"install", "--frozen-lockfile"}) {
		t.Fatalf("configurePnpmInstall() mutated source args: %#v", args)
	}
}

func TestConfigurePnpmInstallLeavesOtherCommandsAndManagersAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		binary string
		args   []string
	}{
		{name: "npm install", binary: "npm", args: []string{"install"}},
		{name: "pnpm exec", binary: "pnpm", args: []string{"exec", "vite"}},
		{name: "pnpx", binary: "pnpx", args: []string{"create-vite"}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, changed := configurePnpmInstall(test.binary, test.args)
			if changed || !reflect.DeepEqual(got, test.args) {
				t.Fatalf("configurePnpmInstall() = %#v, %v, want unchanged", got, changed)
			}
		})
	}
}

func TestConfigurePnpmInstallRespectsExplicitBuildPolicy(t *testing.T) {
	t.Parallel()

	policies := []string{
		"--config.strict-dep-builds=true",
		"--ignore-scripts",
		"--config.ignore-scripts=false",
		"--dangerously-allow-all-builds",
		"--config.dangerously-allow-all-builds=true",
	}
	for _, policy := range policies {
		policy := policy
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			args := []string{"install", policy}
			got, changed := configurePnpmInstall("pnpm", args)
			if changed || !reflect.DeepEqual(got, args) {
				t.Fatalf("configurePnpmInstall() = %#v, %v, want explicit policy unchanged", got, changed)
			}
		})
	}
}

func TestResolveMakesPnpmInstallNonFatalAndReviewable(t *testing.T) {
	t.Parallel()

	manifest := releaseAgeManifest("pnpm", "npm")
	resolver := &fakeCommandResolver{prefix: []string{"/tools/pnpm.cjs"}}
	plan, err := Resolve(manifest, ScaffoldRequest{
		RecipeID:       manifest.ID,
		ProjectName:    "app",
		ParentDir:      ".",
		PackageManager: "pnpm",
		InstallDeps:    true,
		GitInit:        true,
		Answers:        map[string]any{},
	}, resolver)
	if err != nil {
		t.Fatal(err)
	}

	install := stepByID(t, plan, "install")
	wantArgs := []string{"/tools/pnpm.cjs", "install", pnpmNonStrictBuildsArgument}
	if !reflect.DeepEqual(install.Args, wantArgs) {
		t.Fatalf("install args = %#v, want %#v", install.Args, wantArgs)
	}
	if len(plan.Warnings) != 1 || plan.Warnings[0] != pnpmBlockedBuildsWarning {
		t.Fatalf("warnings = %#v, want pnpm build-approval warning", plan.Warnings)
	}
}
