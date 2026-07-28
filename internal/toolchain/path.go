package toolchain

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const loginShellTimeout = 2 * time.Second

type loginPathFunc func(context.Context, string) (string, error)

// PathResolver discovers the PATH visible to a login shell once and then keeps
// it in memory. An explicit override always wins and can be changed at runtime.
type PathResolver struct {
	mu sync.Mutex

	override string
	cached   string
	resolved bool

	resolving bool
	wait      chan struct{}

	processPath string
	shell       string
	goos        string
	timeout     time.Duration
	discover    loginPathFunc
}

// NewPathResolver creates a PATH resolver using the current process
// environment. macOS and Linux perform login-shell discovery; Windows uses the
// process PATH directly.
func NewPathResolver() *PathResolver {
	return &PathResolver{
		processPath: os.Getenv("PATH"),
		shell:       os.Getenv("SHELL"),
		goos:        runtime.GOOS,
		timeout:     loginShellTimeout,
		discover:    discoverLoginShellPATH,
	}
}

// ResolvedPath returns the effective PATH. Login-shell discovery is attempted
// at most once, has a two-second deadline, and falls back to the process PATH.
func (r *PathResolver) ResolvedPath(ctx context.Context) (string, error) {
	for {
		r.mu.Lock()
		if r.override != "" {
			path := r.override
			r.mu.Unlock()
			return path, nil
		}
		if r.resolved {
			path := r.cached
			r.mu.Unlock()
			return path, nil
		}
		if r.resolving {
			wait := r.wait
			r.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("waiting for PATH discovery: %w", ctx.Err())
			case <-wait:
				continue
			}
		}

		r.resolving = true
		r.wait = make(chan struct{})
		wait := r.wait
		processPath := r.processPath
		shell := r.shell
		goos := r.goos
		timeout := r.timeout
		discover := r.discover
		r.mu.Unlock()

		path := processPath
		if goos != "windows" && shell != "" {
			discoveryCtx, cancel := context.WithTimeout(ctx, timeout)
			discovered, err := discover(discoveryCtx, shell)
			cancel()
			if err == nil && discovered != "" {
				path = discovered
			}
		}

		r.mu.Lock()
		r.cached = path
		r.resolved = true
		r.resolving = false
		close(wait)
		if r.override != "" {
			path = r.override
		}
		r.mu.Unlock()
		return path, nil
	}
}

// SetOverride changes the effective PATH without restarting the application.
// Passing an empty string clears the override and reveals the cached discovered
// PATH again.
func (r *PathResolver) SetOverride(path string) error {
	if strings.IndexByte(path, 0) >= 0 {
		return errors.New("PATH override contains a NUL byte")
	}

	r.mu.Lock()
	r.override = path
	r.mu.Unlock()
	return nil
}

func discoverLoginShellPATH(ctx context.Context, shell string) (string, error) {
	// This is the sole shell invocation in the core. The command text is a
	// constant, and the shell plus each option are supplied as distinct argv
	// elements. No user or recipe value is interpolated into command text.
	cmd := exec.CommandContext(ctx, shell, "-l", "-c", "env")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("query login-shell environment: %w", err)
	}
	return parseEnvironmentPATH(output)
}

func parseEnvironmentPATH(output []byte) (string, error) {
	var path string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "PATH=") {
			path = strings.TrimSuffix(strings.TrimPrefix(line, "PATH="), "\r")
		}
	}
	if path == "" {
		return "", errors.New("login shell returned no PATH")
	}
	return path, nil
}

var defaultPathResolver = NewPathResolver()

// ResolvedPath returns the PATH used by the package-level resolver and
// detector.
func ResolvedPath() (string, error) {
	return defaultPathResolver.ResolvedPath(context.Background())
}

// SetPathOverride updates the PATH used by the package-level resolver and
// detector. The next detection automatically bypasses its cache because the
// effective PATH has changed.
func SetPathOverride(path string) error {
	return defaultPathResolver.SetOverride(path)
}
