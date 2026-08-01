package toolchain

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPathResolverDiscoversOnceAndSupportsRuntimeOverride(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	resolver := &PathResolver{
		processPath: "/process/bin",
		shell:       "/bin/example-shell",
		goos:        "darwin",
		timeout:     time.Second,
		discover: func(context.Context, string) (string, error) {
			calls.Add(1)
			return "/login/bin:/usr/bin", nil
		},
	}

	first, err := resolver.ResolvedPath(context.Background())
	if err != nil {
		t.Fatalf("ResolvedPath() error = %v", err)
	}
	second, err := resolver.ResolvedPath(context.Background())
	if err != nil {
		t.Fatalf("second ResolvedPath() error = %v", err)
	}
	if first != "/login/bin:/usr/bin" || second != first {
		t.Fatalf("resolved paths = %q, %q", first, second)
	}
	if calls.Load() != 1 {
		t.Fatalf("discovery calls = %d, want 1", calls.Load())
	}

	if err := resolver.SetOverride("/custom/bin"); err != nil {
		t.Fatalf("SetOverride() error = %v", err)
	}
	overridden, err := resolver.ResolvedPath(context.Background())
	if err != nil {
		t.Fatalf("overridden ResolvedPath() error = %v", err)
	}
	if overridden != "/custom/bin" {
		t.Fatalf("overridden path = %q", overridden)
	}
	if err := resolver.SetOverride(""); err != nil {
		t.Fatalf("clear override error = %v", err)
	}
	cleared, err := resolver.ResolvedPath(context.Background())
	if err != nil {
		t.Fatalf("cleared ResolvedPath() error = %v", err)
	}
	if cleared != first {
		t.Fatalf("cleared path = %q, want cached %q", cleared, first)
	}
	if calls.Load() != 1 {
		t.Fatalf("discovery calls after overrides = %d, want 1", calls.Load())
	}
}

func TestPathResolverOverrideSkipsDiscovery(t *testing.T) {
	t.Parallel()

	resolver := &PathResolver{
		processPath: "/process/bin",
		shell:       "/bin/example-shell",
		goos:        "linux",
		timeout:     time.Second,
		discover: func(context.Context, string) (string, error) {
			t.Fatal("discovery should not run while an override is set")
			return "", nil
		},
	}
	if err := resolver.SetOverride("/override/bin"); err != nil {
		t.Fatalf("SetOverride() error = %v", err)
	}
	got, err := resolver.ResolvedPath(context.Background())
	if err != nil {
		t.Fatalf("ResolvedPath() error = %v", err)
	}
	if got != "/override/bin" {
		t.Fatalf("ResolvedPath() = %q", got)
	}
}

func TestPathResolverFallsBackOnErrorAndTimeout(t *testing.T) {
	t.Parallel()

	tests := map[string]loginPathFunc{
		"error": func(context.Context, string) (string, error) {
			return "", errors.New("login failed")
		},
		"timeout": func(ctx context.Context, _ string) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	for name, discover := range tests {
		discover := discover
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			resolver := &PathResolver{
				processPath: "/fallback/bin",
				shell:       "/bin/example-shell",
				goos:        "darwin",
				timeout:     10 * time.Millisecond,
				discover:    discover,
			}
			got, err := resolver.ResolvedPath(context.Background())
			if err != nil {
				t.Fatalf("ResolvedPath() error = %v", err)
			}
			if got != "/fallback/bin" {
				t.Fatalf("ResolvedPath() = %q, want fallback", got)
			}
			// Falling back silently is what makes "every tool is missing" look
			// like a broken install rather than a PATH problem.
			warning := resolver.DiscoveryWarning()
			if warning == "" {
				t.Fatal("DiscoveryWarning() = \"\", want the fallback reason retained")
			}
			if !strings.Contains(warning, "/bin/example-shell") {
				t.Fatalf("DiscoveryWarning() = %q, want the shell named", warning)
			}
		})
	}
}

func TestPathResolverDiscoveryWarning(t *testing.T) {
	t.Parallel()

	t.Run("successful discovery leaves no warning", func(t *testing.T) {
		t.Parallel()
		resolver := &PathResolver{
			processPath: "/fallback/bin",
			shell:       "/bin/example-shell",
			goos:        "darwin",
			timeout:     time.Second,
			discover: func(context.Context, string) (string, error) {
				return "/discovered/bin", nil
			},
		}
		if _, err := resolver.ResolvedPath(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := resolver.DiscoveryWarning(); got != "" {
			t.Fatalf("DiscoveryWarning() = %q, want empty", got)
		}
	})

	t.Run("an empty PATH from the shell is reported", func(t *testing.T) {
		t.Parallel()
		resolver := &PathResolver{
			processPath: "/fallback/bin",
			shell:       "/bin/example-shell",
			goos:        "darwin",
			timeout:     time.Second,
			discover: func(context.Context, string) (string, error) {
				return "", nil
			},
		}
		if _, err := resolver.ResolvedPath(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := resolver.DiscoveryWarning(); !strings.Contains(got, "no PATH") {
			t.Fatalf("DiscoveryWarning() = %q, want an empty-PATH explanation", got)
		}
	})

	// An explicit override replaces discovery entirely, so a stale discovery
	// failure is no longer something the user needs to act on.
	t.Run("an override suppresses the warning", func(t *testing.T) {
		t.Parallel()
		resolver := &PathResolver{
			processPath: "/fallback/bin",
			shell:       "/bin/example-shell",
			goos:        "darwin",
			timeout:     time.Second,
			discover: func(context.Context, string) (string, error) {
				return "", errors.New("login failed")
			},
		}
		if _, err := resolver.ResolvedPath(context.Background()); err != nil {
			t.Fatal(err)
		}
		if resolver.DiscoveryWarning() == "" {
			t.Fatal("DiscoveryWarning() = \"\", want a warning before the override")
		}
		if err := resolver.SetOverride("/manual/bin"); err != nil {
			t.Fatal(err)
		}
		if got := resolver.DiscoveryWarning(); got != "" {
			t.Fatalf("DiscoveryWarning() = %q, want empty once overridden", got)
		}
	})

	t.Run("windows never attempts discovery", func(t *testing.T) {
		t.Parallel()
		resolver := &PathResolver{
			processPath: `C:\node`,
			shell:       "powershell.exe",
			goos:        "windows",
			timeout:     time.Second,
			discover: func(context.Context, string) (string, error) {
				t.Fatal("discovery ran on Windows")
				return "", nil
			},
		}
		if _, err := resolver.ResolvedPath(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := resolver.DiscoveryWarning(); got != "" {
			t.Fatalf("DiscoveryWarning() = %q, want empty", got)
		}
	})
}

func TestPathResolverWindowsNeverLaunchesLoginShell(t *testing.T) {
	t.Parallel()

	resolver := &PathResolver{
		processPath: `C:\Program Files\node`,
		shell:       "powershell.exe",
		goos:        "windows",
		timeout:     time.Second,
		discover: func(context.Context, string) (string, error) {
			t.Fatal("Windows PATH resolution launched a login shell")
			return "", nil
		},
	}
	got, err := resolver.ResolvedPath(context.Background())
	if err != nil {
		t.Fatalf("ResolvedPath() error = %v", err)
	}
	if got != `C:\Program Files\node` {
		t.Fatalf("ResolvedPath() = %q", got)
	}
}

func TestLoginShellUsesMacOSDefaultWhenFinderOmitsShell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configured string
		goos       string
		want       string
	}{
		{name: "configured shell wins", configured: "/opt/homebrew/bin/fish", goos: "darwin", want: "/opt/homebrew/bin/fish"},
		{name: "macOS Finder fallback", goos: "darwin", want: "/bin/zsh"},
		{name: "Linux has no assumed shell", goos: "linux", want: ""},
		{name: "Windows has no assumed shell", goos: "windows", want: ""},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := loginShell(test.configured, test.goos); got != test.want {
				t.Fatalf("loginShell(%q, %q) = %q, want %q", test.configured, test.goos, got, test.want)
			}
		})
	}
}

func TestDiscoverLoginShellPATHUsesInteractiveLoginShell(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("executable shell fixture requires Unix")
	}

	shell := filepath.Join(t.TempDir(), "test-shell")
	script := `#!/bin/sh
if [ "$1" != "-l" ] || [ "$2" != "-i" ] || [ "$3" != "-c" ] || [ "$4" != "env" ]; then
	exit 2
fi
printf 'PATH=/interactive/bin:/usr/bin\n'
`
	if err := os.WriteFile(shell, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := discoverLoginShellPATH(context.Background(), shell)
	if err != nil {
		t.Fatalf("discoverLoginShellPATH() error = %v", err)
	}
	if got != "/interactive/bin:/usr/bin" {
		t.Fatalf("discoverLoginShellPATH() = %q", got)
	}
}

func TestPathResolverCoalescesConcurrentDiscovery(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	resolver := &PathResolver{
		processPath: "/fallback/bin",
		shell:       "/bin/example-shell",
		goos:        "linux",
		timeout:     time.Second,
		discover: func(context.Context, string) (string, error) {
			if calls.Add(1) == 1 {
				close(started)
			}
			<-release
			return "/login/bin", nil
		},
	}

	const goroutines = 12
	results := make(chan string, goroutines)
	var wait sync.WaitGroup
	for range goroutines {
		wait.Add(1)
		go func() {
			defer wait.Done()
			path, err := resolver.ResolvedPath(context.Background())
			if err != nil {
				results <- "error: " + err.Error()
				return
			}
			results <- path
		}()
	}
	<-started
	close(release)
	wait.Wait()
	close(results)

	for result := range results {
		if result != "/login/bin" {
			t.Errorf("concurrent result = %q", result)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("discovery calls = %d, want 1", calls.Load())
	}
}

func TestPathResolverWaitingCallerCanCancel(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	resolver := &PathResolver{
		processPath: "/fallback/bin",
		shell:       "/bin/example-shell",
		goos:        "linux",
		timeout:     time.Second,
		discover: func(context.Context, string) (string, error) {
			close(started)
			<-release
			return "/login/bin", nil
		},
	}
	done := make(chan struct{})
	go func() {
		_, _ = resolver.ResolvedPath(context.Background())
		close(done)
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolver.ResolvedPath(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting ResolvedPath() error = %v, want context.Canceled", err)
	}
	close(release)
	<-done
}

func TestPathResolverRejectsNULOverride(t *testing.T) {
	t.Parallel()

	resolver := NewPathResolver()
	if err := resolver.SetOverride("one\x00two"); err == nil {
		t.Fatal("SetOverride() accepted a NUL byte")
	}
}

func TestParseEnvironmentPATH(t *testing.T) {
	t.Parallel()

	got, err := parseEnvironmentPATH([]byte("SHELL=/bin/zsh\nPATH=/one:/two\r\nHOME=/tmp\n"))
	if err != nil {
		t.Fatalf("parseEnvironmentPATH() error = %v", err)
	}
	if got != "/one:/two" {
		t.Fatalf("parseEnvironmentPATH() = %q", got)
	}
	if _, err := parseEnvironmentPATH([]byte("HOME=/tmp\n")); err == nil {
		t.Fatal("parseEnvironmentPATH() accepted environment without PATH")
	}
}
