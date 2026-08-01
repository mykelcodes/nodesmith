// Package environ builds child-process environments.
//
// Three near-identical helpers used to do this — one each in internal/runner,
// internal/toolchain, and internal/project — with three sets of edge cases and
// uneven test coverage. Matching environment variable names correctly (case
// sensitively everywhere except Windows) and deduplicating existing entries is
// exactly the kind of detail that should have one implementation.
package environ

import (
	"slices"
	"strings"
)

// Merge returns base with every entry whose key matches an override removed,
// followed by the overrides appended in sorted key order.
//
// Ordering is deterministic by construction: environment variables have no
// meaningful order, but a stable one keeps process environments reproducible
// and keeps golden plan tests a usable oracle.
//
// Matching is case-insensitive when goos is "windows", where PATH and Path name
// the same variable, and case-sensitive everywhere else. An entry with no "="
// has no key to match and is preserved untouched.
func Merge(base []string, overrides map[string]string, goos string) []string {
	if len(overrides) == 0 {
		return base
	}

	// Overrides are matched by a single map lookup per environment entry rather
	// than by scanning every override for every entry. On Windows the lookup
	// table is folded to lower case so that one lookup still answers a
	// case-insensitive question.
	windows := goos == "windows"
	lookup := make(map[string]struct{}, len(overrides))
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
		if windows {
			lookup[strings.ToLower(key)] = struct{}{}
			continue
		}
		lookup[key] = struct{}{}
	}
	slices.Sort(keys)

	result := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if windows {
				key = strings.ToLower(key)
			}
			if _, overridden := lookup[key]; overridden {
				continue
			}
		}
		result = append(result, entry)
	}
	for _, key := range keys {
		result = append(result, key+"="+overrides[key])
	}
	return result
}

// WithPATH returns base with PATH replaced by pathValue, adding it when absent.
func WithPATH(base []string, pathValue string, goos string) []string {
	return Merge(base, map[string]string{"PATH": pathValue}, goos)
}
