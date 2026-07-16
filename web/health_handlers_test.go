package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/history"
)

func TestHandleHealthReady_RequiredDependenciesMissing(t *testing.T) {
	cfg := &config.Config{}
	cfg.Scheduler.Enabled = true
	cfg.Engines.Fofa.Enabled = true
	s := &Server{config: cfg, orchestrator: adapter.NewEngineOrchestrator()}
	rec := httptest.NewRecorder()
	s.handleHealthReady(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleHealthReady_ReportsMissingEnabledEngine(t *testing.T) {
	cfg := &config.Config{}
	cfg.Engines.Fofa.Enabled = true
	cfg.Engines.Hunter.Enabled = true
	orch := adapter.NewEngineOrchestrator()
	orch.RegisterAdapter(adapter.NewFofaAdapterWebOnly())
	s := &Server{config: cfg, orchestrator: orch}

	rec := httptest.NewRecorder()
	s.handleHealthReady(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when Hunter is enabled but missing: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Checks map[string]string `json:"checks"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Checks["engines"], "hunter") {
		t.Fatalf("engine readiness did not identify Hunter: %#v", resp.Checks)
	}
}

func TestHandleHealthReady_ConcurrentConfigReplacement(t *testing.T) {
	s := &Server{config: &config.Config{}, orchestrator: adapter.NewEngineOrchestrator()}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			s.configMutex.Lock()
			next := s.config.Clone()
			next.Scheduler.Enabled = i%2 == 0
			s.config = next
			s.configMutex.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			rec := httptest.NewRecorder()
			s.handleHealthReady(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
			if rec.Code != http.StatusOK && rec.Code != http.StatusServiceUnavailable {
				t.Errorf("unexpected readiness status %d", rec.Code)
				return
			}
		}
	}()
	wg.Wait()
}

func TestHandleHealthReady_DoesNotExposeDatabaseErrors(t *testing.T) {
	db, err := history.NewDatabase(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.History.Enabled = true
	s := &Server{config: cfg, historyDB: db, orchestrator: adapter.NewEngineOrchestrator()}
	rec := httptest.NewRecorder()
	s.handleHealthReady(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", rec.Code, rec.Body.String())
	}
	body := strings.ToLower(rec.Body.String())
	if strings.Contains(body, "database is closed") || strings.Contains(body, "sql:") {
		t.Fatalf("readiness exposed a database error: %s", rec.Body.String())
	}
}

func TestHandleHealthReady_OK(t *testing.T) {
	orch := adapter.NewEngineOrchestrator()
	s := &Server{
		orchestrator: orch,
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	s.handleHealthReady(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", resp["status"])
	}
	if _, ok := resp["checks"]; !ok {
		t.Fatal("expected checks in response")
	}
}

func TestHandleHealthReady_NoOrchestrator(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	s.handleHealthReady(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	checks := resp["checks"].(map[string]interface{})
	if checks["engines"] != "not initialized" {
		t.Fatalf("expected 'not initialized', got %v", checks["engines"])
	}
}

func TestHandleHealthLive_OK(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	s.handleHealthLive(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", resp["status"])
	}
}

func TestLivenessCheck(t *testing.T) {
	// 正常状态应返回 true
	if !livenessCheck(context.Background()) {
		t.Fatal("expected liveness check to return true")
	}
}
