//go:build live_e2e

package screenshot

import (
	"os"
	"testing"
	"time"
)

func TestLiveQuakeL1NetworkCollection(t *testing.T) {
	chromePath := os.Getenv("UNIMAP_CHROME_PATH")
	profilePath := os.Getenv("UNIMAP_LIVE_QUAKE_PROFILE")
	if chromePath == "" || profilePath == "" {
		t.Skip("UNIMAP_CHROME_PATH and UNIMAP_LIVE_QUAKE_PROFILE are required")
	}
	mgr := NewManager(Config{
		BaseDir:     t.TempDir(),
		ChromePath:  chromePath,
		UserDataDir: profilePath,
		Headless:    true,
		Timeout:     60 * time.Second,
		WaitTime:    2 * time.Second,
		MaxSessions: 1,
	})
	results, err := mgr.CollectViaNetwork(t.Context(), "quake", `port:"80"`, "live_quake_l1")
	if err != nil {
		t.Fatalf("Quake L1 network collection failed: %v", err)
	}
	if len(results) != 1 || len(results[0].Assets) == 0 {
		t.Fatalf("Quake L1 returned no structured assets")
	}
}
