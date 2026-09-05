package web

import (
	"context"
	"testing"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/service"
)

type browserFirstProvider struct {
	successfulScreenshotProvider
	done chan struct{}
}

func (p *browserFirstProvider) CollectSearchEngineResult(context.Context, string, string, string) ([]collection.CollectResult, error) {
	defer close(p.done)
	return []collection.CollectResult{{Engine: "fofa", Assets: []model.UnifiedAsset{{IP: "203.0.113.10", Port: 443}}, Total: 1}}, nil
}

type delayedEmptyAPI struct {
	adapter.EngineAdapter
	browserDone <-chan struct{}
}

func (*delayedEmptyAPI) IsWebOnly() bool { return false }
func (a *delayedEmptyAPI) Search(ctx context.Context, _ string, _, _ int) (*model.EngineResult, error) {
	select {
	case <-a.browserDone:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// The browser result channel is delivered and closed while API is pending.
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &model.EngineResult{EngineName: "fofa"}, nil
}
func (*delayedEmptyAPI) Normalize(*model.EngineResult) ([]model.UnifiedAsset, error) {
	return []model.UnifiedAsset{}, nil
}

func TestWSBrowserResultSurvivesClosedChannel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p := &browserFirstProvider{done: make(chan struct{})}
	u := service.NewUnifiedService()
	defer u.Shutdown()
	u.RegisterAdapter(&delayedEmptyAPI{EngineAdapter: adapter.NewFofaAdapterWebOnly(), browserDone: p.done})
	s := &Server{
		queryApp:         service.NewQueryAppService(u, u.GetOrchestrator()),
		screenshotRouter: screenshot.NewScreenshotRouter(screenshot.RouterConfig{Priority: screenshot.ModeCDP}, p, nil, nil),
		queryStatus:      map[string]*QueryStatus{"fixture": {ID: "fixture", Status: "running"}},
		shutdownCtx:      ctx, connManager: &ConnectionManager{connections: make(map[string]*managedConn)},
	}
	var complete map[string]interface{}
	s.executeWSQueryAsync(ctx, "fixture-conn", "fixture", `port="443"`, []string{"fofa"}, []string{"fofa"}, 50, true, "collect", func(v interface{}) error { complete = v.(map[string]interface{}); return nil })
	if complete == nil {
		t.Fatal("missing completion")
	}
	result, ok := complete["results"].(QueryAPIPayload)
	if !ok || len(result.Assets) != 1 || result.Assets[0].IP != "203.0.113.10" {
		t.Fatalf("browser-first result lost: %#v", complete["results"])
	}
	if len(result.BrowserCollectedData) != 1 {
		t.Fatalf("browser data overwritten: %#v", result.BrowserCollectedData)
	}
}
