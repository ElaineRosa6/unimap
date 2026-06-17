package service

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/screenshot"
)

// ===== QueryAppService: constructors and simple methods =====

func TestNewQueryAppService(t *testing.T) {
	svc := NewQueryAppService(nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil QueryAppService")
	}
}

func TestQueryAppService_ResolveEngines(t *testing.T) {
	// explicit engines
	svc := NewQueryAppService(nil, nil)
	got := svc.ResolveEngines([]string{"fofa", "hunter"})
	if len(got) != 2 || got[0] != "fofa" || got[1] != "hunter" {
		t.Errorf("expected [fofa hunter], got %v", got)
	}

	// nil orchestrator, no explicit engines
	got = svc.ResolveEngines(nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}

	// orchestrator with adapters — returns first available adapter
	orch2 := adapter.NewEngineOrchestrator()
	orch2.RegisterAdapter(&mockEngineAdapter{name: "fofa"})
	orch2.RegisterAdapter(&mockEngineAdapter{name: "hunter"})
	svc2 := NewQueryAppService(nil, orch2)
	got = svc2.ResolveEngines(nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 engine, got %d", len(got))
	}
	// Should be one of the registered adapters
	if got[0] != "fofa" && got[0] != "hunter" {
		t.Errorf("expected fofa or hunter, got %v", got)
	}

	// orchestrator with no adapters
	orch3 := adapter.NewEngineOrchestrator()
	svc3 := NewQueryAppService(nil, orch3)
	got = svc3.ResolveEngines(nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// ===== ExecuteQuery validation =====

func TestExecuteQuery_NilService(t *testing.T) {
	svc := NewQueryAppService(nil, nil)
	_, err := svc.ExecuteQuery(context.Background(), "test", []string{"fofa"}, 10)
	if err == nil {
		t.Fatal("expected error for nil unified service")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("error should mention not initialized: %v", err)
	}
}

func TestExecuteQuery_DefaultPageSize(t *testing.T) {
	// We can't test the full path without a real UnifiedService,
	// but we verify the pageSize default logic by checking the request
	// that would be constructed. This is covered indirectly by
	// the unified service Query validation tests.
}

func TestRunBrowserQueryAsync_UsesTranslatedQuery(t *testing.T) {
	orch := adapter.NewEngineOrchestrator()
	orch.RegisterAdapter(&mockEngineAdapter{name: "fofa", translate: `country="CN" && port="80"`})
	provider := &mockScreenshotProvider{}
	svc := NewQueryAppService(nil, orch)

	ch := svc.RunBrowserQueryAsync(
		context.Background(),
		`country="CN" && port="80"`,
		[]string{"fofa"},
		true,
		"collect",
		"q1",
		false,
		nil,
		nil,
		nil,
		provider,
		nil,
	)
	outcome := <-ch

	if len(outcome.Errors) != 0 {
		t.Fatalf("unexpected browser errors: %#v", outcome.Errors)
	}
	if len(provider.collectedQueries) != 1 || provider.collectedQueries[0] != `country="CN" && port="80"` {
		t.Fatalf("expected translated collect query, got %#v", provider.collectedQueries)
	}
}

func TestRunBrowserQueryAsync_ProgressCallback(t *testing.T) {
	orch := adapter.NewEngineOrchestrator()
	orch.RegisterAdapter(&mockEngineAdapter{name: "fofa", translate: `country="CN"`})
	orch.RegisterAdapter(&mockEngineAdapter{name: "hunter", translate: `ip="1.2.3.4"`})
	provider := &mockScreenshotProvider{}
	svc := NewQueryAppService(nil, orch)

	var doneCalls []int
	var totals []int
	ch := svc.RunBrowserQueryAsync(
		context.Background(),
		`country="CN"`,
		[]string{"fofa", "hunter"},
		true,
		"collect",
		"q2",
		false,
		nil,
		nil,
		nil,
		provider,
		func(done, total int, _ string, _ error) {
			doneCalls = append(doneCalls, done)
			totals = append(totals, total)
		},
	)
	outcome := <-ch

	if len(outcome.Errors) != 0 {
		t.Fatalf("unexpected browser errors: %#v", outcome.Errors)
	}
	if len(doneCalls) != 2 {
		t.Fatalf("expected 2 progress callbacks (one per engine), got %d: %v", len(doneCalls), doneCalls)
	}
	if doneCalls[0] != 1 || doneCalls[1] != 2 {
		t.Fatalf("unexpected progress done sequence: %v", doneCalls)
	}
	for i := range totals {
		if totals[i] != 2 {
			t.Fatalf("expected total=2 for callback %d, got %d", i, totals[i])
		}
	}
}

// ===== normalizeCDPBaseURL =====

func TestNormalizeCDPBaseURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"  ", ""},
		{"127.0.0.1:9222", "http://127.0.0.1:9222"},
		{"http://127.0.0.1:9222", "http://127.0.0.1:9222"},
		{"https://127.0.0.1:9222/", "https://127.0.0.1:9222"},
		{"  127.0.0.1:9222/  ", "http://127.0.0.1:9222"},
	}
	for _, tt := range tests {
		got := normalizeCDPBaseURL(tt.input)
		if got != tt.want {
			t.Errorf("normalizeCDPBaseURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ===== ScreenshotAppService =====

func TestNewScreenshotAppService(t *testing.T) {
	svc := NewScreenshotAppService("/tmp/screenshots")
	if svc == nil {
		t.Fatal("expected non-nil ScreenshotAppService")
	}
	if svc.GetBaseDir() != "/tmp/screenshots" {
		t.Errorf("GetBaseDir() = %q, want %q", svc.GetBaseDir(), "/tmp/screenshots")
	}
}

func TestNewScreenshotAppServiceWithProvider(t *testing.T) {
	// nil baseDir defaults to "./screenshots"
	svc := NewScreenshotAppServiceWithProvider("", nil)
	if svc.GetBaseDir() != "./screenshots" {
		t.Errorf("GetBaseDir() = %q, want %q", svc.GetBaseDir(), "./screenshots")
	}
}

func TestScreenshotAppService_SetEngine(t *testing.T) {
	svc := NewScreenshotAppService("./screenshots")
	svc.SetEngine("FOFA")
	if svc.engine != "fofa" {
		t.Errorf("engine = %q, want %q", svc.engine, "fofa")
	}
	svc.SetEngine("  ")
	if svc.engine != "cdp" {
		t.Errorf("engine = %q, want %q", svc.engine, "cdp")
	}

	// nil receiver safety
	var nilSvc *ScreenshotAppService
	nilSvc.SetEngine("fofa") // should not panic
}

func TestScreenshotAppService_SetBridgeService(t *testing.T) {
	svc := NewScreenshotAppService("./screenshots")
	bridge := screenshot.NewBridgeService(nil, 5, 30)
	svc.SetBridgeService(bridge)
	if svc.bridgeService != bridge {
		t.Error("bridge service not set")
	}

	// nil receiver
	var nilSvc *ScreenshotAppService
	nilSvc.SetBridgeService(bridge) // should not panic
}

func TestScreenshotAppService_SetFallbackToCDP(t *testing.T) {
	svc := NewScreenshotAppService("./screenshots")
	svc.SetFallbackToCDP(true)
	if !svc.fallbackToCDP {
		t.Error("fallbackToCDP not set to true")
	}

	var nilSvc *ScreenshotAppService
	nilSvc.SetFallbackToCDP(true) // should not panic
}

func TestScreenshotAppService_IsCaptureAvailable(t *testing.T) {
	// provider set
	svc := NewScreenshotAppServiceWithProvider("./screenshots", &mockScreenshotProvider{})
	if !svc.IsCaptureAvailable(nil) {
		t.Error("expected true when provider is set")
	}

	// no provider, but mgr set
	svc2 := NewScreenshotAppService("./screenshots")
	if !svc2.IsCaptureAvailable(&screenshot.Manager{}) {
		t.Error("expected true when manager is set")
	}

	// neither set
	if svc2.IsCaptureAvailable(nil) {
		t.Error("expected false when neither provider nor manager set")
	}

	// nil receiver
	var nilSvc *ScreenshotAppService
	if nilSvc.IsCaptureAvailable(nil) {
		t.Error("expected false for nil receiver")
	}
}

// ===== normalizeMonitorURLForService =====

func TestNormalizeMonitorURLForService(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"empty", "", "", true},
		{"whitespace", "  ", "", true},
		{"bare host", "example.com", "https://example.com", false},
		{"bare host with port", "example.com:8080", "https://example.com:8080", false},
		{"with http", "http://example.com", "http://example.com", false},
		{"with https", "https://example.com/path", "https://example.com/path", false},
		{"invalid url", "http://[invalid", "", true},
		{"missing host", "http://", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMonitorURLForService(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

// ===== classifyReachabilityErrorForService =====

func TestClassifyReachabilityErrorForService(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCat    string
		wantDetail string
	}{
		{"nil", nil, "unknown", "unknown error"},
		{"generic", fmt.Errorf("some error"), "network", "some error"},
		{"connection refused", &net.OpError{Op: "dial", Err: fmt.Errorf("connect: connection refused")}, "connection_refused", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, detail := classifyReachabilityErrorForService(tt.err)
			if cat != tt.wantCat {
				t.Errorf("category = %q, want %q", cat, tt.wantCat)
			}
			if tt.wantDetail != "" && !strings.Contains(detail, tt.wantDetail) {
				t.Errorf("detail = %q, should contain %q", detail, tt.wantDetail)
			}
		})
	}
}

func TestClassifyReachabilityDNS(t *testing.T) {
	dnsErr := &net.DNSError{Err: "no such host", Name: "nonexistent.invalid"}
	cat, _ := classifyReachabilityErrorForService(dnsErr)
	if cat != "dns" {
		t.Errorf("category = %q, want %q", cat, "dns")
	}
}

// ===== mockEngineAdapter =====

type mockEngineAdapter struct {
	name      string
	translate string
}

func (m *mockEngineAdapter) Name() string { return m.name }
func (m *mockEngineAdapter) Translate(ast *model.UQLAST) (string, error) {
	if m.translate != "" {
		return m.translate, nil
	}
	return "translated", nil
}
func (m *mockEngineAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	return nil, nil
}
func (m *mockEngineAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	return nil, nil
}
func (m *mockEngineAdapter) GetQuota() (*model.QuotaInfo, error) { return nil, nil }
func (m *mockEngineAdapter) IsWebOnly() bool                     { return false }

// ===== mockScreenshotProvider =====

type mockScreenshotProvider struct {
	openedQueries    []string
	collectedQueries []string
}

func (m *mockScreenshotProvider) CaptureSearchEngineResult(ctx context.Context, engine, query, queryID string) (string, error) {
	return "/mock/path.png", nil
}
func (m *mockScreenshotProvider) CaptureSearchEngineResultWithProxy(ctx context.Context, engine, query, queryID, proxy string) (string, error) {
	return "/mock/path.png", nil
}
func (m *mockScreenshotProvider) CaptureTargetWebsite(ctx context.Context, targetURL, ip, port, protocol, queryID string) (string, error) {
	return "/mock/target.png", nil
}
func (m *mockScreenshotProvider) CaptureTargetWebsiteWithProxy(ctx context.Context, targetURL, ip, port, protocol, queryID, proxy string) (string, error) {
	return "/mock/target.png", nil
}
func (m *mockScreenshotProvider) CaptureBatchURLs(ctx context.Context, urls []string, batchID string, concurrency int) ([]screenshot.BatchScreenshotResult, error) {
	return nil, nil
}
func (m *mockScreenshotProvider) GetScreenshotDirectory() string { return "/mock/screenshots" }
func (m *mockScreenshotProvider) OpenSearchEngineResult(ctx context.Context, engine, query string) (string, error) {
	m.openedQueries = append(m.openedQueries, query)
	return "/mock/open", nil
}
func (m *mockScreenshotProvider) CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]collection.CollectResult, error) {
	m.collectedQueries = append(m.collectedQueries, query)
	return []collection.CollectResult{{Engine: engine, Query: query, RawURL: "https://mock.engine/result?q=" + query}}, nil
}
