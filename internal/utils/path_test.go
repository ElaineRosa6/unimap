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
