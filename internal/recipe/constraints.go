package recipe

import (
	"fmt"
	"regexp"
	"sync"
	"unicode/utf8"
)

// patternCache memoises compiled field patterns.
//
// Validate compiles every pattern when a manifest loads, which both rejects a
// bad expression before it can reach a plan and leaves the cache warm. Answer
// validation then costs a map lookup rather than a compile.
var patternCache sync.Map // map[string]*regexp.Regexp

// CompilePattern returns the compiled form of a field pattern, compiling and
// caching it on first use. Go's regexp is RE2, so a pattern cannot backtrack
// catastrophically regardless of what a recipe author writes.
func CompilePattern(pattern string) (*regexp.Regexp, error) {
	if cached, ok := patternCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile pattern %q: %w", pattern, err)
	}
	actual, _ := patternCache.LoadOrStore(pattern, compiled)
	return actual.(*regexp.Regexp), nil
}

// validateFieldConstraints checks that a field's constraints are internally
// coherent and apply to its type. It runs at manifest-load time.
func validateFieldConstraints(path string, field Field) error {
	textOnly := field.Pattern != "" || field.MinLength != nil || field.MaxLength != nil
	if textOnly && field.Type != FieldText {
		return fmt.Errorf(
			"%s: pattern, minLength, and maxLength apply only to text fields, not %q",
			path,
			field.Type,
		)
	}
	if (field.Min != nil || field.Max != nil) && field.Type != FieldNumber {
		return fmt.Errorf("%s: min and max apply only to number fields, not %q", path, field.Type)
	}

	if field.Pattern != "" {
		if _, err := CompilePattern(field.Pattern); err != nil {
			return fmt.Errorf("%s.pattern: %w", path, err)
		}
	}
	if field.MinLength != nil && *field.MinLength < 0 {
		return fmt.Errorf("%s.minLength: must not be negative", path)
	}
	if field.MaxLength != nil && *field.MaxLength < 0 {
		return fmt.Errorf("%s.maxLength: must not be negative", path)
	}
	if field.MinLength != nil && field.MaxLength != nil && *field.MinLength > *field.MaxLength {
		return fmt.Errorf(
			"%s: minLength %d exceeds maxLength %d",
			path,
			*field.MinLength,
			*field.MaxLength,
		)
	}
	if field.Min != nil && field.Max != nil && *field.Min > *field.Max {
		return fmt.Errorf("%s: min %v exceeds max %v", path, *field.Min, *field.Max)
	}

	// A default that violates its own field's constraints would be substituted
	// into argv whenever the field is left unanswered, which defeats the point
	// of declaring the constraint.
	if err := CheckFieldConstraints(field, field.Default); err != nil {
		return fmt.Errorf("%s.default: %w", path, err)
	}
	return nil
}

// CheckFieldConstraints reports whether value satisfies the field's declared
// value constraints. Type checking is the caller's responsibility; this only
// applies the optional bounds, so a value of an unexpected type is ignored here
// rather than reported twice.
func CheckFieldConstraints(field Field, value any) error {
	switch field.Type {
	case FieldText:
		text, ok := value.(string)
		if !ok {
			return nil
		}
		// Runes, not bytes: a limit expressed in bytes would reject fewer
		// characters than the author wrote for any non-ASCII answer.
		length := utf8.RuneCountInString(text)
		if field.MinLength != nil && length < *field.MinLength {
			return fmt.Errorf(
				"must be at least %d characters, got %d",
				*field.MinLength,
				length,
			)
		}
		if field.MaxLength != nil && length > *field.MaxLength {
			return fmt.Errorf(
				"must be at most %d characters, got %d",
				*field.MaxLength,
				length,
			)
		}
		if field.Pattern != "" {
			expression, err := CompilePattern(field.Pattern)
			if err != nil {
				return err
			}
			if !expression.MatchString(text) {
				return fmt.Errorf("%q does not match the required pattern %s", text, field.Pattern)
			}
		}
	case FieldNumber:
		number, ok := numericValue(value)
		if !ok {
			return nil
		}
		if field.Min != nil && number < *field.Min {
			return fmt.Errorf("must be at least %v, got %v", *field.Min, number)
		}
		if field.Max != nil && number > *field.Max {
			return fmt.Errorf("must be at most %v, got %v", *field.Max, number)
		}
	}
	return nil
}
