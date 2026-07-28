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
	versionProbeTimeout = 2 * time.Second
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
	path, err := detector.resolver.Resolve(name)
	if err != nil {
		if !errors.Is(err, ErrBinaryNotFound) {
			tool.Error = err.Error()
		}
		return tool
	}
	tool.Path = path
	tool.Present = true

	commandPath := path
	var prefixArgs []string
	if resolver, ok := detector.resolver.(commandResolver); ok {
		commandPath, prefixArgs, err = resolver.ResolveCommand(name)
		if err != nil {
			tool.Error = err.Error()
			return tool
		}
	}
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
