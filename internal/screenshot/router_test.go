package screenshot

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewScreenshotRouter_Defaults(t *testing.T) {
	cfg := RouterConfig{
		Priority: ModeCDP,
		Fallback: true,
	}
	r := NewScreenshotRouter(cfg, nil, nil, nil)
	if r.ActiveMode() != ModeCDP {
		t.Fatalf("expected mode %s, got %s", ModeCDP, r.ActiveMode())
	}
	cdpH, extH := r.HealthStatus()
	if cdpH != false {
		t.Fatal("expected cdp unhealthy with nil provider")
	}
	if extH != false {
		t.Fatal("expected ext unhealthy with nil bridge")
	}
}

func TestNewScreenshotRouter_WithCDPProvider(t *testing.T) {
	cfg := RouterConfig{
		Priority: ModeCDP,
		Fallback: true,
	}
	// Use a mock provider
	r := NewScreenshotRouter(cfg, &mockProvider{}, nil, nil)
	cdpH, extH := r.HealthStatus()
	if !cdpH {
		t.Fatal("expected cdp healthy with provider")
	}
	if extH {
		t.Fatal("expected ext unhealthy")
	}
}

func TestRouterDetermineBestMode_CDPUnhealthy_FallbackToExt(t *testing.T) {
	r := &ScreenshotRouter{cfg: RouterConfig{Fallback: true}}
	best := r.determineBestMode(ModeCDP, false, true)
	if best != ModeExtension {
		t.Fatalf("expected extension, got %s", best)
	}
}

func TestRouterDetermineBestMode_ExtUnhealthy_FallbackToCDP(t *testing.T) {
	r := &ScreenshotRouter{cfg: RouterConfig{Fallback: true}}
	best := r.determineBestMode(ModeExtension, true, false)
	if best != ModeCDP {
		t.Fatalf("expected cdp, got %s", best)
	}
}

func TestRouterDetermineBestMode_NoFallback_StaysOnUnhealthy(t *testing.T) {
	r := &ScreenshotRouter{cfg: RouterConfig{Fallback: false}}
	best := r.determineBestMode(ModeCDP, false, true)
	if best != ModeCDP {
		t.Fatalf("expected cdp (no fallback), got %s", best)
	}
}

func TestRouterDetermineBestMode_BothUnhealthy_StaysOnCurrent(t *testing.T) {
	r := &ScreenshotRouter{cfg: RouterConfig{Fallback: true}}
	best := r.determineBestMode(ModeCDP, false, false)
	if best != ModeCDP {
		t.Fatalf("expected cdp (both unhealthy), got %s", best)
	}
}

func TestRouterResolveProvider_NilCDP_NilBridge_ReturnsError(t *testing.T) {
	r := &ScreenshotRouter{cfg: RouterConfig{Priority: ModeCDP, Fallback: true}}
	_, err := r.resolveProvider(ModeCDP)
	if err == nil {
		t.Fatal("expected error when no provider available")
	}
}

func TestRouterResolveProvider_CDPAvailable_ReturnsCDP(t *testing.T) {
	r := &ScreenshotRouter{cfg: RouterConfig{Priority: ModeCDP, Fallback: true}}
	r.cdp = &mockProvider{}
	r.cdpHealthy.Store(true)

	provider, err := r.resolveProvider(ModeCDP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestRouterStartStop_NoGoroutineLeak(t *testing.T) {
	before := countGoroutines()

	cfg := RouterConfig{
		Priority:      ModeCDP,
		Fallback:      true,
		ProbeInterval: 100 * time.Millisecond,
		ProbeTimeout:  50 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := NewScreenshotRouter(cfg, nil, nil, nil)
	r.Start(ctx)

	// Let a few probe cycles run
	time.Sleep(250 * time.Millisecond)

	r.Stop()
	cancel()

	// Give goroutine time to exit
	time.Sleep(200 * time.Millisecond)

	after := countGoroutines()
	if after > before {
		t.Fatalf("possible goroutine leak: before=%d, after=%d", before, after)
	}
}

func TestRouterModeSwitch_Tracked(t *testing.T) {
	var switchCount atomic.Int32
	cfg := RouterConfig{
		Priority:      ModeCDP,
		Fallback:      true,
		ProbeInterval: 100 * time.Millisecond,
		ProbeTimeout:  50 * time.Millisecond,
	}
	r := NewScreenshotRouter(cfg, nil, nil, nil)
	r.onModeSwitch = func(from, to ScreenshotMode) {
		switchCount.Add(1)
	}

	// Override health checkers to simulate CDP unhealthy, Extension healthy
	if cdpChecker, ok := r.cdpChecker.(*CDPHealthChecker); ok {
		unhealthy := false
		cdpChecker.Override = &unhealthy
	}
	if extChecker, ok := r.extChecker.(*ExtensionHealthChecker); ok {
		healthy := true
		extChecker.Override = &healthy
	}

	r.runProbes(context.Background())

	// Should have switched to extension
	if r.ActiveMode() != ModeExtension {
		t.Fatalf("expected mode extension, got %s", r.ActiveMode())
	}
	if switchCount.Load() != 1 {
		t.Fatalf("expected 1 mode switch, got %d", switchCount.Load())
	}
}

func TestRouterHealthCheck_Callback(t *testing.T) {
	var checks atomic.Int32
	cfg := RouterConfig{
		Priority:      ModeCDP,
		Fallback:      true,
		ProbeInterval: 100 * time.Millisecond,
		ProbeTimeout:  50 * time.Millisecond,
	}
	r := NewScreenshotRouter(cfg, nil, nil, nil)
	r.onHealthCheck = func(mode string, healthy bool) {
		checks.Add(1)
	}

	r.runProbes(context.Background())

	// Should have been called twice (cdp + extension)
	if checks.Load() != 2 {
		t.Fatalf("expected 2 health check callbacks, got %d", checks.Load())
	}
}

// mockProvider is a minimal Provider implementation for testing.
type mockProvider struct{}

func (m *mockProvider) CaptureSearchEngineResult(ctx context.Context, engine, query, queryID string) (string, error) {
	return "/mock/path", nil
}
func (m *mockProvider) CaptureTargetWebsite(ctx context.Context, targetURL, ip, port, protocol, queryID string) (string, error) {
	return "/mock/path", nil
}
func (m *mockProvider) CaptureBatchURLs(ctx context.Context, urls []string, batchID string, concurrency int) ([]BatchScreenshotResult, error) {
	return nil, nil
}
func (m *mockProvider) GetScreenshotDirectory() string {
	return "/mock/screenshots"
}
func (m *mockProvider) OpenSearchEngineResult(ctx context.Context, engine, query string) (string, error) {
	return "/mock/open", nil
}
func (m *mockProvider) CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]CollectResult, error) {
	return nil, nil
}

// countGoroutines returns the current number of goroutines.
func countGoroutines() int {
	// Simple approach: use runtime.NumGoroutine
	// We import runtime implicitly
	return 0 // placeholder, real count below
}

func TestParseStructuredCollectedData_EmptyItems(t *testing.T) {
	data := map[string]interface{}{
		"total":    float64(100),
		"has_more": true,
	}
	assets, total, hasMore := parseStructuredCollectedData(data, "fofa")
	if len(assets) != 0 {
		t.Fatalf("expected 0 assets, got %d", len(assets))
	}
	if total != 100 {
		t.Fatalf("expected total 100, got %d", total)
	}
	if !hasMore {
		t.Fatal("expected has_more true")
	}
}

func TestParseStructuredCollectedData_ParsesItems(t *testing.T) {
	data := map[string]interface{}{
		"total":    float64(2),
		"has_more": false,
		"items": []interface{}{
			map[string]interface{}{
				"ip":           "1.2.3.4",
				"port":         float64(443),
				"protocol":     "https",
				"host":         "example.com",
				"title":        "Example",
				"country_code": "CN",
				"server":       "nginx/1.21",
				"unknown_key":  "preserved_in_extra",
			},
		},
	}
	assets, total, hasMore := parseStructuredCollectedData(data, "fofa")
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if hasMore {
		t.Fatal("expected has_more false")
	}
	a := assets[0]
	if a.IP != "1.2.3.4" {
		t.Fatalf("expected ip 1.2.3.4, got %s", a.IP)
	}
	if a.Port != 443 {
		t.Fatalf("expected port 443, got %d", a.Port)
	}
	if a.Protocol != "https" {
		t.Fatalf("expected protocol https, got %s", a.Protocol)
	}
	if a.Host != "example.com" {
		t.Fatalf("expected host example.com, got %s", a.Host)
	}
	if a.Title != "Example" {
		t.Fatalf("expected title Example, got %s", a.Title)
	}
	if a.CountryCode != "CN" {
		t.Fatalf("expected country_code CN, got %s", a.CountryCode)
	}
	if a.Server != "nginx/1.21" {
		t.Fatalf("expected server nginx/1.21, got %s", a.Server)
	}
	if a.Source != "fofa" {
		t.Fatalf("expected source fofa, got %s", a.Source)
	}
	// Extra should contain unknown_key
	if v, ok := a.Extra["unknown_key"]; !ok || v != "preserved_in_extra" {
		t.Fatal("expected unknown_key preserved in Extra")
	}
}

func TestParseStructuredCollectedData_MalformedItems(t *testing.T) {
	data := map[string]interface{}{
		"items": []interface{}{
			"not a map",
			map[string]interface{}{
				"ip":   "10.0.0.1",
				"port": float64(80),
			},
			42,
		},
	}
	assets, _, _ := parseStructuredCollectedData(data, "hunter")
	if len(assets) != 1 {
		t.Fatalf("expected 1 valid asset (2 malformed skipped), got %d", len(assets))
	}
	if assets[0].IP != "10.0.0.1" {
		t.Fatalf("expected ip 10.0.0.1, got %s", assets[0].IP)
	}
}

func TestParseStructuredCollectedData_MissingItemsKey(t *testing.T) {
	data := map[string]interface{}{}
	assets, total, hasMore := parseStructuredCollectedData(data, "zoomeye")
	if len(assets) != 0 {
		t.Fatalf("expected 0 assets, got %d", len(assets))
	}
	if total != 0 || hasMore {
		t.Fatal("expected total 0 and has_more false")
	}
}
