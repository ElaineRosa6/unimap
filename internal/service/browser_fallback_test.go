package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/core/unimap"
	"github.com/unimap/project/internal/model"
)

// fakeBrowserBackend records calls and returns canned results / errors.
type fakeBrowserBackend struct {
	calls     int32
	gotEngine string
	gotQuery  string
	results   []collection.CollectResult
	err       error
}

func (f *fakeBrowserBackend) CollectSearchEngineResult(_ context.Context, engine, query, _ string) ([]collection.CollectResult, error) {
	atomic.AddInt32(&f.calls, 1)
	f.gotEngine = engine
	f.gotQuery = query
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

func (f *fakeBrowserBackend) callCount() int { return int(atomic.LoadInt32(&f.calls)) }

func newServiceWith(backend *fakeBrowserBackend, cfg *BrowserFallbackConfig) *UnifiedService {
	s := NewUnifiedService()
	if backend != nil {
		s.browserBackend = backend
	}
	s.browserFallbackCfg = cfg
	return s
}

func cfg(enabled, onErr, onEmpty bool, engines ...string) *BrowserFallbackConfig {
	set := make(map[string]bool, len(engines))
	for _, e := range engines {
		set[e] = true
	}
	return &BrowserFallbackConfig{
		Enabled:       enabled,
		OnAPIError:    onErr,
		OnEmptyResult: onEmpty,
		Engines:       set,
	}
}

// TestTryBrowserFallback_NotConfigured verifies fallback is a no-op when cfg or
// backend is missing or disabled. Each case keeps the original engineResults
// slice unchanged.
func TestTryBrowserFallback_NotConfigured(t *testing.T) {
	apiErr := []*model.EngineResult{{EngineName: "fofa", Error: "boom"}}
	queries := []model.EngineQuery{{EngineName: "fofa", Query: `app="x"`}}

	tests := []struct {
		name    string
		backend *fakeBrowserBackend
		cfg     *BrowserFallbackConfig
	}{
		{"cfg nil", &fakeBrowserBackend{}, nil},
		{"cfg disabled", &fakeBrowserBackend{}, cfg(false, true, true, "fofa")},
		{"backend nil", nil, cfg(true, true, true, "fofa")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newServiceWith(tt.backend, tt.cfg)
			got := svc.tryBrowserFallback(context.Background(), apiErr, queries)
			if len(got) != 1 || got[0].EngineName != "fofa" {
				t.Fatalf("expected unchanged results, got %+v", got)
			}
			if tt.backend != nil && tt.backend.callCount() != 0 {
				t.Fatalf("backend should not be called, got %d calls", tt.backend.callCount())
			}
		})
	}
}

// TestTryBrowserFallback_NotInWhitelist confirms an engine outside the
// whitelist never triggers fallback even if other conditions match.
func TestTryBrowserFallback_NotInWhitelist(t *testing.T) {
	backend := &fakeBrowserBackend{}
	svc := newServiceWith(backend, cfg(true, true, true, "fofa"))

	results := []*model.EngineResult{{EngineName: "hunter", Error: "boom"}}
	queries := []model.EngineQuery{{EngineName: "hunter", Query: "x"}}

	got := svc.tryBrowserFallback(context.Background(), results, queries)
	if len(got) != 1 {
		t.Fatalf("hunter is not whitelisted, expected no fallback append, got %d entries", len(got))
	}
	if backend.callCount() != 0 {
		t.Fatalf("backend should not be called for non-whitelisted engine")
	}
}

// TestTryBrowserFallback_OnAPIError_Triggers verifies the success path: engine
// is whitelisted, API returned an error, OnAPIError=true → fallback collects
// assets and tags them with collection_method=browser_fallback.
func TestTryBrowserFallback_OnAPIError_Triggers(t *testing.T) {
	backend := &fakeBrowserBackend{
		results: []collection.CollectResult{{
			Engine: "fofa",
			Assets: []model.UnifiedAsset{{IP: "1.2.3.4", Port: 80}},
		}},
	}
	svc := newServiceWith(backend, cfg(true, true, false, "fofa"))

	results := []*model.EngineResult{{EngineName: "fofa", Error: "rate-limit"}}
	queries := []model.EngineQuery{{EngineName: "fofa", Query: `app="x"`}}

	got := svc.tryBrowserFallback(context.Background(), results, queries)

	if backend.callCount() != 1 {
		t.Fatalf("expected 1 backend call, got %d", backend.callCount())
	}
	if backend.gotEngine != "fofa" || backend.gotQuery != `app="x"` {
		t.Fatalf("backend got wrong args: engine=%q query=%q", backend.gotEngine, backend.gotQuery)
	}
	if len(got) != 2 {
		t.Fatalf("expected original + fallback result, got %d", len(got))
	}

	// Original API error must be preserved
	if got[0].Error == "" {
		t.Fatal("original API error must not be overwritten")
	}

	fb := got[1]
	if fb.EngineName != "fofa" || fb.Total != 1 || len(fb.NormalizedData) != 1 {
		t.Fatalf("fallback result malformed: %+v", fb)
	}
	if fb.NormalizedData[0].Extra["collection_method"] != "browser_fallback" {
		t.Fatalf("asset must be tagged collection_method=browser_fallback, got %v",
			fb.NormalizedData[0].Extra)
	}
}

// TestTryBrowserFallback_OnAPIError_Disabled verifies that when OnAPIError is
// false, an API error alone does not trigger fallback.
func TestTryBrowserFallback_OnAPIError_Disabled(t *testing.T) {
	backend := &fakeBrowserBackend{}
	svc := newServiceWith(backend, cfg(true, false, false, "fofa"))

	results := []*model.EngineResult{{EngineName: "fofa", Error: "rate-limit"}}
	queries := []model.EngineQuery{{EngineName: "fofa", Query: "x"}}

	got := svc.tryBrowserFallback(context.Background(), results, queries)
	if len(got) != 1 {
		t.Fatalf("expected only the original entry, got %d", len(got))
	}
	if backend.callCount() != 0 {
		t.Fatal("backend should not be invoked when OnAPIError=false")
	}
}

// TestTryBrowserFallback_OnEmptyResult_Triggers verifies that an API call that
// succeeds with zero NormalizedData triggers fallback only when OnEmptyResult
// is true.
func TestTryBrowserFallback_OnEmptyResult_Triggers(t *testing.T) {
	backend := &fakeBrowserBackend{
		results: []collection.CollectResult{{
			Assets: []model.UnifiedAsset{{IP: "9.9.9.9"}},
		}},
	}
	svc := newServiceWith(backend, cfg(true, false, true, "shodan"))

	results := []*model.EngineResult{{EngineName: "shodan"}} // no error, no data
	queries := []model.EngineQuery{{EngineName: "shodan", Query: "x"}}

	got := svc.tryBrowserFallback(context.Background(), results, queries)
	if backend.callCount() != 1 {
		t.Fatalf("expected backend to run once, got %d", backend.callCount())
	}
	if len(got) != 2 || got[1].Total != 1 {
		t.Fatalf("expected fallback to append 1 asset, got %+v", got)
	}
}

// TestTryBrowserFallback_BackendError_PreservesOriginal verifies that when the
// browser backend itself fails, the original API error is preserved and no
// fallback entry is appended.
func TestTryBrowserFallback_BackendError_PreservesOriginal(t *testing.T) {
	backend := &fakeBrowserBackend{err: errors.New("dom parse failed")}
	svc := newServiceWith(backend, cfg(true, true, false, "fofa"))

	originalErr := "API quota exceeded"
	results := []*model.EngineResult{{EngineName: "fofa", Error: originalErr}}
	queries := []model.EngineQuery{{EngineName: "fofa", Query: "x"}}

	got := svc.tryBrowserFallback(context.Background(), results, queries)
	if len(got) != 1 {
		t.Fatalf("backend failure must not append fallback entry, got %d", len(got))
	}
	if got[0].Error != originalErr {
		t.Fatalf("original API error must be preserved, got %q", got[0].Error)
	}
}

// TestTryBrowserFallback_APISuccess_NoFallback verifies the most important
// guarantee: when the API adapter succeeds with data, fallback never runs even
// if OnAPIError and OnEmptyResult are both true.
func TestTryBrowserFallback_APISuccess_NoFallback(t *testing.T) {
	backend := &fakeBrowserBackend{}
	svc := newServiceWith(backend, cfg(true, true, true, "fofa"))

	results := []*model.EngineResult{{
		EngineName:     "fofa",
		NormalizedData: []model.UnifiedAsset{{IP: "1.1.1.1"}},
	}}
	queries := []model.EngineQuery{{EngineName: "fofa", Query: "x"}}

	got := svc.tryBrowserFallback(context.Background(), results, queries)
	if len(got) != 1 {
		t.Fatalf("API success must not trigger fallback, got %d entries", len(got))
	}
	if backend.callCount() != 0 {
		t.Fatal("backend must not be called when API succeeded with data")
	}
}

// TestTryBrowserFallback_MultipleEngines verifies the multi-engine mixed
// scenario: one engine API succeeds, one engine API fails and is in the
// whitelist, one engine API fails but is NOT in the whitelist. Only the
// whitelisted failed engine triggers fallback, and results are correctly
// appended.
func TestTryBrowserFallback_MultipleEngines(t *testing.T) {
	backend := &fakeBrowserBackend{
		results: []collection.CollectResult{{
			Engine: "hunter",
			Assets: []model.UnifiedAsset{{IP: "10.0.0.1", Port: 443}},
		}},
	}
	// Only "hunter" is whitelisted; "fofa" succeeds, "shodan" fails but is not whitelisted.
	svc := newServiceWith(backend, cfg(true, true, false, "hunter"))

	results := []*model.EngineResult{
		{EngineName: "fofa", NormalizedData: []model.UnifiedAsset{{IP: "1.1.1.1", Port: 80}}}, // success
		{EngineName: "hunter", Error: "rate-limit"},                                           // failed, whitelisted
		{EngineName: "shodan", Error: "unauthorized"},                                         // failed, NOT whitelisted
	}
	queries := []model.EngineQuery{
		{EngineName: "fofa", Query: `app="x"`},
		{EngineName: "hunter", Query: `app="y"`},
		{EngineName: "shodan", Query: `app="z"`},
	}

	got := svc.tryBrowserFallback(context.Background(), results, queries)

	// Should have original 3 results + 1 fallback entry for hunter
	if len(got) != 4 {
		t.Fatalf("expected 4 results (3 original + 1 fallback), got %d", len(got))
	}

	// Verify backend was called exactly once (for hunter)
	if backend.callCount() != 1 {
		t.Fatalf("expected 1 backend call for hunter, got %d", backend.callCount())
	}
	if backend.gotEngine != "hunter" {
		t.Fatalf("expected backend called for hunter, got %q", backend.gotEngine)
	}

	// The last entry should be the fallback result for hunter
	fb := got[3]
	if fb.EngineName != "hunter" {
		t.Fatalf("fallback result engine should be hunter, got %q", fb.EngineName)
	}
	if fb.Total != 1 || len(fb.NormalizedData) != 1 {
		t.Fatalf("fallback result should have 1 asset, got total=%d len=%d", fb.Total, len(fb.NormalizedData))
	}
	if fb.NormalizedData[0].IP != "10.0.0.1" {
		t.Fatalf("fallback asset IP mismatch: %q", fb.NormalizedData[0].IP)
	}
	if fb.NormalizedData[0].Extra["collection_method"] != "browser_fallback" {
		t.Fatalf("fallback asset missing collection_method tag, got %v", fb.NormalizedData[0].Extra)
	}
}

// TestTryBrowserFallback_FallbackResultEntersMerger verifies that fallback
// assets tagged with collection_method=browser_fallback survive the
// ResultMerger.Merge round-trip and the tag is preserved in the merged output.
func TestTryBrowserFallback_FallbackResultEntersMerger(t *testing.T) {
	// Build assets as the fallback path would produce them.
	fallbackAsset := model.UnifiedAsset{
		IP:     "192.168.1.1",
		Port:   8080,
		Source: "fofa",
		Extra: map[string]interface{}{
			"collection_method": "browser_fallback",
		},
	}
	apiAsset := model.UnifiedAsset{
		IP:     "192.168.1.2",
		Port:   443,
		Source: "hunter",
		Extra: map[string]interface{}{
			"via": "api",
		},
	}

	merger := unimap.NewResultMerger()
	result := merger.Merge([]model.UnifiedAsset{fallbackAsset, apiAsset})

	if result.Total != 2 {
		t.Fatalf("expected 2 unique assets after merge, got %d", result.Total)
	}

	// Find the fallback asset in the merged output and verify the tag survived.
	found := false
	for _, a := range result.Assets {
		if a.IP == "192.168.1.1" {
			found = true
			if a.Extra == nil {
				t.Fatal("fallback asset Extra is nil after merge")
			}
			if a.Extra["collection_method"] != "browser_fallback" {
				t.Fatalf("collection_method tag lost or changed after merge: got %v", a.Extra["collection_method"])
			}
		}
	}
	if !found {
		t.Fatal("fallback asset not found in merged result")
	}

	// Also verify that merging a duplicate IP:port preserves the fallback tag
	// when the other source does not have it.
	dupAsset := model.UnifiedAsset{
		IP:     "192.168.1.1",
		Port:   8080,
		Source: "hunter",
		Extra:  map[string]interface{}{},
	}
	result2 := merger.Merge([]model.UnifiedAsset{fallbackAsset, dupAsset})
	if result2.Total != 1 {
		t.Fatalf("expected 1 asset after dedup, got %d", result2.Total)
	}
	merged := result2.Assets["192.168.1.1:8080"]
	if merged == nil {
		t.Fatal("merged asset not found by key")
	}
	if merged.Extra["collection_method"] != "browser_fallback" {
		t.Fatalf("collection_method tag dropped during merge: got %v", merged.Extra)
	}
}
