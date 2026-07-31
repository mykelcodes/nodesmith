package toolchain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type pathProvider interface {
	ResolvedPath(context.Context) (string, error)
}

// Resolver locates only allowlisted binaries on the effective PATH.
type Resolver struct {
	paths pathProvider
	goos  string
	stat  func(string) (os.FileInfo, error)
}

var windowsNodeCLIScripts = map[string][][]string{
	"npm": {
		{"node_modules", "npm", "bin", "npm-cli.js"},
	},
	"npx": {
		{"node_modules", "npm", "bin", "npx-cli.js"},
	},
	"pnpm": {
		{"node_modules", "corepack", "dist", "pnpm.js"},
		{"node_modules", "pnpm", "bin", "pnpm.cjs"},
	},
	"pnpx": {
		{"node_modules", "pnpm", "bin", "pnpx.cjs"},
	},
	"yarn": {
		{"node_modules", "corepack", "dist", "yarn.js"},
		{"node_modules", "yarn", "bin", "yarn.js"},
	},
}

// NewResolver creates a binary resolver backed by paths. A nil paths value
// uses a fresh login-shell-aware PathResolver.
func NewResolver(paths *PathResolver) *Resolver {
	if paths == nil {
		paths = NewPathResolver()
	}
	return &Resolver{
		paths: paths,
		goos:  runtime.GOOS,
		stat:  os.Stat,
	}
}

// Resolve locates an allowlisted logical binary name.
func (r *Resolver) Resolve(name string) (string, error) {
	return r.ResolveContext(context.Background(), name)
}

// ResolveCommand returns a native executable plus any fixed argv prefix needed
// to invoke a logical tool. On Windows, Node package-manager batch shims are
// translated to node.exe plus their JavaScript entrypoint so the runner never
// invokes cmd.exe or PowerShell.
func (r *Resolver) ResolveCommand(name string) (string, []string, error) {
	return r.ResolveCommandContext(context.Background(), name)
}

// ResolveCommandContext is ResolveCommand with caller cancellation for PATH
// discovery.
func (r *Resolver) ResolveCommandContext(
	ctx context.Context,
	name string,
) (string, []string, error) {
	path, err := r.ResolveContext(ctx, name)
	if err != nil {
		return "", nil, err
	}
	return r.nativeCommand(ctx, name, path)
}

// ResolveToolContext resolves name with a single PATH walk and reports both the
// path the tool was discovered at and the native command used to invoke it.
// Callers that need both — the toolchain scan needs the discovered path for
// display and the native command for the version probe — would otherwise walk
// the PATH twice per tool.
//
// When resolution succeeds but no native command can be derived, the discovered
// path is still returned alongside the error so callers can report the tool as
// present but unusable.
func (r *Resolver) ResolveToolContext(
	ctx context.Context,
	name string,
) (string, string, []string, error) {
	path, err := r.ResolveContext(ctx, name)
	if err != nil {
		return "", "", nil, err
	}
	command, prefixArgs, err := r.nativeCommand(ctx, name, path)
	if err != nil {
		return path, "", nil, err
	}
	return path, command, prefixArgs, nil
}

func (r *Resolver) nativeCommand(
	ctx context.Context,
	name string,
	path string,
) (string, []string, error) {
	if r.goos != "windows" {
		return path, nil, nil
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".com":
		return path, nil, nil
	}
	candidates, supported := windowsNodeCLIScripts[name]
	if !supported {
		return "", nil, fmt.Errorf(
			"%w: %s resolved to non-native Windows shim %q",
			ErrBinaryNotFound,
			name,
			path,
		)
	}
	directory := filepath.Dir(path)
	nodePath, err := r.resolveWindowsShimNode(ctx, name, directory)
	if err != nil {
		return "", nil, err
	}
	for _, parts := range candidates {
		script := filepath.Join(append([]string{directory}, parts...)...)
		info, statErr := r.stat(script)
		if statErr == nil && !info.IsDir() {
			absolute, absErr := filepath.Abs(script)
			if absErr != nil {
				return "", nil, fmt.Errorf("make %s entrypoint absolute: %w", name, absErr)
			}
			return nodePath, []string{absolute}, nil
		}
	}
	return "", nil, fmt.Errorf(
		"%w: JavaScript entrypoint for Windows %s shim %q",
		ErrBinaryNotFound,
		name,
		path,
	)
}

func (r *Resolver) resolveWindowsShimNode(
	ctx context.Context,
	name string,
	shimDirectory string,
) (string, error) {
	for _, filename := range []string{"node.exe", "node.com"} {
		candidate := filepath.Join(shimDirectory, filename)
		info, err := r.stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			return "", fmt.Errorf("make adjacent node executable absolute: %w", err)
		}
		return absolute, nil
	}

	nodePath, err := r.ResolveContext(ctx, "node")
	if err != nil {
		return "", fmt.Errorf("resolve node.exe for %s shim: %w", name, err)
	}
	nodeExtension := strings.ToLower(filepath.Ext(nodePath))
	if nodeExtension != ".exe" && nodeExtension != ".com" {
		return "", fmt.Errorf(
			"resolve node.exe for %s shim: non-native executable %q",
			name,
			nodePath,
		)
	}
	return nodePath, nil
}

// ResolveContext is Resolve with caller cancellation for PATH discovery.
func (r *Resolver) ResolveContext(ctx context.Context, name string) (string, error) {
	if err := validateBinaryName(name); err != nil {
		return "", err
	}
	pathValue, err := r.paths.ResolvedPath(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve PATH for %s: %w", name, err)
	}
	return resolveInPath(name, pathValue, r.goos, r.stat)
}

// ResolvedPath exposes the effective PATH backing this resolver.
func (r *Resolver) ResolvedPath(ctx context.Context) (string, error) {
	return r.paths.ResolvedPath(ctx)
}

func resolveInPath(
	name string,
	pathValue string,
	goos string,
	stat func(string) (os.FileInfo, error),
) (string, error) {
	candidates := []string{name}
	if goos == "windows" {
		candidates = windowsCandidates(name)
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
			return absolute, nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrBinaryNotFound, name)
}

func windowsCandidates(name string) []string {
	if filepath.Ext(name) != "" {
		return []string{name}
	}
	// Prefer native executables, then the command shims installed by Node
	// package managers. PowerShell shims are included for discovery parity but
	// are never invoked through a shell by this package.
	extensions := []string{".exe", ".com", ".cmd", ".bat", ".ps1"}
	candidates := make([]string, 0, len(extensions)+1)
	for _, extension := range extensions {
		candidates = append(candidates, name+extension)
	}
	// Extensionless files in Node installations are POSIX shell shims. Keep
	// them as a last-resort discovery result so they cannot shadow npm.cmd.
	candidates = append(candidates, name)
	return candidates
}

func replacePATH(environment []string, pathValue string, goos string) []string {
	result := make([]string, 0, len(environment)+1)
	replaced := false
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		isPath := key == "PATH"
		if goos == "windows" {
			isPath = strings.EqualFold(key, "PATH")
		}
		if isPath {
			if !replaced {
				result = append(result, "PATH="+pathValue)
				replaced = true
			}
			continue
		}
		result = append(result, entry)
	}
	if !replaced {
		result = append(result, "PATH="+pathValue)
	}
	return result
}

var defaultResolver = NewResolver(defaultPathResolver)

// Resolve locates an allowlisted binary using the package-level PATH resolver.
func Resolve(name string) (string, error) {
	return defaultResolver.Resolve(name)
}
