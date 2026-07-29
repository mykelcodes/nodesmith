package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nodesmith/internal/planner"
)

func writeProjectConfig(step planner.PlanStep) error {
	if step.Config == nil {
		return errors.New("project configuration details are missing")
	}
	config := *step.Config
	if config.Path == "" || filepath.Base(config.Path) != config.Path ||
		config.Path == "." || config.Path == ".." {
		return fmt.Errorf("project configuration path %q is not a file name", config.Path)
	}
	if strings.TrimSpace(config.Key) == "" {
		return errors.New("project configuration key is empty")
	}

	info, err := os.Stat(step.Dir)
	if err != nil {
		return fmt.Errorf("open generated project: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("generated project path %q is not a directory", step.Dir)
	}

	path := filepath.Join(step.Dir, config.Path)
	mode := os.FileMode(0o644)
	fileInfo, err := os.Lstat(path)
	switch {
	case err == nil:
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse to update symlinked project configuration %s", config.Path)
		}
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("project configuration %s is not a regular file", config.Path)
		}
		mode = fileInfo.Mode().Perm()
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("inspect %s: %w", config.Path, err)
	}

	var content []byte
	if fileInfo != nil {
		content, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", config.Path, err)
		}
	}

	rendered, err := renderProjectConfig(string(content), config)
	if err != nil {
		return fmt.Errorf("update %s: %w", config.Path, err)
	}
	if rendered == string(content) {
		return nil
	}
	if err := os.WriteFile(path, []byte(rendered), mode); err != nil {
		return fmt.Errorf("write %s: %w", config.Path, err)
	}
	return nil
}

func renderProjectConfig(content string, config planner.ProjectConfig) (string, error) {
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		newline = "\r\n"
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := splitConfigLines(normalized)

	var rendered []string
	switch config.Format {
	case planner.ConfigFormatProperties:
		rendered = upsertTopLevel(lines, config.Key, config.Key+"="+config.Value, "=")
	case planner.ConfigFormatYAML:
		rendered = upsertTopLevel(lines, config.Key, config.Key+": "+config.Value, ":")
	case planner.ConfigFormatTOML:
		if strings.TrimSpace(config.Section) == "" {
			return "", errors.New("TOML configuration section is empty")
		}
		rendered = upsertTOML(lines, config.Section, config.Key, config.Key+" = "+config.Value)
	default:
		return "", fmt.Errorf("configuration format %q is not supported", config.Format)
	}
	return strings.Join(rendered, newline) + newline, nil
}

func splitConfigLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return []string{}
	}
	return strings.Split(content, "\n")
}

func upsertTopLevel(lines []string, key string, replacement string, separator string) []string {
	for index, line := range lines {
		if len(line) != len(strings.TrimLeft(line, " \t")) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		before, _, found := strings.Cut(trimmed, separator)
		if found && strings.TrimSpace(before) == key {
			lines[index] = replacement
			return lines
		}
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	return append(lines, replacement)
}

func upsertTOML(lines []string, section string, key string, replacement string) []string {
	header := "[" + section + "]"
	sectionStart := -1
	sectionEnd := len(lines)
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == header {
			sectionStart = index
			continue
		}
		if sectionStart >= 0 && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sectionEnd = index
			break
		}
	}
	if sectionStart < 0 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		return append(lines, header, replacement)
	}

	for index := sectionStart + 1; index < sectionEnd; index++ {
		trimmed := strings.TrimSpace(lines[index])
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		before, _, found := strings.Cut(trimmed, "=")
		if found && strings.TrimSpace(before) == key {
			lines[index] = replacement
			return lines
		}
	}

	insertAt := sectionEnd
	for insertAt > sectionStart+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}
	lines = append(lines, "")
	copy(lines[insertAt+1:], lines[insertAt:])
	lines[insertAt] = replacement
	return lines
}
