package toolchain

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	semverTokenPattern = regexp.MustCompile(`(?i)v?[0-9]+(?:\.[0-9]+){1,2}(?:-[0-9a-z.-]+)?(?:\+[0-9a-z.-]+)?`)
	prefixedPatterns   = map[string]*regexp.Regexp{
		"git":   regexp.MustCompile(`(?im)\bgit version\s+(v?[0-9]+(?:\.[0-9]+){1,2}(?:-[0-9a-z.-]+)?(?:\+[0-9a-z.-]+)?)`),
		"go":    regexp.MustCompile(`(?im)\bgo version go(v?[0-9]+(?:\.[0-9]+){1,2}(?:-[0-9a-z.-]+)?(?:\+[0-9a-z.-]+)?)`),
		"cargo": regexp.MustCompile(`(?im)\bcargo\s+(v?[0-9]+(?:\.[0-9]+){1,2}(?:-[0-9a-z.-]+)?(?:\+[0-9a-z.-]+)?)`),
		"gh":    regexp.MustCompile(`(?im)\bgh version\s+(v?[0-9]+(?:\.[0-9]+){1,2}(?:-[0-9a-z.-]+)?(?:\+[0-9a-z.-]+)?)`),
	}
)

// SemanticVersion is a comparable semantic version. Build metadata is parsed
// but intentionally excluded from ordering.
type SemanticVersion struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

// ParseSemanticVersion parses a semantic version with an optional v prefix.
// Two-component tool versions are accepted and normalized with a zero patch.
func ParseSemanticVersion(value string) (SemanticVersion, error) {
	original := value
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	if value == "" {
		return SemanticVersion{}, errors.New("semantic version is empty")
	}

	var build string
	if before, after, found := strings.Cut(value, "+"); found {
		value = before
		build = after
		if !validIdentifiers(build) {
			return SemanticVersion{}, fmt.Errorf("invalid build metadata in %q", original)
		}
	}

	var prerelease string
	if before, after, found := strings.Cut(value, "-"); found {
		value = before
		prerelease = after
		if !validIdentifiers(prerelease) {
			return SemanticVersion{}, fmt.Errorf("invalid prerelease in %q", original)
		}
	}

	parts := strings.Split(value, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return SemanticVersion{}, fmt.Errorf("invalid semantic version %q", original)
	}
	numbers := [3]int{}
	for index, part := range parts {
		if part == "" {
			return SemanticVersion{}, fmt.Errorf("invalid semantic version %q", original)
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return SemanticVersion{}, fmt.Errorf("invalid semantic version %q", original)
		}
		numbers[index] = number
	}

	return SemanticVersion{
		Major:      numbers[0],
		Minor:      numbers[1],
		Patch:      numbers[2],
		Prerelease: prerelease,
		Build:      build,
	}, nil
}

func validIdentifiers(value string) bool {
	if value == "" {
		return false
	}
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" {
			return false
		}
		for _, character := range identifier {
			if (character < '0' || character > '9') &&
				(character < 'A' || character > 'Z') &&
				(character < 'a' || character > 'z') &&
				character != '-' {
				return false
			}
		}
	}
	return true
}

func (version SemanticVersion) String() string {
	value := fmt.Sprintf("%d.%d.%d", version.Major, version.Minor, version.Patch)
	if version.Prerelease != "" {
		value += "-" + version.Prerelease
	}
	if version.Build != "" {
		value += "+" + version.Build
	}
	return value
}

// Compare returns -1, 0, or 1 when version sorts before, equals, or sorts
// after other according to semantic-version precedence.
func (version SemanticVersion) Compare(other SemanticVersion) int {
	left := [...]int{version.Major, version.Minor, version.Patch}
	right := [...]int{other.Major, other.Minor, other.Patch}
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return comparePrerelease(version.Prerelease, other.Prerelease)
}

func comparePrerelease(left string, right string) int {
	if left == right {
		return 0
	}
	if left == "" {
		return 1
	}
	if right == "" {
		return -1
	}

	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	count := min(len(leftParts), len(rightParts))
	for index := 0; index < count; index++ {
		leftNumber, leftErr := strconv.Atoi(leftParts[index])
		rightNumber, rightErr := strconv.Atoi(rightParts[index])
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNumber < rightNumber {
				return -1
			}
			if leftNumber > rightNumber {
				return 1
			}
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		default:
			if leftParts[index] < rightParts[index] {
				return -1
			}
			if leftParts[index] > rightParts[index] {
				return 1
			}
		}
	}
	if len(leftParts) < len(rightParts) {
		return -1
	}
	return 1
}

// SatisfiesRange evaluates an AND-only semantic version range. Supported
// comparators are >=, >, <=, <, =, ==, and an exact bare version.
func SatisfiesRange(version string, constraint string) (bool, error) {
	parsedVersion, err := ParseSemanticVersion(version)
	if err != nil {
		return false, fmt.Errorf("parse detected version: %w", err)
	}
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return true, nil
	}
	if strings.Contains(constraint, "||") {
		return false, fmt.Errorf("unsupported semantic version range %q: OR is not supported", constraint)
	}

	tokens := strings.Fields(strings.ReplaceAll(constraint, ",", " "))
	clauses := make([]string, 0, len(tokens))
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if isComparator(token) {
			if index+1 >= len(tokens) {
				return false, fmt.Errorf("missing version after %q", token)
			}
			index++
			token += tokens[index]
		}
		clauses = append(clauses, token)
	}

	for _, clause := range clauses {
		operator, requiredText, err := splitComparator(clause)
		if err != nil {
			return false, err
		}
		required, err := ParseSemanticVersion(requiredText)
		if err != nil {
			return false, fmt.Errorf("parse range clause %q: %w", clause, err)
		}
		comparison := parsedVersion.Compare(required)
		matches := false
		switch operator {
		case ">=":
			matches = comparison >= 0
		case ">":
			matches = comparison > 0
		case "<=":
			matches = comparison <= 0
		case "<":
			matches = comparison < 0
		case "=", "==":
			matches = comparison == 0
		}
		if !matches {
			return false, nil
		}
	}
	return true, nil
}

func isComparator(value string) bool {
	switch value {
	case ">=", ">", "<=", "<", "=", "==":
		return true
	default:
		return false
	}
}

func splitComparator(clause string) (string, string, error) {
	for _, operator := range []string{">=", "<=", "==", ">", "<", "="} {
		if strings.HasPrefix(clause, operator) {
			version := strings.TrimPrefix(clause, operator)
			if version == "" {
				return "", "", fmt.Errorf("missing version in range clause %q", clause)
			}
			return operator, version, nil
		}
	}
	if strings.HasPrefix(clause, "^") || strings.HasPrefix(clause, "~") {
		return "", "", fmt.Errorf("unsupported semantic version range clause %q", clause)
	}
	return "=", clause, nil
}

// ParseToolVersion extracts and normalizes a version from a tool's --version
// output.
func ParseToolVersion(tool string, output string) (string, error) {
	var token string
	if pattern, ok := prefixedPatterns[tool]; ok {
		match := pattern.FindStringSubmatch(output)
		if len(match) == 2 {
			token = match[1]
		}
	} else {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for index := len(lines) - 1; index >= 0; index-- {
			if match := semverTokenPattern.FindString(lines[index]); match != "" {
				token = match
				break
			}
		}
	}
	if token == "" {
		return "", fmt.Errorf("could not parse %s version from %q", tool, strings.TrimSpace(output))
	}
	version, err := ParseSemanticVersion(token)
	if err != nil {
		return "", fmt.Errorf("parse %s version: %w", tool, err)
	}
	return version.String(), nil
}
