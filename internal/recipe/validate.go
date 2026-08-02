package recipe

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"nodesmith/internal/allowlist"
)

const maxArgObjectDepth = 3

var kebabCase = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var setupProjectDirectory = regexp.MustCompile(`^[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*$`)

var builtInVariables = map[string]struct{}{
	"projectName":    {},
	"projectDir":     {},
	"parentDir":      {},
	"packageManager": {},
	"installDeps":    {},
	"gitInit":        {},
}

var allowedPackageManagers = map[string]struct{}{
	"npm":  {},
	"pnpm": {},
	"yarn": {},
	"bun":  {},
}

var allowedCategories = map[string]struct{}{
	"frontend":  {},
	"fullstack": {},
	"backend":   {},
	"desktop":   {},
	"mobile":    {},
	"tooling":   {},
}

func Validate(manifest Manifest) error {
	if manifest.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("schemaVersion: unsupported version %d (want %d)", manifest.SchemaVersion, CurrentSchemaVersion)
	}
	if !kebabCase.MatchString(manifest.ID) {
		return fmt.Errorf("id: must be a non-empty kebab-case identifier")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return fmt.Errorf("name: must not be empty")
	}
	if _, ok := allowedCategories[manifest.Category]; !ok {
		return fmt.Errorf("category: unsupported value %q", manifest.Category)
	}
	if strings.TrimSpace(manifest.Description) == "" {
		return fmt.Errorf("description: must not be empty")
	}
	if err := validateDocsURL(manifest.DocsURL); err != nil {
		return fmt.Errorf("docsUrl: %w", err)
	}
	if manifest.Tags == nil {
		return fmt.Errorf("tags: must be an array")
	}
	seenTags := make(map[string]int, len(manifest.Tags))
	for index, tag := range manifest.Tags {
		if !kebabCase.MatchString(tag) {
			return fmt.Errorf("tags[%d]: must be a non-empty kebab-case identifier", index)
		}
		if previous, duplicate := seenTags[tag]; duplicate {
			return fmt.Errorf("tags[%d]: duplicate value %q (already used at index %d)", index, tag, previous)
		}
		seenTags[tag] = index
	}
	if strings.TrimSpace(manifest.Icon) == "" {
		return fmt.Errorf("icon: must not be empty")
	}
	if _, err := time.Parse("2006-01-02", manifest.VerifiedAt); err != nil {
		return fmt.Errorf("verifiedAt: must use YYYY-MM-DD: %w", err)
	}
	switch manifest.InstallPolicy {
	case "", InstallOptional, InstallRequired:
	default:
		return fmt.Errorf("installPolicy: unsupported value %q", manifest.InstallPolicy)
	}
	if manifest.MinimumReleaseAge != nil {
		if err := ValidateMinimumReleaseAge(*manifest.MinimumReleaseAge); err != nil {
			return fmt.Errorf("minimumReleaseAge: %w", err)
		}
	}
	if err := validateRequirements(manifest.Requires); err != nil {
		return err
	}
	if manifest.Fields == nil {
		return fmt.Errorf("fields: must be an array")
	}

	fields, err := validateFieldDefinitions(manifest.Fields)
	if err != nil {
		return err
	}
	if err := validateSetup(manifest.Setup, manifest.Fields); err != nil {
		return err
	}
	known := make(map[string]FieldType, len(fields)+len(builtInVariables))
	for identifier := range builtInVariables {
		known[identifier] = ""
	}
	for identifier, fieldType := range fields {
		known[identifier] = fieldType
	}

	for index, field := range manifest.Fields {
		if field.VisibleIf == "" {
			continue
		}
		path := fmt.Sprintf("fields[%d].visibleIf", index)
		if err := validateConditionReference(path, field.VisibleIf, known); err != nil {
			return err
		}
	}

	if len(manifest.Steps) == 0 {
		return fmt.Errorf("steps: must contain at least one step")
	}
	stepIDs := make(map[string]int, len(manifest.Steps))
	for index, step := range manifest.Steps {
		path := fmt.Sprintf("steps[%d]", index)
		if !kebabCase.MatchString(step.ID) {
			return fmt.Errorf("%s.id: must be a non-empty kebab-case identifier", path)
		}
		if previous, exists := stepIDs[step.ID]; exists {
			return fmt.Errorf("%s.id: duplicate id %q (already used at steps[%d].id)", path, step.ID, previous)
		}
		stepIDs[step.ID] = index
		if strings.TrimSpace(step.Label) == "" {
			return fmt.Errorf("%s.label: must not be empty", path)
		}
		if err := validateBinary(path+".bin", step.Bin, known); err != nil {
			return err
		}
		if step.CWD != "parentDir" && step.CWD != "projectDir" {
			return fmt.Errorf("%s.cwd: must be %q or %q", path, "parentDir", "projectDir")
		}
		if err := validateStepEnvironment(path+".env", step.Env, known); err != nil {
			return err
		}
		if step.Env["CI"] != "1" {
			return fmt.Errorf("%s.env.CI: must be %q for non-interactive execution", path, "1")
		}
		if step.Args == nil {
			return fmt.Errorf("%s.args: must be an array", path)
		}
		if step.When != "" {
			if err := validateConditionReference(path+".when", step.When, known); err != nil {
				return err
			}
		}
		if err := validateArgNodes(step.Args, path+".args", known, false, 0); err != nil {
			return err
		}
	}

	return nil
}

func validateSetup(setup Setup, fields []Field) error {
	if setup.NodeProjectDir != "" {
		valid := setup.NodeTooling && setupProjectDirectory.MatchString(setup.NodeProjectDir)
		for _, segment := range strings.Split(setup.NodeProjectDir, "/") {
			valid = valid && segment != "." && segment != ".."
		}
		if !valid {
			return fmt.Errorf("setup.nodeProjectDir: must be a relative project subdirectory used with nodeTooling")
		}
	}
	byID := make(map[string]Field, len(fields))
	for _, field := range fields {
		byID[field.ID] = field
	}
	if setup.NodeTooling {
		lintingField := "linting"
		if _, exists := byID[lintingField]; !exists {
			if _, legacyExists := byID["linter"]; legacyExists {
				lintingField = "linter"
			}
		}
		if err := validateSetupSelect(byID, lintingField, []string{"eslint", "oxlint", "biome"}); err != nil {
			return fmt.Errorf("setup.nodeTooling: %w", err)
		}
		if err := validateSetupSelect(byID, "formatting", []string{"prettier", "oxfmt"}); err != nil {
			return fmt.Errorf("setup.nodeTooling: %w", err)
		}
	}
	if setup.ExpoStyling {
		if err := validateSetupSelect(byID, "styling", []string{"uniwind", "nativewind", "unistyles"}); err != nil {
			return fmt.Errorf("setup.expoStyling: %w", err)
		}
		if err := validateSetupSelect(byID, "template", []string{"default", "blank", "blank-typescript", "tabs", "bare-minimum"}); err != nil {
			return fmt.Errorf("setup.expoStyling: %w", err)
		}
	}
	return nil
}

func validateSetupSelect(fields map[string]Field, id string, expected []string) error {
	field, exists := fields[id]
	if !exists {
		return fmt.Errorf("requires a %q field", id)
	}
	if field.Type != FieldSelect {
		return fmt.Errorf("field %q must have type %q", id, FieldSelect)
	}
	actual := make([]string, 0, len(field.Options))
	for _, option := range field.Options {
		actual = append(actual, option.Value)
	}
	slices.Sort(actual)
	wanted := slices.Clone(expected)
	slices.Sort(wanted)
	if !slices.Equal(actual, wanted) {
		return fmt.Errorf("field %q options must be %s", id, strings.Join(expected, ", "))
	}
	return nil
}

func validateStepEnvironment(path string, environment map[string]string, known map[string]FieldType) error {
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	seen := make(map[string]string, len(keys))
	for _, key := range keys {
		if key == "" || strings.ContainsAny(key, "=\x00") {
			return fmt.Errorf("%s: invalid environment variable name %q", path, key)
		}
		if strings.ContainsRune(environment[key], '\x00') {
			return fmt.Errorf("%s.%s: value contains a NUL byte", path, key)
		}
		variables, err := Variables(environment[key])
		if err != nil {
			return fmt.Errorf("%s.%s: %w", path, key, err)
		}
		for _, identifier := range variables {
			if _, exists := known[identifier]; !exists {
				return fmt.Errorf("%s.%s: unknown identifier %q", path, key, identifier)
			}
		}
		folded := strings.ToUpper(key)
		if previous, duplicate := seen[folded]; duplicate {
			return fmt.Errorf(
				"%s.%s: duplicates %q on case-insensitive platforms",
				path,
				key,
				previous,
			)
		}
		seen[folded] = key
		if strings.EqualFold(key, "PATH") {
			return fmt.Errorf("%s.%s: PATH is managed by Nodesmith", path, key)
		}
	}
	return nil
}

func validateDocsURL(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return fmt.Errorf("must be an absolute HTTP(S) URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" || parsed.Host == "" {
		return fmt.Errorf("must be an absolute HTTP(S) URL")
	}
	return nil
}

func validateRequirements(requirements Requirements) error {
	if strings.TrimSpace(requirements.Node) == "" {
		return fmt.Errorf("requires.node: must not be empty")
	}
	if requirements.PackageManagers == nil {
		return fmt.Errorf("requires.packageManagers: must be an array")
	}
	if requirements.Tools == nil {
		return fmt.Errorf("requires.tools: must be an array")
	}
	seenManagers := make(map[string]int, len(requirements.PackageManagers))
	for index, manager := range requirements.PackageManagers {
		if _, ok := allowedPackageManagers[manager]; !ok {
			return fmt.Errorf("requires.packageManagers[%d]: unsupported package manager %q", index, manager)
		}
		if previous, exists := seenManagers[manager]; exists {
			return fmt.Errorf("requires.packageManagers[%d]: duplicate value %q (already used at index %d)", index, manager, previous)
		}
		seenManagers[manager] = index
	}
	seenTools := make(map[string]int, len(requirements.Tools))
	for index, tool := range requirements.Tools {
		if !allowlist.IsAllowed(tool) {
			return fmt.Errorf("requires.tools[%d]: binary %q is not in the allowlist", index, tool)
		}
		if previous, exists := seenTools[tool]; exists {
			return fmt.Errorf("requires.tools[%d]: duplicate value %q (already used at index %d)", index, tool, previous)
		}
		seenTools[tool] = index
	}
	return nil
}

func validateFieldDefinitions(fields []Field) (map[string]FieldType, error) {
	known := make(map[string]FieldType, len(fields))
	positions := make(map[string]int, len(fields))
	for index, field := range fields {
		path := fmt.Sprintf("fields[%d]", index)
		if _, shadows := builtInVariables[field.ID]; shadows {
			return nil, fmt.Errorf("%s.id: field id %q shadows a built-in variable", path, field.ID)
		}
		if !kebabCase.MatchString(field.ID) {
			return nil, fmt.Errorf("%s.id: must be a non-empty kebab-case identifier", path)
		}
		if previous, duplicate := positions[field.ID]; duplicate {
			return nil, fmt.Errorf("%s.id: duplicate id %q (already used at fields[%d].id)", path, field.ID, previous)
		}
		positions[field.ID] = index
		if strings.TrimSpace(field.Label) == "" {
			return nil, fmt.Errorf("%s.label: must not be empty", path)
		}
		switch field.Type {
		case FieldSelect, FieldMultiselect, FieldBoolean, FieldText, FieldNumber:
		default:
			return nil, fmt.Errorf("%s.type: unsupported value %q", path, field.Type)
		}
		if err := validateFieldDefault(path, field); err != nil {
			return nil, err
		}
		if err := validateFieldConstraints(path, field); err != nil {
			return nil, err
		}
		known[field.ID] = field.Type
	}
	return known, nil
}

func validateFieldDefault(path string, field Field) error {
	optionValues := make([]string, 0, len(field.Options))
	seenOptions := make(map[string]int, len(field.Options))
	for index, option := range field.Options {
		if option.Value == "" {
			return fmt.Errorf("%s.options[%d].value: must not be empty", path, index)
		}
		if previous, duplicate := seenOptions[option.Value]; duplicate {
			return fmt.Errorf("%s.options[%d].value: duplicate value %q (already used at index %d)", path, index, option.Value, previous)
		}
		if strings.TrimSpace(option.Label) == "" {
			return fmt.Errorf("%s.options[%d].label: must not be empty", path, index)
		}
		seenOptions[option.Value] = index
		optionValues = append(optionValues, option.Value)
	}

	switch field.Type {
	case FieldSelect:
		defaultValue, ok := field.Default.(string)
		if !ok {
			return fmt.Errorf("%s.default: select default must be a string", path)
		}
		if !slices.Contains(optionValues, defaultValue) {
			return fmt.Errorf("%s.default: select default %q is not among options", path, defaultValue)
		}
	case FieldMultiselect:
		defaultValues, ok := stringSlice(field.Default)
		if !ok {
			return fmt.Errorf("%s.default: multiselect default must be an array of strings", path)
		}
		for index, value := range defaultValues {
			if !slices.Contains(optionValues, value) {
				return fmt.Errorf("%s.default[%d]: value %q is not among options", path, index, value)
			}
		}
	case FieldBoolean:
		if _, ok := field.Default.(bool); !ok {
			return fmt.Errorf("%s.default: boolean default must be true or false", path)
		}
	case FieldText:
		if _, ok := field.Default.(string); !ok {
			return fmt.Errorf("%s.default: text default must be a string", path)
		}
	case FieldNumber:
		if _, ok := numericValue(field.Default); !ok {
			return fmt.Errorf("%s.default: number default must be numeric", path)
		}
	}
	return nil
}

func validateConditionReference(path string, expression string, known map[string]FieldType) error {
	condition, err := ParseCondition(expression)
	if err != nil {
		return fmt.Errorf("%s: expression parse error: %w", path, err)
	}
	fieldType, exists := known[condition.Identifier]
	if !exists {
		return fmt.Errorf("%s: unknown identifier %q", path, condition.Identifier)
	}
	if condition.Operator == ConditionIncludes && fieldType != FieldMultiselect {
		return fmt.Errorf("%s: %q requires a multiselect identifier, got %q", path, ConditionIncludes, condition.Identifier)
	}
	return nil
}

func validateBinary(path string, binary string, known map[string]FieldType) error {
	variables, err := Variables(binary)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for _, identifier := range variables {
		if _, exists := known[identifier]; !exists {
			return fmt.Errorf("%s: unknown identifier %q", path, identifier)
		}
	}
	if binary == "${packageManager}" {
		return nil
	}
	if !allowlist.IsAllowed(binary) {
		return fmt.Errorf("%s: binary %q is not in the allowlist", path, binary)
	}
	return nil
}

func validateArgNodes(nodes []ArgNode, path string, known map[string]FieldType, itemAvailable bool, objectDepth int) error {
	for index, node := range nodes {
		nodePath := fmt.Sprintf("%s[%d]", path, index)
		variantCount := 0
		if node.Literal != nil {
			variantCount++
		}
		if node.Conditional != nil {
			variantCount++
		}
		if node.Iteration != nil {
			variantCount++
		}
		if variantCount != 1 {
			return fmt.Errorf("%s: arg node must contain exactly one variant", nodePath)
		}

		switch {
		case node.Literal != nil:
			variables, err := Variables(*node.Literal)
			if err != nil {
				return fmt.Errorf("%s: %w", nodePath, err)
			}
			for _, identifier := range variables {
				if identifier == "item" && itemAvailable {
					continue
				}
				if _, exists := known[identifier]; !exists {
					return fmt.Errorf("%s: unknown identifier %q", nodePath, identifier)
				}
			}
		case node.Conditional != nil:
			nextDepth := objectDepth + 1
			if nextDepth > maxArgObjectDepth {
				return fmt.Errorf("%s: arg-node nesting depth exceeds %d", nodePath, maxArgObjectDepth)
			}
			if err := validateConditionReference(nodePath+".if", node.Conditional.If, known); err != nil {
				return err
			}
			if err := validateArgNodes(node.Conditional.Then, nodePath+".then", known, itemAvailable, nextDepth); err != nil {
				return err
			}
			if err := validateArgNodes(node.Conditional.Else, nodePath+".else", known, itemAvailable, nextDepth); err != nil {
				return err
			}
		case node.Iteration != nil:
			nextDepth := objectDepth + 1
			if nextDepth > maxArgObjectDepth {
				return fmt.Errorf("%s: arg-node nesting depth exceeds %d", nodePath, maxArgObjectDepth)
			}
			fieldType, exists := known[node.Iteration.Field]
			if !exists {
				return fmt.Errorf("%s.forEach: unknown identifier %q", nodePath, node.Iteration.Field)
			}
			if fieldType != FieldMultiselect {
				return fmt.Errorf("%s.forEach: identifier %q is not a multiselect", nodePath, node.Iteration.Field)
			}
			if err := validateArgNodes(node.Iteration.Args, nodePath+".args", known, true, nextDepth); err != nil {
				return err
			}
		}
	}
	return nil
}

func stringSlice(value any) ([]string, bool) {
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
