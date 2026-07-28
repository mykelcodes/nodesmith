package recipe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type ConditionOperator string

const (
	ConditionTruthy   ConditionOperator = ""
	ConditionEqual    ConditionOperator = "=="
	ConditionUnequal  ConditionOperator = "!="
	ConditionIncludes ConditionOperator = "includes"
)

type Condition struct {
	Identifier string
	Operator   ConditionOperator
	Literal    any
	Negated    bool
}

func ParseCondition(expression string) (Condition, error) {
	parser := conditionParser{source: expression}
	parser.skipSpace()

	negated := false
	if parser.peek("!") && !parser.peek("!=") {
		negated = true
		parser.position++
		parser.skipSpace()
	}

	identifier, err := parser.identifier()
	if err != nil {
		return Condition{}, fmt.Errorf("parse condition %q: %w", expression, err)
	}
	parser.skipSpace()

	if parser.done() {
		return Condition{Identifier: identifier, Operator: ConditionTruthy, Negated: negated}, nil
	}
	if negated {
		return Condition{}, fmt.Errorf("parse condition %q: negation only accepts an identifier", expression)
	}

	operator, err := parser.operator()
	if err != nil {
		return Condition{}, fmt.Errorf("parse condition %q: %w", expression, err)
	}
	parser.skipSpace()
	literalText := strings.TrimSpace(parser.source[parser.position:])
	if literalText == "" {
		return Condition{}, fmt.Errorf("parse condition %q: missing literal", expression)
	}
	literal, err := parseConditionLiteral(literalText)
	if err != nil {
		return Condition{}, fmt.Errorf("parse condition %q: %w", expression, err)
	}

	return Condition{Identifier: identifier, Operator: operator, Literal: literal}, nil
}

func (condition Condition) Evaluate(values map[string]any) (bool, error) {
	value, ok := values[condition.Identifier]
	if !ok {
		return false, fmt.Errorf("unknown identifier %q", condition.Identifier)
	}

	switch condition.Operator {
	case ConditionTruthy:
		result := truthy(value)
		if condition.Negated {
			result = !result
		}
		return result, nil
	case ConditionEqual:
		return valuesEqual(value, condition.Literal), nil
	case ConditionUnequal:
		return !valuesEqual(value, condition.Literal), nil
	case ConditionIncludes:
		return includes(value, condition.Literal)
	default:
		return false, fmt.Errorf("unknown condition operator %q", condition.Operator)
	}
}

type conditionParser struct {
	source   string
	position int
}

func (parser *conditionParser) done() bool {
	return parser.position == len(parser.source)
}

func (parser *conditionParser) peek(prefix string) bool {
	return strings.HasPrefix(parser.source[parser.position:], prefix)
}

func (parser *conditionParser) skipSpace() {
	for parser.position < len(parser.source) {
		switch parser.source[parser.position] {
		case ' ', '\t', '\r', '\n':
			parser.position++
		default:
			return
		}
	}
}

func (parser *conditionParser) identifier() (string, error) {
	start := parser.position
	if start == len(parser.source) || !isIdentifierStart(parser.source[start]) {
		return "", fmt.Errorf("expected identifier")
	}
	parser.position++
	for parser.position < len(parser.source) && isIdentifierPart(parser.source[parser.position]) {
		parser.position++
	}
	return parser.source[start:parser.position], nil
}

func (parser *conditionParser) operator() (ConditionOperator, error) {
	switch {
	case parser.peek(string(ConditionEqual)):
		parser.position += len(ConditionEqual)
		return ConditionEqual, nil
	case parser.peek(string(ConditionUnequal)):
		parser.position += len(ConditionUnequal)
		return ConditionUnequal, nil
	case parser.peek(string(ConditionIncludes)):
		end := parser.position + len(ConditionIncludes)
		if end < len(parser.source) && isIdentifierPart(parser.source[end]) {
			return "", fmt.Errorf("expected ==, !=, or includes")
		}
		parser.position = end
		return ConditionIncludes, nil
	default:
		return "", fmt.Errorf("expected ==, !=, or includes")
	}
}

func parseConditionLiteral(text string) (any, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(text))
	decoder.UseNumber()
	var literal any
	if err := decoder.Decode(&literal); err != nil {
		return nil, fmt.Errorf("invalid literal: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("unexpected data after literal: %w", err)
		}
		return nil, fmt.Errorf("unexpected data after literal")
	}

	switch literal.(type) {
	case string, json.Number, bool:
		return literal, nil
	default:
		return nil, fmt.Errorf("literal must be a quoted string, number, true, or false")
	}
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	case json.Number:
		number, err := typed.Float64()
		return err == nil && number != 0
	case float64:
		return typed != 0
	case float32:
		return typed != 0
	case int:
		return typed != 0
	case int8:
		return typed != 0
	case int16:
		return typed != 0
	case int32:
		return typed != 0
	case int64:
		return typed != 0
	case uint:
		return typed != 0
	case uint8:
		return typed != 0
	case uint16:
		return typed != 0
	case uint32:
		return typed != 0
	case uint64:
		return typed != 0
	case []string:
		return len(typed) != 0
	case []any:
		return len(typed) != 0
	default:
		return true
	}
}

func valuesEqual(left any, right any) bool {
	leftNumber, leftNumeric := numericValue(left)
	rightNumber, rightNumeric := numericValue(right)
	if leftNumeric || rightNumeric {
		return leftNumeric && rightNumeric && leftNumber == rightNumber
	}

	switch leftTyped := left.(type) {
	case string:
		rightTyped, ok := right.(string)
		return ok && leftTyped == rightTyped
	case bool:
		rightTyped, ok := right.(bool)
		return ok && leftTyped == rightTyped
	case nil:
		return right == nil
	default:
		return false
	}
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func includes(collection any, wanted any) (bool, error) {
	switch typed := collection.(type) {
	case []string:
		for _, value := range typed {
			if valuesEqual(value, wanted) {
				return true, nil
			}
		}
		return false, nil
	case []any:
		for _, value := range typed {
			if valuesEqual(value, wanted) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("%q requires a multiselect value, got %T", ConditionIncludes, collection)
	}
}

func isIdentifierStart(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value == '_'
}

func isIdentifierPart(value byte) bool {
	return isIdentifierStart(value) || value >= '0' && value <= '9' || value == '-'
}
