package recipe

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const CurrentSchemaVersion = 1

type Manifest struct {
	SchemaVersion int           `json:"schemaVersion"`
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Category      string        `json:"category"`
	Description   string        `json:"description"`
	DocsURL       string        `json:"docsUrl"`
	Tags          []string      `json:"tags"`
	Icon          string        `json:"icon"`
	VerifiedAt    string        `json:"verifiedAt"`
	InstallPolicy InstallPolicy `json:"installPolicy,omitempty"`
	// MinimumReleaseAge is the recipe-local package cooldown in minutes. A nil
	// pointer means the recipe expresses no opinion and the global preference
	// applies; zero explicitly disables the cooldown for this recipe.
	MinimumReleaseAge *int         `json:"minimumReleaseAge,omitempty"`
	Requires          Requirements `json:"requires"`
	Fields            []Field      `json:"fields"`
	Steps             []Step       `json:"steps"`
}

// MaxMinimumReleaseAge caps a cooldown at one year of minutes so an obvious
// unit mix-up (seconds or milliseconds) is rejected instead of silently
// blocking every install.
const MaxMinimumReleaseAge = 525600

// ValidateMinimumReleaseAge reports whether minutes is a usable cooldown. It is
// shared by manifest validation and the local settings store so recipe authors
// and users are held to the same range.
func ValidateMinimumReleaseAge(minutes int) error {
	if minutes < 0 {
		return fmt.Errorf("must not be negative, got %d", minutes)
	}
	if minutes > MaxMinimumReleaseAge {
		return fmt.Errorf("must be at most %d minutes (one year), got %d", MaxMinimumReleaseAge, minutes)
	}
	return nil
}

// InstallPolicy declares whether a generator can honour installDeps=false.
// The zero value is treated as optional for backwards-compatible user recipes.
type InstallPolicy string

const (
	InstallOptional InstallPolicy = "optional"
	InstallRequired InstallPolicy = "required"
)

type Requirements struct {
	Node            string   `json:"node"`
	PackageManagers []string `json:"packageManagers"`
	Tools           []string `json:"tools"`
}

type Field struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Type      FieldType `json:"type"`
	Default   any       `json:"default"`
	Help      string    `json:"help,omitempty"`
	Options   []Option  `json:"options,omitempty"`
	VisibleIf string    `json:"visibleIf,omitempty"`

	// Value constraints. All are optional and all are additive, so manifests
	// written before they existed stay valid and CurrentSchemaVersion is
	// unchanged.
	//
	// These do not exist to prevent command injection — execution is argv-only
	// with no shell, and every binary is allowlisted and resolved to an absolute
	// path. They exist because an unconstrained answer is substituted straight
	// into argv, where a value beginning with "-" is read by the generator as a
	// flag, an empty string arrives where a token is expected, and a
	// several-kilobyte value can trip an OS argv limit.

	// Required makes an explicit answer mandatory instead of silently falling
	// back to Default. It is enforced only when the field is visible, so a field
	// hidden by VisibleIf never blocks a plan.
	Required bool `json:"required,omitempty"`
	// Pattern is an RE2 expression a text answer must match. It is compiled when
	// the manifest loads, not per answer. It is unanchored: use ^ and $ to match
	// the whole value.
	Pattern string `json:"pattern,omitempty"`
	// MinLength and MaxLength bound a text answer in UTF-8 runes, not bytes.
	MinLength *int `json:"minLength,omitempty"`
	MaxLength *int `json:"maxLength,omitempty"`
	// Min and Max bound a number answer inclusively.
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

type FieldType string

const (
	FieldSelect      FieldType = "select"
	FieldMultiselect FieldType = "multiselect"
	FieldBoolean     FieldType = "boolean"
	FieldText        FieldType = "text"
	FieldNumber      FieldType = "number"
)

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Step struct {
	ID    string            `json:"id"`
	Label string            `json:"label"`
	Bin   string            `json:"bin"`
	CWD   string            `json:"cwd"`
	Env   map[string]string `json:"env"`
	Args  []ArgNode         `json:"args"`
	When  string            `json:"when,omitempty"`
}

// ArgNode is the closed union described by the manifest arg grammar. Exactly
// one of Literal, Conditional, or Iteration is non-nil.
type ArgNode struct {
	Literal     *string
	Conditional *ConditionalArg
	Iteration   *ForEachArg
}

type ConditionalArg struct {
	If   string
	Then []ArgNode
	Else []ArgNode
}

type ForEachArg struct {
	Field string
	Args  []ArgNode
}

func (node *ArgNode) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("arg node is empty")
	}

	if trimmed[0] == '"' {
		var literal string
		if err := decodeStrict(bytes.NewReader(trimmed), &literal); err != nil {
			return fmt.Errorf("decode literal arg: %w", err)
		}
		*node = ArgNode{Literal: &literal}
		return nil
	}

	if trimmed[0] != '{' {
		return fmt.Errorf("arg node must be a string, conditional object, or forEach object")
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &keys); err != nil {
		return fmt.Errorf("decode arg object: %w", err)
	}

	_, hasIf := keys["if"]
	_, hasForEach := keys["forEach"]
	if hasIf == hasForEach {
		return fmt.Errorf("arg object must contain exactly one of %q or %q", "if", "forEach")
	}

	if hasIf {
		var raw struct {
			If   *string    `json:"if"`
			Then *[]ArgNode `json:"then"`
			Else *[]ArgNode `json:"else,omitempty"`
		}
		if err := decodeStrict(bytes.NewReader(trimmed), &raw); err != nil {
			return fmt.Errorf("decode conditional arg: %w", err)
		}
		if raw.If == nil {
			return fmt.Errorf("conditional arg requires %q", "if")
		}
		if raw.Then == nil {
			return fmt.Errorf("conditional arg requires %q", "then")
		}
		conditional := &ConditionalArg{If: *raw.If, Then: *raw.Then}
		if raw.Else != nil {
			conditional.Else = *raw.Else
		}
		*node = ArgNode{Conditional: conditional}
		return nil
	}

	var raw struct {
		ForEach *string    `json:"forEach"`
		Args    *[]ArgNode `json:"args"`
	}
	if err := decodeStrict(bytes.NewReader(trimmed), &raw); err != nil {
		return fmt.Errorf("decode forEach arg: %w", err)
	}
	if raw.ForEach == nil {
		return fmt.Errorf("forEach arg requires %q", "forEach")
	}
	if raw.Args == nil {
		return fmt.Errorf("forEach arg requires %q", "args")
	}
	*node = ArgNode{Iteration: &ForEachArg{Field: *raw.ForEach, Args: *raw.Args}}
	return nil
}

func (node ArgNode) MarshalJSON() ([]byte, error) {
	switch {
	case node.Literal != nil && node.Conditional == nil && node.Iteration == nil:
		return json.Marshal(*node.Literal)
	case node.Literal == nil && node.Conditional != nil && node.Iteration == nil:
		value := struct {
			If   string    `json:"if"`
			Then []ArgNode `json:"then"`
			Else []ArgNode `json:"else,omitempty"`
		}{
			If:   node.Conditional.If,
			Then: node.Conditional.Then,
			Else: node.Conditional.Else,
		}
		return json.Marshal(value)
	case node.Literal == nil && node.Conditional == nil && node.Iteration != nil:
		value := struct {
			ForEach string    `json:"forEach"`
			Args    []ArgNode `json:"args"`
		}{
			ForEach: node.Iteration.Field,
			Args:    node.Iteration.Args,
		}
		return json.Marshal(value)
	default:
		return nil, fmt.Errorf("arg node must contain exactly one variant")
	}
}

func Literal(value string) ArgNode {
	return ArgNode{Literal: &value}
}
