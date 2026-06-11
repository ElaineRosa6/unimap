package service

import (
	"context"
	"errors"
	"testing"

	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/model"
)

type stubBrowserRouter struct {
	openErrByEngine map[string]error
	collectResults  map[string][]collection.CollectResult
	openCalls       int
}

func (r *stubBrowserRouter) OpenSearchEngineResult(_ context.Context, engine, _ string) (string, error) {
	r.openCalls++
	if err := r.openErrByEngine[engine]; err != nil {
		return "", err
	}
	return "https://example.test/search", nil
}

func (r *stubBrowserRouter) CollectSearchEngineResult(_ context.Context, engine, query, _ string) ([]collection.CollectResult, error) {
	if results, ok := r.collectResults[engine]; ok {
		return results, nil
	}
	return []collection.CollectResult{{Engine: engine, Query: query}}, nil
}

type stubCombinedBrowserRouter struct {
	stubBrowserRouter
	combinedCalls int
}

func (r *stubCombinedBrowserRouter) CollectAndCaptureSearchEngineResult(_ context.Context, engine, query, queryID string) ([]collection.CollectResult, string, error) {
	r.combinedCalls++
	return []collection.CollectResult{{Engine: engine, Query: queryID, Assets: []model.UnifiedAsset{{URL: query}}}}, "/tmp/capture.png", nil
}

func TestRunBrowserQueryAsync_ReportsProgressForEachEngine(t *testing.T) {
	svc := NewQueryAppService(nil, nil)
	router := &stubBrowserRouter{
		openErrByEngine: map[string]error{"hunter": errors.New("login required")},
	}

	var calls []struct {
		done   int
		total  int
		engine string
		err    error
	}
	ch := svc.RunBrowserQueryAsync(
		context.Background(),
		"test",
		[]string{"fofa", "hunter"},
		true,
		"collect",
		"q1",
		false,
		nil,
		nil,
		nil,
		router,
		func(done, total int, engine string, err error) {
			calls = append(calls, struct {
				done   int
				total  int
				engine string
				err    error
			}{done: done, total: total, engine: engine, err: err})
		},
	)

	outcome := <-ch
	if len(calls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d", len(calls))
	}
	if calls[0].done != 1 || calls[0].total != 2 || calls[0].engine != "fofa" || calls[0].err != nil {
		t.Fatalf("unexpected first progress call: %+v", calls[0])
	}
	if calls[1].done != 2 || calls[1].total != 2 || calls[1].engine != "hunter" || calls[1].err == nil {
		t.Fatalf("unexpected second progress call: %+v", calls[1])
	}
	if len(outcome.CollectedResults) != 1 || outcome.CollectedResults[0].Engine != "fofa" {
		t.Fatalf("expected collected fofa result, got %#v", outcome.CollectedResults)
	}
	if len(outcome.Errors) != 1 {
		t.Fatalf("expected one browser error, got %#v", outcome.Errors)
	}
}

func TestRunBrowserQueryAsync_CollectsStructuredAssets(t *testing.T) {
	svc := NewQueryAppService(nil, nil)
	router := &stubBrowserRouter{
		collectResults: map[string][]collection.CollectResult{
			"fofa": {{
				Engine: "fofa",
				Query:  "test",
				Assets: []model.UnifiedAsset{{URL: "https://example.test", Title: "Example"}},
				Total:  1,
			}},
		},
	}

	ch := svc.RunBrowserQueryAsync(context.Background(), "test", []string{"fofa"}, true, "collect", "q1", false, nil, nil, nil, router, nil)
	outcome := <-ch

	if len(outcome.OpenedEngines) != 1 || outcome.OpenedEngines[0] != "fofa" {
		t.Fatalf("expected fofa to be opened, got %#v", outcome.OpenedEngines)
	}
	if len(outcome.CollectedResults) != 1 || len(outcome.CollectedResults[0].Assets) != 1 {
		t.Fatalf("expected one collected asset, got %#v", outcome.CollectedResults)
	}
	if outcome.CollectedResults[0].Assets[0].URL != "https://example.test" {
		t.Fatalf("unexpected asset: %#v", outcome.CollectedResults[0].Assets[0])
	}
}

func TestRunBrowserQueryAsync_CollectAndCaptureSkipsPreOpenForCombinedRouter(t *testing.T) {
	svc := NewQueryAppService(nil, nil)
	router := &stubCombinedBrowserRouter{}
	screenshotApp := NewScreenshotAppServiceWithProvider(t.TempDir(), &mockScreenshotProvider{})

	ch := svc.RunBrowserQueryAsync(
		context.Background(), "test", []string{"fofa"}, true, "collect_and_capture", "q1", true,
		screenshotApp, nil, func(path string) string { return "preview:" + path }, router, nil,
	)
	outcome := <-ch

	if router.openCalls != 0 {
		t.Fatalf("expected combined collect+capture to skip pre-open, got %d opens", router.openCalls)
	}
	if router.combinedCalls != 1 {
		t.Fatalf("expected one combined call, got %d", router.combinedCalls)
	}
	if len(outcome.OpenedEngines) != 1 || outcome.OpenedEngines[0] != "fofa" {
		t.Fatalf("expected combined flow to mark fofa opened, got %#v", outcome.OpenedEngines)
	}
	if got := outcome.AutoCapturedPaths["fofa"]; got != "preview:/tmp/capture.png" {
		t.Fatalf("unexpected preview path: %q", got)
	}
}
