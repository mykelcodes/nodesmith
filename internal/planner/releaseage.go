package planner

import (
	"fmt"
	"strconv"
)

// Package-manager families that read a minimum release age. Nodesmith stores
// the cooldown in minutes because that is what pnpm and Yarn use natively; npm
// counts whole days and bun counts seconds.
const (
	familyNPM  = "npm"
	familyPNPM = "pnpm"
	familyYarn = "yarn"
	familyBun  = "bun"
)

const minutesPerDay = 1440

// binaryFamily maps a manifest binary name onto the package-manager family that
// would read a cooldown from the environment. Binaries outside the JavaScript
// package managers return an empty family.
func binaryFamily(binary string) string {
	switch binary {
	case "npm", "npx":
		return familyNPM
	case "pnpm", "pnpx":
		return familyPNPM
	case "yarn":
		return familyYarn
	case "bun", "bunx":
		return familyBun
	default:
		return ""
	}
}

// releaseAgeEnvironment renders the cooldown for one step as environment
// variables. families is the set of package managers that could reach the
// registry during the step: its own binary plus the package manager the user
// selected, because generators routinely delegate installation.
func releaseAgeEnvironment(minutes int, families map[string]struct{}) map[string]string {
	environment := make(map[string]string, 3)
	value := strconv.Itoa(minutes)

	if _, ok := families[familyNPM]; ok {
		// npm's min-release-age counts whole days, so round up rather than
		// silently weakening the cooldown to a shorter one.
		environment["npm_config_min_release_age"] = strconv.Itoa(ceilDays(minutes))
	}
	if _, ok := families[familyPNPM]; ok {
		environment["pnpm_config_minimum_release_age"] = value
		// pnpm 10 reads npm-style environment config; pnpm 11 dropped it. The
		// fallback is skipped when npm also runs in this step, because npm
		// warns loudly about config keys it does not recognise.
		if _, npmRuns := families[familyNPM]; !npmRuns {
			environment["npm_config_minimum_release_age"] = value
		}
	}
	if _, ok := families[familyYarn]; ok {
		environment["YARN_NPM_MINIMAL_AGE_GATE"] = value
	}
	// bun has no environment equivalent. The planner adds a bunfig.toml project
	// configuration step before dependency installation instead.
	return environment
}

// releaseAgeWarnings explains, once per plan, where the requested cooldown
// cannot be applied exactly.
func releaseAgeWarnings(minutes int, families map[string]struct{}) []string {
	warnings := make([]string, 0, 1)
	if _, ok := families[familyNPM]; ok && minutes%minutesPerDay != 0 {
		warnings = append(warnings, fmt.Sprintf(
			"npm measures its package cooldown in whole days, so %d minutes was rounded up to %d for npm and npx steps.",
			minutes,
			ceilDays(minutes),
		))
	}
	return warnings
}

func releaseAgeProjectConfig(packageManager string, minutes int) *ProjectConfig {
	switch binaryFamily(packageManager) {
	case familyNPM:
		return &ProjectConfig{
			Path:   ".npmrc",
			Format: ConfigFormatProperties,
			Key:    "min-release-age",
			Value:  strconv.Itoa(ceilDays(minutes)),
		}
	case familyPNPM:
		return &ProjectConfig{
			Path:   "pnpm-workspace.yaml",
			Format: ConfigFormatYAML,
			Key:    "minimumReleaseAge",
			Value:  strconv.Itoa(minutes),
		}
	case familyYarn:
		return &ProjectConfig{
			Path:   ".yarnrc.yml",
			Format: ConfigFormatYAML,
			Key:    "npmMinimalAgeGate",
			Value:  strconv.Quote(yarnReleaseAge(minutes)),
		}
	case familyBun:
		return &ProjectConfig{
			Path:    "bunfig.toml",
			Format:  ConfigFormatTOML,
			Section: "install",
			Key:     "minimumReleaseAge",
			Value:   strconv.Itoa(minutes * 60),
		}
	default:
		return nil
	}
}

func yarnReleaseAge(minutes int) string {
	switch {
	case minutes == 0:
		return "0m"
	case minutes%10080 == 0:
		return strconv.Itoa(minutes/10080) + "w"
	case minutes%minutesPerDay == 0:
		return strconv.Itoa(minutes/minutesPerDay) + "d"
	case minutes%60 == 0:
		return strconv.Itoa(minutes/60) + "h"
	default:
		return strconv.Itoa(minutes) + "m"
	}
}

func releaseAgeConfigDisplay(config ProjectConfig) string {
	key := config.Key
	if config.Section != "" {
		key = config.Section + "." + key
	}
	return fmt.Sprintf("write %s: %s = %s", config.Path, key, config.Value)
}

func ceilDays(minutes int) int {
	if minutes <= 0 {
		return 0
	}
	return (minutes + minutesPerDay - 1) / minutesPerDay
}
