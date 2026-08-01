package toolchain

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"nodesmith/internal/allowlist"
)

func TestVersionArgumentsUseSupportedToolSyntax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want []string
	}{
		{name: "node", want: []string{"--version"}},
		{name: "npm", want: []string{"--version"}},
		{name: "go", want: []string{"version"}},
		{name: "wails", want: []string{"version"}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := versionArguments(test.name); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("versionArguments(%q) = %#v, want %#v", test.name, got, test.want)
			}
		})
	}
}

type fakeBinaryResolver struct {
	path     string
	missing  map[string]bool
	resolves atomic.Int32
}

func (resolver *fakeBinaryResolver) Resolve(name string) (string, error) {
	resolver.resolves.Add(1)
	if resolver.missing[name] {
		return "", fmt.Errorf("%w: %s", ErrBinaryNotFound, name)
	}
	return "/tools/" + name, nil
}

func (resolver *fakeBinaryResolver) ResolvedPath(context.Context) (string, error) {
	return resolver.path, nil
}

func TestDetectorCachesForSixtySecondsAndForceBypasses(t *testing.T) {
	t.Parallel()

	resolver := &fakeBinaryResolver{path: "/tools"}
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	detector := NewDetector(resolver)
	detector.now = func() time.Time { return now }
	detector.probe = fakeVersionProbe

	first, err := detector.Detect(context.Background(), false)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got := resolver.resolves.Load(); got != int32(len(allowlist.Detected())) {
		t.Fatalf("resolve calls = %d", got)
	}

	// A caller cannot mutate the cached map through a returned value.
	delete(first.Tools, "node")
	now = now.Add(59 * time.Second)
	started := time.Now()
	second, err := detector.Detect(context.Background(), false)
	if err != nil {
		t.Fatalf("warm Detect() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed >= 400*time.Millisecond {
		t.Fatalf("warm Detect() took %s, want under 400ms", elapsed)
	}
	if _, found := second.Tools["node"]; !found {
		t.Fatal("cached Toolchain was mutated by caller")
	}
	if got := resolver.resolves.Load(); got != int32(len(allowlist.Detected())) {
		t.Fatalf("warm resolve calls = %d, want unchanged", got)
	}

	if _, err := detector.Detect(context.Background(), true); err != nil {
		t.Fatalf("forced Detect() error = %v", err)
	}
	if got := resolver.resolves.Load(); got != int32(2*len(allowlist.Detected())) {
		t.Fatalf("forced resolve calls = %d", got)
	}
}

func TestDetectorExpiresAtSixtySecondsAndPathChangeInvalidates(t *testing.T) {
	t.Parallel()

	resolver := &fakeBinaryResolver{path: "/one"}
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	detector := NewDetector(resolver)
	detector.now = func() time.Time { return now }
	detector.probe = fakeVersionProbe

	if _, err := detector.Detect(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(60 * time.Second)
	if _, err := detector.Detect(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if got := resolver.resolves.Load(); got != int32(2*len(allowlist.Detected())) {
		t.Fatalf("post-expiry resolve calls = %d", got)
	}

	resolver.path = "/two"
	if _, err := detector.Detect(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if got := resolver.resolves.Load(); got != int32(3*len(allowlist.Detected())) {
		t.Fatalf("post-PATH-change resolve calls = %d", got)
	}
}

func TestDetectorRepresentsMissingAndProbeFailuresWithoutFailingScan(t *testing.T) {
	t.Parallel()

	resolver := &fakeBinaryResolver{
		path:    "/tools",
		missing: map[string]bool{"node": true},
	}
	detector := NewDetector(resolver)
	detector.probe = func(
		_ context.Context,
		_ string,
		name string,
		_ string,
		_ []string,
	) (string, error) {
		if name == "npm" {
			return "", errors.New("probe failed")
		}
		return fakeVersionProbe(context.Background(), "", name, "", nil)
	}

	toolchain, err := detector.Detect(context.Background(), false)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	node := toolchain.Tools["node"]
	if node.Present || node.Error != "" {
		t.Fatalf("missing node = %#v, want clean absent result", node)
	}
	npm := toolchain.Tools["npm"]
	if !npm.Present || npm.Error == "" || npm.Version != "" {
		t.Fatalf("failed npm probe = %#v", npm)
	}
}

func fakeVersionProbe(
	_ context.Context,
	_ string,
	name string,
	_ string,
	_ []string,
) (string, error) {
	switch name {
	case "git":
		return "git version 2.45.2", nil
	case "go":
		return "go version go1.25.0 darwin/arm64", nil
	case "cargo":
		return "cargo 1.80.1 (hash date)", nil
	case "gh":
		return "gh version 2.54.0 (date)", nil
	default:
		return "20.1.2", nil
	}
}
