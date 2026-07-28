package toolchain

import (
	"fmt"
	"strings"
)

// Requirements mirrors the tool-related portion of a recipe without coupling
// the toolchain package to the recipe package.
type Requirements struct {
	Node            string   `json:"node,omitempty"`
	PackageManagers []string `json:"packageManagers,omitempty"`
	Tools           []string `json:"tools,omitempty"`
}

// GateResult explains whether a recipe can run on a detected toolchain.
type GateResult struct {
	Available      bool     `json:"available"`
	Reasons        []string `json:"reasons"`
	PackageManager string   `json:"packageManager,omitempty"`
}

// EvaluateRequirements checks node version, package-manager availability, and
// additional required tools. Reasons are deterministic and suitable for
// disabling a recipe's run action with actionable feedback.
func EvaluateRequirements(toolchain Toolchain, requirements Requirements) GateResult {
	result := GateResult{Available: true, Reasons: []string{}}

	if requirements.Node != "" {
		node, found := toolchain.Lookup("node")
		switch {
		case !found:
			result.Reasons = append(
				result.Reasons,
				fmt.Sprintf("node %s is required but node was not found", requirements.Node),
			)
		case node.Error != "":
			result.Reasons = append(
				result.Reasons,
				"node was found but could not be used: "+node.Error,
			)
		case !node.Present:
			result.Reasons = append(
				result.Reasons,
				fmt.Sprintf("node %s is required but node was not found", requirements.Node),
			)
		case node.Version == "":
			result.Reasons = append(
				result.Reasons,
				"node was found but its version could not be determined",
			)
		default:
			satisfied, err := SatisfiesRange(node.Version, requirements.Node)
			if err != nil {
				result.Reasons = append(
					result.Reasons,
					fmt.Sprintf("node requirement %q is invalid: %v", requirements.Node, err),
				)
			} else if !satisfied {
				result.Reasons = append(
					result.Reasons,
					fmt.Sprintf(
						"node %s does not satisfy required version %s",
						node.Version,
						requirements.Node,
					),
				)
			}
		}
	}

	for _, name := range requirements.Tools {
		tool, found := toolchain.Lookup(name)
		switch {
		case !found:
			result.Reasons = append(
				result.Reasons,
				fmt.Sprintf("required tool %s was not found", name),
			)
		case tool.Error != "":
			result.Reasons = append(
				result.Reasons,
				fmt.Sprintf("required tool %s could not be used: %s", name, tool.Error),
			)
		case !tool.Present:
			result.Reasons = append(
				result.Reasons,
				fmt.Sprintf("required tool %s was not found", name),
			)
		case tool.Version == "":
			result.Reasons = append(
				result.Reasons,
				fmt.Sprintf("required tool %s has no usable version", name),
			)
		}
	}

	if len(requirements.PackageManagers) > 0 {
		unusable := make([]string, 0, len(requirements.PackageManagers))
		for _, name := range requirements.PackageManagers {
			tool, found := toolchain.Lookup(name)
			switch {
			case !found:
				unusable = append(unusable, name+" (not found)")
			case tool.Error != "":
				unusable = append(unusable, name+" ("+tool.Error+")")
			case !tool.Present:
				unusable = append(unusable, name+" (not found)")
			case tool.Version == "":
				unusable = append(unusable, name+" (version unavailable)")
			default:
				result.PackageManager = name
			}
			if result.PackageManager != "" {
				break
			}
		}
		if result.PackageManager == "" {
			result.Reasons = append(
				result.Reasons,
				"no supported package manager is usable: "+strings.Join(unusable, ", "),
			)
		}
	}

	result.Available = len(result.Reasons) == 0
	return result
}
