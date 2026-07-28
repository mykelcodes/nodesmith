package recipe

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expression string
		identifier string
		operator   ConditionOperator
		literal    any
		negated    bool
	}{
		{expression: "enabled", identifier: "enabled"},
		{expression: " ! enabled ", identifier: "enabled", negated: true},
		{expression: `template == "react ts"`, identifier: "template", operator: ConditionEqual, literal: "react ts"},
		{expression: `template!="library"`, identifier: "template", operator: ConditionUnequal, literal: "library"},
		{expression: `addons includes "tailwind"`, identifier: "addons", operator: ConditionIncludes, literal: "tailwind"},
		{expression: `count == -1.25e2`, identifier: "count", operator: ConditionEqual, literal: json.Number("-1.25e2")},
		{expression: `enabled == false`, identifier: "enabled", operator: ConditionEqual, literal: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.expression, func(t *testing.T) {
			t.Parallel()
			got, err := ParseCondition(test.expression)
			if err != nil {
				t.Fatalf("ParseCondition() error = %v", err)
			}
			if got.Identifier != test.identifier || got.Operator != test.operator || got.Negated != test.negated ||
				!valuesEqual(got.Literal, test.literal) {
				t.Fatalf("ParseCondition() = %#v, want id=%q op=%q literal=%#v negated=%t",
					got, test.identifier, test.operator, test.literal, test.negated)
			}
		})
	}
}

func TestParseConditionRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"!",
		"1field",
		"enabled nope true",
		"!enabled == true",
		"enabled ==",
		`enabled == "unterminated`,
		"enabled == null",
		"enabled == []",
		"enabled == true false",
		"addons includesThing true",
	}
	for _, expression := range tests {
		expression := expression
		t.Run(expression, func(t *testing.T) {
			t.Parallel()
			if condition, err := ParseCondition(expression); err == nil {
				t.Fatalf("ParseCondition(%q) = %#v, want error", expression, condition)
			}
		})
	}
}

func TestConditionEvaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition Condition
		values    map[string]any
		want      bool
		wantError string
	}{
		{name: "truthy bool", condition: Condition{Identifier: "v"}, values: map[string]any{"v": true}, want: true},
		{name: "negated empty", condition: Condition{Identifier: "v", Negated: true}, values: map[string]any{"v": ""}, want: true},
		{name: "number equality", condition: Condition{Identifier: "v", Operator: ConditionEqual, Literal: json.Number("2")}, values: map[string]any{"v": 2.0}, want: true},
		{name: "unequal", condition: Condition{Identifier: "v", Operator: ConditionUnequal, Literal: "a"}, values: map[string]any{"v": "b"}, want: true},
		{name: "includes strings", condition: Condition{Identifier: "v", Operator: ConditionIncludes, Literal: "b"}, values: map[string]any{"v": []string{"a", "b"}}, want: true},
		{name: "includes any", condition: Condition{Identifier: "v", Operator: ConditionIncludes, Literal: "b"}, values: map[string]any{"v": []any{"a", "b"}}, want: true},
		{name: "does not include", condition: Condition{Identifier: "v", Operator: ConditionIncludes, Literal: "x"}, values: map[string]any{"v": []string{"a"}}, want: false},
		{name: "unknown", condition: Condition{Identifier: "missing"}, values: map[string]any{}, wantError: "unknown identifier"},
		{name: "includes wrong type", condition: Condition{Identifier: "v", Operator: ConditionIncludes, Literal: "x"}, values: map[string]any{"v": true}, wantError: "requires a multiselect"},
		{name: "unknown operator", condition: Condition{Identifier: "v", Operator: "??"}, values: map[string]any{"v": true}, wantError: "unknown condition operator"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := test.condition.Evaluate(test.values)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("Evaluate() error = %v, want substring %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Evaluate() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestVariables(t *testing.T) {
	t.Parallel()

	got, err := Variables(`prefix-${projectName}-${item}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "projectName,item" {
		t.Fatalf("Variables() = %v", got)
	}
	for _, invalid := range []string{`${`, `${}`, `${not valid}`} {
		if _, err := Variables(invalid); err == nil {
			t.Errorf("Variables(%q) unexpectedly succeeded", invalid)
		}
	}
}

func TestTruthinessAndNumericKinds(t *testing.T) {
	t.Parallel()

	truthyCases := []struct {
		value any
		want  bool
	}{
		{value: nil, want: false},
		{value: false, want: false},
		{value: true, want: true},
		{value: "", want: false},
		{value: "x", want: true},
		{value: json.Number("0"), want: false},
		{value: json.Number("bad"), want: false},
		{value: float64(1), want: true},
		{value: float32(1), want: true},
		{value: int(1), want: true},
		{value: int8(1), want: true},
		{value: int16(1), want: true},
		{value: int32(1), want: true},
		{value: int64(1), want: true},
		{value: uint(1), want: true},
		{value: uint8(1), want: true},
		{value: uint16(1), want: true},
		{value: uint32(1), want: true},
		{value: uint64(1), want: true},
		{value: []string{}, want: false},
		{value: []string{"x"}, want: true},
		{value: []any{}, want: false},
		{value: []any{"x"}, want: true},
		{value: struct{}{}, want: true},
	}
	for _, test := range truthyCases {
		if got := truthy(test.value); got != test.want {
			t.Errorf("truthy(%#v) = %t, want %t", test.value, got, test.want)
		}
	}

	numericCases := []any{
		json.Number("1"),
		float64(1),
		float32(1),
		int(1),
		int8(1),
		int16(1),
		int32(1),
		int64(1),
		uint(1),
		uint8(1),
		uint16(1),
		uint32(1),
		uint64(1),
	}
	for _, value := range numericCases {
		if got, ok := numericValue(value); !ok || got != 1 {
			t.Errorf("numericValue(%T) = %v,%t, want 1,true", value, got, ok)
		}
	}
	if _, ok := numericValue("1"); ok {
		t.Error("numericValue(string) unexpectedly succeeded")
	}
	if valuesEqual(true, false) || valuesEqual("x", true) || !valuesEqual(nil, nil) || valuesEqual(nil, false) {
		t.Error("valuesEqual returned an incorrect cross-type result")
	}
}
