package planner

import "strings"

const (
	pnpmNonStrictBuildsArgument = "--config.strict-dep-builds=false"
	pnpmBlockedBuildsWarning    = "pnpm will leave unapproved dependency build scripts blocked instead of failing project creation. Review them and run pnpm approve-builds in the generated project if those scripts are required."
)

// configurePnpmInstall keeps pnpm's build-script approval boundary while
// preventing a completed dependency download from failing the entire scaffold.
// The CLI argument applies only to this reviewed install; it does not weaken
// strictDepBuilds for later installs in the generated project.
func configurePnpmInstall(binary string, args []string) ([]string, bool) {
	if binary != "pnpm" || len(args) == 0 || (args[0] != "install" && args[0] != "i") {
		return args, false
	}
	for _, arg := range args[1:] {
		if isPnpmBuildPolicyArgument(arg) {
			return args, false
		}
	}
	return append(args, pnpmNonStrictBuildsArgument), true
}

func isPnpmBuildPolicyArgument(argument string) bool {
	name, _, _ := strings.Cut(strings.ToLower(argument), "=")
	switch name {
	case "--strict-dep-builds",
		"--config.strict-dep-builds",
		"--ignore-scripts",
		"--config.ignore-scripts",
		"--dangerously-allow-all-builds",
		"--config.dangerously-allow-all-builds":
		return true
	default:
		return false
	}
}
