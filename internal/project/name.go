package project

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const defaultMaxNameBytes = 255

var (
	ErrInvalidName    = errors.New("invalid project name")
	ErrInvalidParent  = errors.New("invalid parent directory")
	ErrPathTraversal  = errors.New("project path escapes parent directory")
	ErrTargetExists   = errors.New("project target already exists")
	ErrTargetNotEmpty = errors.New("project target directory is not empty")
	ErrNotWritable    = errors.New("project target is not writable")
)

// NameOptions controls the small set of project-name validation exceptions.
type NameOptions struct {
	AllowLeadingDot bool
	// MaxBytes defaults to 255, the conservative cross-platform component
	// limit. It can be lowered by a caller but cannot disable the limit.
	MaxBytes int
}

// ValidateProjectName applies the default cross-platform project-name rules.
func ValidateProjectName(name string, allowLeadingDot bool) error {
	return ValidateName(name, NameOptions{AllowLeadingDot: allowLeadingDot})
}

// ValidateName rejects unsafe or non-portable single-component names. Windows
// rules are applied on all platforms so a project created on macOS or Linux can
// still be checked out on Windows.
func ValidateName(name string, options NameOptions) error {
	if !utf8.ValidString(name) {
		return fmt.Errorf("%w: name is not valid UTF-8", ErrInvalidName)
	}
	if name == "" {
		return fmt.Errorf("%w: name is empty", ErrInvalidName)
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("%w: leading or trailing whitespace is not allowed", ErrInvalidName)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("%w: %q is a traversal component", ErrInvalidName, name)
	}
	if !options.AllowLeadingDot && strings.HasPrefix(name, ".") {
		return fmt.Errorf("%w: leading dots are not allowed", ErrInvalidName)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%w: path separators are not allowed", ErrInvalidName)
	}
	if strings.ContainsAny(name, `<>:"|?*`) {
		return fmt.Errorf("%w: contains a character that is invalid on Windows", ErrInvalidName)
	}
	for _, character := range name {
		if character == 0 || character < 0x20 || character == 0x7f {
			return fmt.Errorf("%w: control characters are not allowed", ErrInvalidName)
		}
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return fmt.Errorf("%w: trailing dots or spaces are not allowed", ErrInvalidName)
	}

	maxBytes := options.MaxBytes
	if maxBytes <= 0 || maxBytes > defaultMaxNameBytes {
		maxBytes = defaultMaxNameBytes
	}
	if len(name) > maxBytes {
		return fmt.Errorf(
			"%w: name is %d bytes; maximum is %d",
			ErrInvalidName,
			len(name),
			maxBytes,
		)
	}
	if isWindowsReservedName(name) {
		return fmt.Errorf("%w: %q is a reserved Windows device name", ErrInvalidName, name)
	}
	return nil
}

func isWindowsReservedName(name string) bool {
	base, _, _ := strings.Cut(name, ".")
	base = strings.ToUpper(strings.TrimRight(base, " ."))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$", "CONIN$", "CONOUT$":
		return true
	}
	if len(base) == 4 {
		prefix := base[:3]
		number := base[3]
		if (prefix == "COM" || prefix == "LPT") && number >= '1' && number <= '9' {
			return true
		}
	}
	for _, reserved := range []string{
		"COM¹",
		"COM²",
		"COM³",
		"LPT¹",
		"LPT²",
		"LPT³",
	} {
		if base == reserved {
			return true
		}
	}
	return false
}
