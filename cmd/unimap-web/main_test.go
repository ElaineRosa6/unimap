package main

import (
	"testing"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/service"
)

func TestRegisterEnginesMakesAllBrowserEnginesAvailableWithoutAPICredentials(t *testing.T) {
	cfg := &config.Config{}
	cfg.Engines.Hunter.Enabled = true
	cfg.Engines.Censys.Enabled = true
	cfg.Engines.Daydaymap.Enabled = true

	svc := service.NewUnifiedService()
	registerEngines(svc, cfg)

	if _, ok := svc.GetOrchestrator().GetAdapter("hunter"); !ok {
		t.Fatal("Hunter without API credentials should remain available in Web-only mode")
	}
	if _, ok := svc.GetOrchestrator().GetAdapter("censys"); !ok {
		t.Fatal("Censys without API credentials should remain available in Web-only mode")
	}
	if _, ok := svc.GetOrchestrator().GetAdapter("daydaymap"); !ok {
		t.Fatal("DayDayMap without API credentials should remain available in Web-only mode")
	}
}
