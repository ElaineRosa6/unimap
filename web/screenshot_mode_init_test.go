package web

import (
	"context"
	"testing"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/service"
)

func TestInitScreenshotModeAlwaysCreatesUnifiedRouter(t *testing.T) {
	for _, mode := range []string{"cdp", "extension", "auto"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			cfg := &config.Config{}
			cfg.Screenshot.Enabled = true
			cfg.Screenshot.Mode = mode
			cfg.Screenshot.Priority = mode
			if mode == "auto" {
				cfg.Screenshot.Priority = "cdp"
			}
			cfg.Screenshot.Extension.MaxConcurrency = 1
			cfg.Screenshot.Extension.TaskTimeoutSeconds = 1
			fallback := false
			cfg.Screenshot.Fallback = &fallback

			srv := &Server{bridge: &BridgeState{
				Tokens:   make(map[string]int64),
				LastSeen: make(map[string]int64),
			}}
			app := service.NewScreenshotAppService("./screenshots")

			initScreenshotMode(srv, cfg, nil, nil, app, ctx)

			if srv.screenshotRouter == nil {
				t.Fatalf("mode %q did not initialize ScreenshotRouter", mode)
			}
			if got := srv.screenshotRouter.ConfiguredMode(); got != screenshot.ScreenshotMode(mode) {
				t.Fatalf("configured router mode = %q, want %q", got, mode)
			}
			if provider := srv.browserQueryProvider(); provider != srv.screenshotRouter {
				t.Fatalf("browser query provider = %T, want unified router", provider)
			}
			if !app.IsCaptureAvailable(nil) {
				t.Fatal("screenshot app was not wired to the unified router")
			}
		})
	}
}
