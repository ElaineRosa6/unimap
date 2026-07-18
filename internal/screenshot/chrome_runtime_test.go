package screenshot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveChromePathPrefersConfiguredExecutable(t *testing.T) {
	t.Setenv("UNIMAP_CHROME_PATH", "")
	path := filepath.Join(t.TempDir(), "custom-chrome")
	if err := os.WriteFile(path, []byte("test executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveChromePath(path)
	if err != nil {
		t.Fatalf("configured Chrome path was rejected: %v", err)
	}
	if got != path {
		t.Fatalf("ResolveChromePath() = %q, want %q", got, path)
	}
}

func TestResolveChromePathRejectsMissingConfiguredExecutable(t *testing.T) {
	t.Setenv("UNIMAP_CHROME_PATH", "")
	missing := filepath.Join(t.TempDir(), "missing-chrome")

	if _, err := ResolveChromePath(missing); err == nil {
		t.Fatal("expected an explicit missing Chrome path to return an error")
	}
}

func TestCDPHealthCheckerUsesConfiguredChromePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-chrome")
	if err := os.WriteFile(path, []byte("test executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	checker := &CDPHealthChecker{
		ConfiguredChromePath: path,
		LocalChromeFinder:    func() string { return "" },
		ChromeProbe:          func(_ context.Context, got string) bool { return got == path },
	}
	healthy, err := checker.Check(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !healthy {
		t.Fatal("configured Chrome executable should make CDP spawnable")
	}
}

func TestCDPHealthCheckerRejectsNonChromeExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-chrome")
	if err := os.WriteFile(path, []byte("not a browser"), 0o755); err != nil {
		t.Fatal(err)
	}
	healthy, err := (&CDPHealthChecker{ConfiguredChromePath: path}).Check(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if healthy {
		t.Fatal("ordinary executable must not satisfy Chrome readiness")
	}
}
