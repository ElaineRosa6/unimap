package web

import (
	"slices"
	"testing"

	"github.com/unimap/project/internal/config"
)

func TestBuildCDPChromeArgsUsesCloudSafeHeadlessDefaults(t *testing.T) {
	cfg := &config.Config{}
	cfg.Screenshot.NoSandbox = true
	args, userDataDir := buildCDPChromeArgs(cfg, 9222, "127.0.0.1", t.TempDir())
	for _, want := range []string{
		"--headless=new", "--no-sandbox", "--disable-setuid-sandbox",
		"--disable-dev-shm-usage", "--disable-gpu",
		"--remote-debugging-address=127.0.0.1",
	} {
		if !slices.Contains(args, want) {
			t.Errorf("launch args missing %q: %v", want, args)
		}
	}
	if userDataDir == "" || !slices.Contains(args, "--user-data-dir="+userDataDir) {
		t.Fatalf("expected an isolated user data dir, args=%v dir=%q", args, userDataDir)
	}
}

func TestBuildCDPChromeArgsRespectsExplicitHeadfulMode(t *testing.T) {
	headless := false
	cfg := &config.Config{}
	cfg.Screenshot.Headless = &headless
	args, _ := buildCDPChromeArgs(cfg, 9222, "127.0.0.1", t.TempDir())
	if slices.Contains(args, "--headless=new") {
		t.Fatalf("explicit headful mode unexpectedly added headless flag: %v", args)
	}
}
