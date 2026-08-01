package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"nodesmith/internal/recipe"
)

func Resolve(manifest recipe.Manifest, request ScaffoldRequest, resolver BinaryResolver) (Plan, error) {
	if err := recipe.Validate(manifest); err != nil {
		return Plan{}, fmt.Errorf("recipe %q is invalid: %w", manifest.ID, err)
	}
	if resolver == nil {
		return Plan{}, fmt.Errorf("resolve recipe %q: binary resolver is nil", manifest.ID)
	}
	if request.RecipeID != manifest.ID {
		return Plan{}, fmt.Errorf("recipe id mismatch: request has %q, manifest has %q", request.RecipeID, manifest.ID)
	}
	if len(manifest.Requires.PackageManagers) > 0 &&
		!slices.Contains(manifest.Requires.PackageManagers, request.PackageManager) {
		return Plan{}, fmt.Errorf(
			"package manager %q is not supported by recipe %q",
			request.PackageManager,
			manifest.ID,
		)
	}

	projectDir := filepath.Join(request.ParentDir, request.ProjectName)
	values, err := resolveValues(manifest, request, projectDir)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{
		RecipeID:   manifest.ID,
		ProjectDir: projectDir,
		Steps:      make([]PlanStep, 0, len(manifest.Steps)),
		Warnings:   []string{},
	}
	if manifest.InstallPolicy == recipe.InstallRequired && !request.InstallDeps {
		plan.Warnings = append(
			plan.Warnings,
			"This generator installs dependencies during scaffolding; dependency installation cannot be disabled.",
		)
	}

	// Families reached across the whole plan, so a cooldown that cannot be
	// applied exactly is reported once instead of once per step.
	planFamilies := make(map[string]struct{}, 4)
	pnpmBuildWarningAdded := false

	for _, step := range manifest.Steps {
		include, err := evaluateWhen(step.When, values)
		if err != nil {
			return Plan{}, fmt.Errorf("resolve step %q when: %w", step.ID, err)
		}
		if !include {
			continue
		}

		binaryName, err := substitute(step.Bin, values)
		if err != nil {
			return Plan{}, fmt.Errorf("resolve step %q binary: %w", step.ID, err)
		}
		var prefixArgs []string
		var binaryPath string
		if commands, ok := resolver.(commandResolver); ok {
			binaryPath, prefixArgs, err = commands.ResolveCommand(binaryName)
		} else {
			binaryPath, err = resolver.Resolve(binaryName)
		}
		if err != nil {
			return Plan{}, fmt.Errorf("resolve step %q binary %q: %w", step.ID, binaryName, err)
		}
		if strings.TrimSpace(binaryPath) == "" {
			return Plan{}, fmt.Errorf("resolve step %q binary %q: resolver returned an empty path", step.ID, binaryName)
		}

		args, err := expandArgNodes(step.Args, values, 0)
		if err != nil {
			return Plan{}, fmt.Errorf("resolve step %q args: %w", step.ID, err)
		}
		var pnpmBuildsRemainBlocked bool
		args, pnpmBuildsRemainBlocked = configurePnpmInstall(binaryName, args)
		if pnpmBuildsRemainBlocked && !pnpmBuildWarningAdded {
			plan.Warnings = append(plan.Warnings, pnpmBlockedBuildsWarning)
			pnpmBuildWarningAdded = true
		}
		if len(prefixArgs) > 0 {
			args = append(append([]string(nil), prefixArgs...), args...)
		}
		directory, err := resolveDirectory(step.CWD, request.ParentDir, projectDir)
		if err != nil {
			return Plan{}, fmt.Errorf("resolve step %q cwd: %w", step.ID, err)
		}

		environment := map[string]string{"CI": "true"}
		keys := make([]string, 0, len(step.Env))
		for key := range step.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if strings.EqualFold(key, "CI") || strings.EqualFold(key, "PATH") {
				continue
			}
			value, err := substitute(step.Env[key], values)
			if err != nil {
				return Plan{}, fmt.Errorf("resolve step %q environment %q: %w", step.ID, key, err)
			}
			environment[key] = value
		}

		// The step's own binary plus the selected package manager, because
		// generators commonly delegate installation to it.
		stepFamilies := make(map[string]struct{}, 2)
		for _, name := range []string{binaryName, request.PackageManager} {
			if family := binaryFamily(name); family != "" {
				stepFamilies[family] = struct{}{}
				planFamilies[family] = struct{}{}
			}
		}
		if request.MinimumReleaseAge != nil {
			// Applied after the recipe's own env so the resolved policy wins.
			for key, value := range releaseAgeEnvironment(*request.MinimumReleaseAge, stepFamilies) {
				environment[key] = value
			}
		}

		planStep := PlanStep{
			ID:    step.ID,
			Label: step.Label,
			Bin:   binaryPath,
			Args:  args,
			Dir:   directory,
			Env:   environment,
		}
		planStep.Display = displayCommand(planStep.Bin, planStep.Args)
		plan.Steps = append(plan.Steps, planStep)
	}

	if request.MinimumReleaseAge != nil {
		plan.Warnings = append(plan.Warnings, releaseAgeWarnings(*request.MinimumReleaseAge, planFamilies)...)
		if config := releaseAgeProjectConfig(request.PackageManager, *request.MinimumReleaseAge); config != nil {
			configStep := PlanStep{
				ID:      "minimum-release-age-config",
				Kind:    StepKindProjectConfig,
				Label:   "Write package-manager security policy",
				Dir:     projectDir,
				Env:     map[string]string{},
				Args:    []string{},
				Display: releaseAgeConfigDisplay(*config),
				Config:  config,
			}
			plan.Steps = insertProjectConfigStep(plan.Steps, configStep, projectDir)
		}
	}

	hash, err := hashSteps(plan.Steps)
	if err != nil {
		return Plan{}, fmt.Errorf("hash resolved steps: %w", err)
	}
	plan.Hash = hash
	return plan, nil
}

func insertProjectConfigStep(steps []PlanStep, configStep PlanStep, projectDir string) []PlanStep {
	insertAt := len(steps)
	cleanProjectDir := filepath.Clean(projectDir)
	// Recipes create the project from their first step. Insert before the first
	// later command that runs inside it (normally dependency installation).
	for index := 1; index < len(steps); index++ {
		if filepath.Clean(steps[index].Dir) == cleanProjectDir {
			insertAt = index
			break
		}
	}
	steps = append(steps, PlanStep{})
	copy(steps[insertAt+1:], steps[insertAt:])
	steps[insertAt] = configStep
	return steps
}

func resolveValues(manifest recipe.Manifest, request ScaffoldRequest, projectDir string) (map[string]any, error) {
	fields := make(map[string]recipe.Field, len(manifest.Fields))
	values := make(map[string]any, len(manifest.Fields)+6)
	for _, field := range manifest.Fields {
		fields[field.ID] = field
		values[field.ID] = cloneValue(field.Default)
	}

	answerIDs := make([]string, 0, len(request.Answers))
	for id := range request.Answers {
		answerIDs = append(answerIDs, id)
	}
	sort.Strings(answerIDs)
	for _, id := range answerIDs {
		field, exists := fields[id]
		if !exists {
			return nil, fmt.Errorf("answers.%s: field does not exist in recipe %q", id, manifest.ID)
		}
		value, err := validateAnswer(field, request.Answers[id])
		if err != nil {
			return nil, fmt.Errorf("answers.%s: %w", id, err)
		}
		values[id] = value
	}

	values["projectName"] = request.ProjectName
	values["projectDir"] = projectDir
	values["parentDir"] = request.ParentDir
	values["packageManager"] = request.PackageManager
	values["installDeps"] = request.InstallDeps
	values["gitInit"] = request.GitInit

	// Required is checked last, against the fully merged values, because
	// VisibleIf may reference any other field. A field the user never saw must
	// not block the plan.
	for _, field := range manifest.Fields {
		if !field.Required {
			continue
		}
		if _, answered := request.Answers[field.ID]; answered {
			continue
		}
		visible, err := evaluateWhen(field.VisibleIf, values)
		if err != nil {
			return nil, fmt.Errorf("fields.%s.visibleIf: %w", field.ID, err)
		}
		if visible {
			return nil, fmt.Errorf("answers.%s: %q requires an answer", field.ID, field.Label)
		}
	}
	return values, nil
}

func validateAnswer(field recipe.Field, value any) (any, error) {
	switch field.Type {
	case recipe.FieldSelect:
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("select answer must be a string, got %T", value)
		}
		if !optionExists(field.Options, text) {
			return nil, fmt.Errorf("select answer %q is not among the available options", text)
		}
		return text, nil
	case recipe.FieldMultiselect:
		values, ok := answerStringSlice(value)
		if !ok {
			return nil, fmt.Errorf("multiselect answer must be an array of strings, got %T", value)
		}
		seen := make(map[string]struct{}, len(values))
		for index, selected := range values {
			if !optionExists(field.Options, selected) {
				return nil, fmt.Errorf("multiselect answer at index %d (%q) is not among the available options", index, selected)
			}
			if _, duplicate := seen[selected]; duplicate {
				return nil, fmt.Errorf("multiselect answer contains duplicate value %q", selected)
			}
			seen[selected] = struct{}{}
		}
		return values, nil
	case recipe.FieldBoolean:
		boolean, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("boolean answer must be true or false, got %T", value)
		}
		return boolean, nil
	case recipe.FieldText:
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("text answer must be a string, got %T", value)
		}
		if err := recipe.CheckFieldConstraints(field, text); err != nil {
			return nil, err
		}
		return text, nil
	case recipe.FieldNumber:
		if !isNumber(value) {
			return nil, fmt.Errorf("number answer must be numeric, got %T", value)
		}
		if err := recipe.CheckFieldConstraints(field, value); err != nil {
			return nil, err
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported field type %q", field.Type)
	}
}

func evaluateWhen(expression string, values map[string]any) (bool, error) {
	if expression == "" {
		return true, nil
	}
	condition, err := recipe.ParseCondition(expression)
	if err != nil {
		return false, err
	}
	return condition.Evaluate(values)
}

func expandArgNodes(nodes []recipe.ArgNode, values map[string]any, depth int) ([]string, error) {
	args := make([]string, 0, len(nodes))
	for index, node := range nodes {
		switch {
		case node.Literal != nil:
			value, err := substitute(*node.Literal, values)
			if err != nil {
				return nil, fmt.Errorf("args[%d]: %w", index, err)
			}
			args = append(args, value)
		case node.Conditional != nil:
			nextDepth := depth + 1
			if nextDepth > 3 {
				return nil, fmt.Errorf("args[%d]: arg-node nesting depth exceeds 3", index)
			}
			condition, err := recipe.ParseCondition(node.Conditional.If)
			if err != nil {
				return nil, fmt.Errorf("args[%d].if: %w", index, err)
			}
			matches, err := condition.Evaluate(values)
			if err != nil {
				return nil, fmt.Errorf("args[%d].if: %w", index, err)
			}
			branch := node.Conditional.Else
			if matches {
				branch = node.Conditional.Then
			}
			expanded, err := expandArgNodes(branch, values, nextDepth)
			if err != nil {
				return nil, fmt.Errorf("args[%d]: %w", index, err)
			}
			args = append(args, expanded...)
		case node.Iteration != nil:
			nextDepth := depth + 1
			if nextDepth > 3 {
				return nil, fmt.Errorf("args[%d]: arg-node nesting depth exceeds 3", index)
			}
			items, ok := answerStringSlice(values[node.Iteration.Field])
			if !ok {
				return nil, fmt.Errorf("args[%d].forEach: %q is not a multiselect value", index, node.Iteration.Field)
			}
			for _, item := range items {
				iterationValues := make(map[string]any, len(values)+1)
				for key, value := range values {
					iterationValues[key] = value
				}
				iterationValues["item"] = item
				expanded, err := expandArgNodes(node.Iteration.Args, iterationValues, nextDepth)
				if err != nil {
					return nil, fmt.Errorf("args[%d].forEach %q: %w", index, item, err)
				}
				args = append(args, expanded...)
			}
		default:
			return nil, fmt.Errorf("args[%d]: arg node has no variant", index)
		}
	}
	return args, nil
}

func substitute(template string, values map[string]any) (string, error) {
	var builder strings.Builder
	remaining := template
	for {
		start := strings.Index(remaining, "${")
		if start < 0 {
			builder.WriteString(remaining)
			return builder.String(), nil
		}
		builder.WriteString(remaining[:start])
		closeIndex := strings.IndexByte(remaining[start+2:], '}')
		if closeIndex < 0 {
			return "", fmt.Errorf("unterminated variable reference")
		}
		closeIndex += start + 2
		identifier := remaining[start+2 : closeIndex]
		value, exists := values[identifier]
		if !exists {
			return "", fmt.Errorf("unknown identifier %q", identifier)
		}
		text, err := stringValue(value)
		if err != nil {
			return "", fmt.Errorf("substitute %q: %w", identifier, err)
		}
		builder.WriteString(text)
		remaining = remaining[closeIndex+1:]
	}
}

// stringValue renders a decoded value as a single argv element.
//
// Only the types encoding/json can actually produce are handled. Recipe
// manifests and user answers both reach this code through recipe.Decode, which
// calls UseNumber, so every number arrives as json.Number; float64 is retained
// because a caller constructing values without a decoder would still produce
// it, and its formatting feeds argv and therefore the plan hash. The int and
// uint arms this function used to carry were unreachable.
func stringValue(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case json.Number:
		if _, err := typed.Float64(); err != nil {
			return "", fmt.Errorf("invalid JSON number %q", typed)
		}
		return typed.String(), nil
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64), nil
	default:
		return "", fmt.Errorf("value of type %T cannot be substituted into one argv element", value)
	}
}

func resolveDirectory(cwd string, parentDir string, projectDir string) (string, error) {
	switch cwd {
	case "parentDir":
		return parentDir, nil
	case "projectDir":
		return projectDir, nil
	default:
		return "", fmt.Errorf("unknown cwd %q", cwd)
	}
}

func displayCommand(binary string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteForDisplay(binary))
	for _, arg := range args {
		parts = append(parts, quoteForDisplay(arg))
	}
	return strings.Join(parts, " ")
}

func quoteForDisplay(value string) string {
	if value != "" && displaySafe(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func displaySafe(value string) bool {
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character >= '0' && character <= '9':
		case strings.ContainsRune("_@%+=:,./-", character):
		default:
			return false
		}
	}
	return true
}

type hashEnvironment struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type hashStep struct {
	ID      string            `json:"id"`
	Kind    string            `json:"kind,omitempty"`
	Label   string            `json:"label"`
	Bin     string            `json:"bin"`
	Args    []string          `json:"args"`
	Dir     string            `json:"dir"`
	Env     []hashEnvironment `json:"env"`
	Display string            `json:"display"`
	Config  *ProjectConfig    `json:"config,omitempty"`
}

func hashSteps(steps []PlanStep) (string, error) {
	canonical := make([]hashStep, 0, len(steps))
	for _, step := range steps {
		keys := make([]string, 0, len(step.Env))
		for key := range step.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		environment := make([]hashEnvironment, 0, len(keys))
		for _, key := range keys {
			environment = append(environment, hashEnvironment{Key: key, Value: step.Env[key]})
		}
		canonical = append(canonical, hashStep{
			ID:      step.ID,
			Kind:    step.Kind,
			Label:   step.Label,
			Bin:     step.Bin,
			Args:    slices.Clone(step.Args),
			Dir:     step.Dir,
			Env:     environment,
			Display: step.Display,
			Config:  step.Config,
		})
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func optionExists(options []recipe.Option, wanted string) bool {
	for _, option := range options {
		if option.Value == wanted {
			return true
		}
	}
	return false
}

func answerStringSlice(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return slices.Clone(typed), true
	case []any:
		values := make([]string, len(typed))
		for index, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			values[index] = text
		}
		return values, true
	default:
		return nil, false
	}
}

// isNumber reports whether value is a numeric answer. A json.Number that does
// not parse is not a usable number, which is the one case that needs more than
// a type test.
func isNumber(value any) bool {
	switch typed := value.(type) {
	case json.Number:
		_, err := typed.Float64()
		return err == nil
	case float64:
		return true
	default:
		return false
	}
}

func cloneValue(value any) any {
	if values, ok := answerStringSlice(value); ok {
		return values
	}
	return value
}
