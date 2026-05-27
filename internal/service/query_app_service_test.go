package service

import (
	"context"
	"errors"
	"testing"

	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/screenshot"
)

type stubBrowserRouter struct {
	openErrByEngine map[string]error
	collectResults  map[string][]screenshot.CollectResult
}

func (r *stubBrowserRouter) OpenSearchEngineResult(_ context.Context, engine, _ string) (string, error) {
	if err := r.openErrByEngine[engine]; err != nil {
		return "", err
	}
	return "https://example.test/search", nil
}

func (r *stubBrowserRouter) CollectSearchEngineResult(_ context.Context, engine, query, _ string) ([]screenshot.CollectResult, error) {
	if results, ok := r.collectResults[engine]; ok {
		return results, nil
	}
	return []screenshot.CollectResult{{Engine: engine, Query: query}}, nil
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
		collectResults: map[string][]screenshot.CollectResult{
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
