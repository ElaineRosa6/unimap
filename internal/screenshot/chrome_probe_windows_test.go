//go:build windows

package screenshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateChromeBinaryRejectsNonBrowserWindowsPE(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if validateChromeBinary(t.Context(), executable) {
		t.Fatalf("non-browser Windows executable %q passed Chrome validation", executable)
	}
}

func TestValidateChromeBinaryRejectsOrdinaryFileOnWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chrome.exe")
	if err := os.WriteFile(path, []byte("not a PE executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if validateChromeBinary(t.Context(), path) {
		t.Fatal("ordinary file passed Windows Chrome binary validation")
	}
}

func TestCDPHealthCheckerValidatesInstalledChromeWithoutLaunchingIt(t *testing.T) {
	path, err := ResolveChromePath("")
	if err != nil {
		t.Skipf("Chrome/Edge is not installed: %v", err)
	}
	healthy, err := (&CDPHealthChecker{ConfiguredChromePath: path}).Check(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !healthy {
		t.Fatalf("installed browser %q was not considered ready", path)
	}
}
