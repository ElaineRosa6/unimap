package web

import (
	"testing"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/config"
)

func TestConfigReloadKeepsAllBrowserEnginesInWebOnlyMode(t *testing.T) {
	cfg := &config.Config{}
	cfg.Engines.Hunter.Enabled = true
	cfg.Engines.Censys.Enabled = true
	cfg.Engines.Daydaymap.Enabled = true
	orchestrator := adapter.NewEngineOrchestrator()
	server := &Server{config: cfg, orchestrator: orchestrator}

	server.registerCoreEngineAdapters(cfg)

	if _, ok := orchestrator.GetAdapter("hunter"); !ok {
		t.Fatal("Hunter without API credentials should remain available in Web-only mode after config reload")
	}
	if _, ok := orchestrator.GetAdapter("censys"); !ok {
		t.Fatal("Censys without API credentials should remain available in Web-only mode after config reload")
	}
	if _, ok := orchestrator.GetAdapter("daydaymap"); !ok {
		t.Fatal("DayDayMap without API credentials should remain available in Web-only mode after config reload")
	}
}

func TestConfigReloadRemovesDisabledAPIOnlyAdapters(t *testing.T) {
	cfg := &config.Config{}
	orchestrator := adapter.NewEngineOrchestrator()
	orchestrator.RegisterAdapter(adapter.NewCensysAdapter("https://search.censys.io", "test-id", "test-secret", "", "", 1, 0))
	orchestrator.RegisterAdapter(adapter.NewDayDayMapAdapter("https://www.daydaymap.com", "test-key", "", 1, 0))
	server := &Server{config: cfg, orchestrator: orchestrator}

	server.reloadEngineAdapters()

	if _, ok := orchestrator.GetAdapter("censys"); ok {
		t.Fatal("disabled Censys adapter remains registered after config reload")
	}
	if _, ok := orchestrator.GetAdapter("daydaymap"); ok {
		t.Fatal("disabled DayDayMap adapter remains registered after config reload")
	}
}
