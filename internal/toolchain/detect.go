package toolchain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

const (
	defaultDetectionTTL = 60 * time.Second
	// versionProbeTimeout has to tolerate genuinely slow first runs: corepack
	// downloads the package manager on first invocation, and Windows Defender
	// adds seconds to a cold process launch. A tight budget here silently
	// disables recipes, so it is deliberately generous.
	versionProbeTimeout = 8 * time.Second
	// probeWaitDelay bounds how long a killed probe may leave its output pipe
	// held open by a process it spawned.
	probeWaitDelay = 1 * time.Second
)

// Tool describes one detected executable.
type Tool struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Present bool   `json:"present"`
	Error   string `json:"error,omitempty"`
}

// Toolchain is a point-in-time scan of Nodesmith's supported tools.
type Toolchain struct {
	Path       string          `json:"path"`
	DetectedAt time.Time       `json:"detectedAt"`
	Tools      map[string]Tool `json:"tools"`
}

// Lookup returns a tool by its logical name.
func (toolchain Toolchain) Lookup(name string) (Tool, bool) {
	tool, ok := toolchain.Tools[name]
	return tool, ok
}

// BinaryResolver is the narrow resolver contract used by Detector.
type BinaryResolver interface {
	Resolve(name string) (string, error)
}

type pathReporter interface {
	ResolvedPath(context.Context) (string, error)
}

type versionProbeFunc func(context.Context, string, string, string, []string) (string, error)

type commandResolver interface {
	ResolveCommand(name string) (string, []string, error)
}

// toolResolver resolves a logical name once and reports both the discovered
// path and the native command used to invoke it.
type toolResolver interface {
	ResolveToolContext(ctx context.Context, name string) (string, string, []string, error)
}

// Detector scans and version-probes the supported toolchain. Successful scans
// are cached for 60 seconds unless force is true or the effective PATH changes.
type Detector struct {
	resolver BinaryResolver

	mu       sync.Mutex
	cached   Toolchain
	hasCache bool

	ttl            time.Duration
	commandTimeout time.Duration
	now            func() time.Time
	probe          versionProbeFunc
}

// NewDetector creates a detector. Passing nil uses the package-level resolver.
func NewDetector(resolver BinaryResolver) *Detector {
	if resolver == nil {
		resolver = defaultResolver
	}
	return &Detector{
		resolver:       resolver,
		ttl:            defaultDetectionTTL,
		commandTimeout: versionProbeTimeout,
		now:            time.Now,
		probe:          probeVersion,
	}
}

// Detect returns the current toolchain, using the warm cache when permitted.
// Missing binaries are represented as Present=false and are not returned as an
// operation-level error.
func (detector *Detector) Detect(ctx context.Context, force bool) (Toolchain, error) {
	pathValue := ""
	if reporter, ok := detector.resolver.(pathReporter); ok {
		var err error
		pathValue, err = reporter.ResolvedPath(ctx)
		if err != nil {
			return Toolchain{}, fmt.Errorf("resolve toolchain PATH: %w", err)
		}
	}

	now := detector.now()
	detector.mu.Lock()
	if !force &&
		detector.hasCache &&
		now.Sub(detector.cached.DetectedAt) < detector.ttl &&
		detector.cached.Path == pathValue {
		cached := cloneToolchain(detector.cached)
		detector.mu.Unlock()
		return cached, nil
	}
	detector.mu.Unlock()

	type detection struct {
		name string
		tool Tool
	}
	results := make(chan detection, len(detectedBinaries))
	var wait sync.WaitGroup
	for _, name := range detectedBinaries {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- detection{name: name, tool: detector.detectOne(ctx, name, pathValue)}
		}()
	}
	wait.Wait()
	close(results)

	tools := make(map[string]Tool, len(detectedBinaries))
	for result := range results {
		tools[result.name] = result.tool
	}
	toolchain := Toolchain{
		Path:       pathValue,
		DetectedAt: now,
		Tools:      tools,
	}

	detector.mu.Lock()
	detector.cached = cloneToolchain(toolchain)
	detector.hasCache = true
	detector.mu.Unlock()
	return toolchain, nil
}

func (detector *Detector) detectOne(
	ctx context.Context,
	name string,
	pathValue string,
) Tool {
	tool := Tool{Name: name}
	path, commandPath, prefixArgs, err := detector.resolveTool(ctx, name)
	if path == "" {
		if err != nil && !errors.Is(err, ErrBinaryNotFound) {
			tool.Error = err.Error()
		}
		return tool
	}
	tool.Path = path
	if err != nil {
		// Found on PATH but not invocable — a Windows package-manager shim with
		// no JavaScript entrypoint, for example. Present means usable, so this
		// stays false and the reason is reported instead.
		tool.Error = err.Error()
		return tool
	}
	tool.Present = true

	probeCtx, cancel := context.WithTimeout(ctx, detector.commandTimeout)
	output, err := detector.probe(probeCtx, commandPath, name, pathValue, prefixArgs)
	cancel()
	if err != nil {
		tool.Error = err.Error()
		return tool
	}
	version, err := ParseToolVersion(name, output)
	if err != nil {
		tool.Error = err.Error()
		return tool
	}
	tool.Version = version
	return tool
}

// resolveTool performs one PATH walk per tool where the resolver supports it,
// returning the discovered path, the native command, and any fixed argv prefix.
// A non-empty path with a non-nil error means the tool exists but cannot be
// invoked.
func (detector *Detector) resolveTool(
	ctx context.Context,
	name string,
) (string, string, []string, error) {
	if resolver, ok := detector.resolver.(toolResolver); ok {
		return resolver.ResolveToolContext(ctx, name)
	}

	path, err := detector.resolver.Resolve(name)
	if err != nil {
		return "", "", nil, err
	}
	if resolver, ok := detector.resolver.(commandResolver); ok {
		command, prefixArgs, commandErr := resolver.ResolveCommand(name)
		if commandErr != nil {
			return path, "", nil, commandErr
		}
		return path, command, prefixArgs, nil
	}
	return path, path, nil, nil
}

func cloneToolchain(source Toolchain) Toolchain {
	clone := source
	clone.Tools = make(map[string]Tool, len(source.Tools))
	for name, tool := range source.Tools {
		clone.Tools[name] = tool
	}
	return clone
}

func probeVersion(
	ctx context.Context,
	path string,
	name string,
	pathValue string,
	prefixArgs []string,
) (string, error) {
	args := versionArguments(name)
	args = append(append([]string(nil), prefixArgs...), args...)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.WaitDelay = probeWaitDelay
	cmd.Env = replacePATH(os.Environ(), pathValue, runtime.GOOS)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return output.String(), fmt.Errorf("%s version probe failed: %w", name, err)
	}
	return output.String(), nil
}

func versionArguments(name string) []string {
	switch name {
	case "go", "wails":
		return []string{"version"}
	default:
		return []string{"--version"}
	}
}

var defaultDetector = NewDetector(defaultResolver)

// Detect scans with the package-level detector.
func Detect(force bool) (Toolchain, error) {
	return defaultDetector.Detect(context.Background(), force)
}
