package project

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"nodesmith/internal/environ"
	"nodesmith/internal/toolchain"
)

var (
	ErrUnsupportedEditor      = errors.New("unsupported editor")
	ErrIntegrationUnavailable = errors.New("desktop integration is unavailable")
)

var supportedEditors = []string{"code", "cursor", "zed"}

type integrationCommand struct {
	name string
	args []string
}

type integrationLauncher struct {
	goos         string
	resolvedPath func() (string, error)
	start        func(string, []string, string) error
	stat         func(string) (os.FileInfo, error)
}

var defaultIntegrationLauncher = integrationLauncher{
	goos:         runtime.GOOS,
	resolvedPath: toolchain.ResolvedPath,
	start:        startDetached,
	stat:         os.Stat,
}

// SupportedEditors returns the stable editor identifier allowlist.
func SupportedEditors() []string {
	return append([]string(nil), supportedEditors...)
}

// ValidateEditor accepts a known editor identifier or an absolute custom
// executable path. Custom shell command text and relative paths are rejected.
func ValidateEditor(editor string) error {
	for _, supported := range supportedEditors {
		if editor == supported {
			return nil
		}
	}
	if strings.IndexByte(editor, 0) >= 0 || !filepath.IsAbs(editor) {
		return fmt.Errorf("%w: %q", ErrUnsupportedEditor, editor)
	}
	return nil
}

// OpenInEditor launches a generated project in VS Code, Cursor, Zed, or an
// explicitly configured executable. The project directory is always one argv
// element and no shell command is interpreted.
func OpenInEditor(directory string, editor string) error {
	canonical, err := canonicalDirectory(directory)
	if err != nil {
		return err
	}
	command, err := editorCommand(canonical, editor)
	if err != nil {
		return err
	}
	if err := defaultIntegrationLauncher.launch(command); err != nil {
		return fmt.Errorf("open %q in %s: %w", canonical, editor, err)
	}
	return nil
}

// RevealInFileManager opens the generated project in the platform file
// manager.
func RevealInFileManager(directory string) error {
	canonical, err := canonicalDirectory(directory)
	if err != nil {
		return err
	}
	command, err := platformRevealCommand(canonical)
	if err != nil {
		return err
	}
	if err := defaultIntegrationLauncher.launch(command); err != nil {
		return fmt.Errorf("reveal %q in file manager: %w", canonical, err)
	}
	return nil
}

func editorCommand(directory string, editor string) (integrationCommand, error) {
	if err := ValidateEditor(editor); err != nil {
		return integrationCommand{}, err
	}
	return integrationCommand{
		name: editor,
		args: []string{directory},
	}, nil
}

func revealCommandFor(goos string, directory string) (integrationCommand, error) {
	switch goos {
	case "darwin":
		return integrationCommand{name: "open", args: []string{"-R", directory}}, nil
	case "linux":
		return integrationCommand{name: "xdg-open", args: []string{directory}}, nil
	case "windows":
		return integrationCommand{name: "explorer.exe", args: []string{directory}}, nil
	default:
		return integrationCommand{}, fmt.Errorf(
			"%w on operating system %q",
			ErrIntegrationUnavailable,
			goos,
		)
	}
}

func canonicalDirectory(directory string) (string, error) {
	if directory == "" {
		return "", fmt.Errorf("%w: integration directory is empty", ErrInvalidParent)
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("%w: make integration directory absolute: %w", ErrInvalidParent, err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("%w: resolve integration directory %q: %w", ErrInvalidParent, absolute, err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: inspect integration directory %q: %w", ErrInvalidParent, canonical, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: integration path %q is not a directory", ErrInvalidParent, canonical)
	}
	return canonical, nil
}

func (launcher integrationLauncher) launch(command integrationCommand) error {
	pathValue, err := launcher.resolvedPath()
	if err != nil {
		return fmt.Errorf("resolve PATH for %s: %w", command.name, err)
	}
	executable := command.name
	if filepath.IsAbs(command.name) {
		info, statErr := launcher.stat(command.name)
		if statErr != nil {
			return fmt.Errorf(
				"%w: inspect custom editor executable %q: %v",
				ErrIntegrationUnavailable,
				command.name,
				statErr,
			)
		}
		if info.IsDir() {
			return fmt.Errorf(
				"%w: custom editor executable %q is a directory",
				ErrIntegrationUnavailable,
				command.name,
			)
		}
		extension := strings.ToLower(filepath.Ext(command.name))
		if launcher.goos == "windows" && extension != ".exe" && extension != ".com" {
			return fmt.Errorf(
				"%w: custom Windows editor must be a native .exe or .com executable",
				ErrIntegrationUnavailable,
			)
		}
		if launcher.goos != "windows" && info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf(
				"%w: custom editor %q is not executable",
				ErrIntegrationUnavailable,
				command.name,
			)
		}
	} else {
		executable, err = findIntegrationExecutable(
			command.name,
			pathValue,
			launcher.goos,
			launcher.stat,
		)
		if err != nil {
			return err
		}
	}
	if err := launcher.start(executable, append([]string(nil), command.args...), pathValue); err != nil {
		return fmt.Errorf("start %s: %w", command.name, err)
	}
	return nil
}

func findIntegrationExecutable(
	name string,
	pathValue string,
	goos string,
	stat func(string) (os.FileInfo, error),
) (string, error) {
	if strings.ContainsAny(name, `/\`) || name == "" {
		return "", fmt.Errorf("%w: invalid integration executable %q", ErrIntegrationUnavailable, name)
	}
	candidates := []string{name}
	if goos == "windows" && filepath.Ext(name) == "" {
		candidates = []string{
			name + ".exe",
			name + ".com",
			name + ".cmd",
			name + ".bat",
			name + ".ps1",
			name,
		}
	}
	for _, directory := range filepath.SplitList(pathValue) {
		if directory == "" {
			directory = "."
		}
		for _, candidate := range candidates {
			path := filepath.Join(directory, candidate)
			info, err := stat(path)
			if err != nil || info.IsDir() {
				continue
			}
			if goos != "windows" && info.Mode().Perm()&0o111 == 0 {
				continue
			}
			absolute, err := filepath.Abs(path)
			if err != nil {
				return "", fmt.Errorf("make %s path absolute: %w", name, err)
			}
			if goos == "windows" {
				if native, ok := nativeWindowsIntegration(name, absolute, stat); ok {
					return native, nil
				}
				continue
			}
			return absolute, nil
		}
	}
	return "", fmt.Errorf(
		"%w: %s was not found on the resolved PATH",
		ErrIntegrationUnavailable,
		name,
	)
}

func nativeWindowsIntegration(
	name string,
	resolved string,
	stat func(string) (os.FileInfo, error),
) (string, bool) {
	extension := strings.ToLower(filepath.Ext(resolved))
	if extension == ".exe" || extension == ".com" {
		return resolved, true
	}

	executableNames := map[string][]string{
		"code":   {"Code.exe", "code.exe"},
		"cursor": {"Cursor.exe", "cursor.exe"},
		"zed":    {"Zed.exe", "zed.exe"},
	}[name]
	if len(executableNames) == 0 {
		return "", false
	}
	directory := filepath.Dir(resolved)
	for range 5 {
		for _, executableName := range executableNames {
			candidate := filepath.Join(directory, executableName)
			info, err := stat(candidate)
			if err == nil && !info.IsDir() {
				absolute, absErr := filepath.Abs(candidate)
				return absolute, absErr == nil
			}
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	return "", false
}

// startDetached launches an editor and deliberately abandons it. No WaitDelay is
// set here, unlike every other exec site in the tree: WaitDelay only takes effect
// inside Cmd.Wait, and this process is released rather than waited on. The editor
// is meant to outlive Nodesmith.
func startDetached(executable string, args []string, pathValue string) error {
	cmd := exec.Command(executable, args...)
	cmd.Env = environ.WithPATH(os.Environ(), pathValue, runtime.GOOS)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release detached process: %w", err)
	}
	return nil
}
