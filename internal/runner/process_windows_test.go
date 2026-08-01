//go:build windows

package runner

import (
	"path/filepath"
	"testing"
)

func TestTaskkillPathIsAbsoluteUnderSystemRoot(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name       string
		systemRoot string
		want       string
	}{
		{name: "configured", systemRoot: `D:\Windows`, want: `D:\Windows\System32\taskkill.exe`},
		{name: "unset", systemRoot: "", want: `C:\Windows\System32\taskkill.exe`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := taskkillPath(testCase.systemRoot)
			if got != testCase.want {
				t.Fatalf("taskkillPath(%q) = %q, want %q", testCase.systemRoot, got, testCase.want)
			}
			if !filepath.IsAbs(got) {
				t.Fatalf("taskkillPath(%q) = %q, want an absolute path", testCase.systemRoot, got)
			}
		})
	}
}
