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

const (
	loginShellTimeout = 2 * time.Second
	// loginShellWaitDelay bounds how long a killed login shell may leave its
	// output pipe held open by a background process it started. Interactive
	// startup files routinely spawn daemons that inherit stdout, and without
	// this bound Output blocks well past the context deadline.
	loginShellWaitDelay = 1 * time.Second
)

type loginPathFunc func(context.Context, string) (string, error)

// PathResolver discovers the PATH visible to a login shell once and then keeps
// it in memory. An explicit override always wins and can be changed at runtime.
type PathResolver struct {
	mu sync.Mutex

	override string
	cached   string
	resolved bool

	// discoveryError records why login-shell discovery did not produce a PATH.
	// Swallowing it entirely is what makes the common macOS failure invisible: a
	// Finder-launched app inherits the minimal system PATH without Homebrew,
	// nvm, fnm, or volta, every tool then looks missing, and the doctor view
	// shows only the symptom.
	discoveryError string

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
	goos := runtime.GOOS
	return &PathResolver{
		processPath: os.Getenv("PATH"),
		shell:       loginShell(os.Getenv("SHELL"), goos),
		goos:        goos,
		timeout:     loginShellTimeout,
		discover:    discoverLoginShellPATH,
	}
}

func loginShell(configured, goos string) string {
	if configured != "" {
		return configured
	}
	if goos == "darwin" {
		// Applications launched from Finder may not receive SHELL. zsh has
		// been the default macOS shell since Catalina and is present at this
		// stable system path.
		return "/bin/zsh"
	}
	return ""
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
		discoveryError := ""
		if goos != "windows" && shell != "" {
			discoveryCtx, cancel := context.WithTimeout(ctx, timeout)
			discovered, err := discover(discoveryCtx, shell)
			cancel()
			switch {
			case err != nil:
				discoveryError = fmt.Sprintf(
					"PATH discovery through the login shell %s failed (%v); using the PATH "+
						"Nodesmith was started with. Tools installed by Homebrew or a Node "+
						"version manager may not be found.",
					shell,
					err,
				)
			case discovered == "":
				discoveryError = fmt.Sprintf(
					"The login shell %s returned no PATH; using the PATH Nodesmith was "+
						"started with. Tools installed by Homebrew or a Node version "+
						"manager may not be found.",
					shell,
				)
			default:
				path = discovered
			}
		}

		r.mu.Lock()
		r.cached = path
		r.discoveryError = discoveryError
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

// DiscoveryWarning returns a human-readable explanation of why login-shell PATH
// discovery did not produce a PATH, or an empty string when discovery succeeded,
// was not attempted, or has not run yet. Discovery failing is not fatal —
// ResolvedPath still returns the process PATH — but it is the root cause of
// "every tool is missing", so it belongs in the doctor view.
func (r *PathResolver) DiscoveryWarning() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.override != "" {
		return ""
	}
	return r.discoveryError
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
	//
	// -i is deliberate despite sourcing interactive startup files: nvm, fnm, and
	// Volta all install their PATH setup into .zshrc/.bashrc, which -l alone does
	// not read. See docs/adr/002-login-shell-path-discovery.md. The hang risk
	// that -i introduces is bounded by the context deadline plus WaitDelay below.
	cmd := exec.CommandContext(ctx, shell, "-l", "-i", "-c", "env")
	cmd.WaitDelay = loginShellWaitDelay
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
