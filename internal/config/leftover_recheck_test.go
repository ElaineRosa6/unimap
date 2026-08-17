package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func exampleConfigPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "configs", "config.yaml.example")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("example config missing: %v", err)
	}
	return path
}

func TestLeftover_ExampleConfigEvidenceScreenshotDisabled(t *testing.T) {
	data, err := os.ReadFile(exampleConfigPath(t))
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}
	if !strings.Contains(string(data), "evidence_screenshot_enabled: false") {
		t.Fatal("example config must declare tamper.evidence_screenshot_enabled: false")
	}
}

func TestLeftover_ExampleConfigLoadThroughManager(t *testing.T) {
	mgr := NewManager(exampleConfigPath(t))
	err := mgr.Load()
	cfg := mgr.GetConfig()
	if cfg == nil {
		t.Fatal("Load must still publish a config snapshot")
	}
	if err == nil {
		if cfg.Tamper.EvidenceScreenshotEnabled {
			t.Fatal("loaded example config enabled evidence screenshots")
		}
		return
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("Load example config failed unexpectedly: %v", err)
	}
	if cfg.Tamper.EvidenceScreenshotEnabled {
		t.Fatal("fallback defaults must keep evidence screenshots disabled")
	}
}

func TestLeftover_ApplyDefaultsLeavesEvidenceScreenshotOff(t *testing.T) {
	mgr := NewManager("")
	var cfg Config
	mgr.applyDefaults(&cfg)
	if cfg.Tamper.EvidenceScreenshotEnabled {
		t.Fatal("defaults must not enable tamper evidence screenshots")
	}
}
