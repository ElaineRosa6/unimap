package utils

import (
	"path/filepath"
	"testing"
)

func TestAppDataDirHonorsExplicitEnvironmentRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("UNIMAP_DATA_DIR", root)

	if got, want := AppDataDir("screenshots", "daily"), filepath.Join(root, "screenshots", "daily"); got != want {
		t.Fatalf("AppDataDir() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathHonorsExplicitEnvironmentPath(t *testing.T) {
	want := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("UNIMAP_CONFIG_PATH", want)

	if got := DefaultConfigPath(); got != want {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, want)
	}
}
