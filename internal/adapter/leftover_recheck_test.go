package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// leftover_recheck_test.go drives shipped Search/GetQuota entry points to
// re-validate leftover empty-key and backup-key behavior. These are
// characterization tests of the current workspace, not live engine calls.

func TestLeftover_QuakeSearchEmptyKeyStillIssuesHTTP(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	a := NewQuakeAdapter(server.URL, "", "", 1, time.Second)
	result, err := a.Search(context.Background(), "port:80", 1, 5)
	if err != nil {
		t.Fatalf("Search must return an EngineResult, got err=%v", err)
	}
	if result == nil || result.Error == "" {
		t.Fatal("empty Quake API key must not report a successful search")
	}
	if got := hits.Load(); got == 0 {
		t.Fatal("leftover still open: Quake Search with empty key issued 0 HTTP requests; expected at least one because Search has no fail-fast guard")
	}
}

func TestLeftover_HunterSearchEmptyPrimaryIgnoresBackup(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"total":1,"arr":[]}}`))
	}))
	t.Cleanup(server.Close)

	a := NewHunterAdapter(server.URL, "", "backup-only-key", 1, time.Second)
	result, err := a.Search(context.Background(), "ip=1.1.1.1", 1, 5)
	if err != nil {
		t.Fatalf("Search must return an EngineResult, got err=%v", err)
	}
	if result == nil || !strings.Contains(result.Error, "not configured") {
		t.Fatalf("empty primary Hunter key should short-circuit as not configured, got %#v", result)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("backup-only Hunter Search should not reach HTTP, got %d request(s)", got)
	}
}

func TestLeftover_FofaSearchEmptyPrimaryIgnoresBackup(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":false,"size":0,"results":[]}`))
	}))
	t.Cleanup(server.Close)

	a := NewFofaAdapter(server.URL, "", "user@example.com", "backup-only-key", "backup@example.com", 1, time.Second)
	result, err := a.Search(context.Background(), "port=80", 1, 5)
	if err != nil {
		t.Fatalf("Search must return an EngineResult, got err=%v", err)
	}
	if result == nil || !strings.Contains(result.Error, "not configured") {
		t.Fatalf("empty primary FOFA key should short-circuit as not configured, got %#v", result)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("backup-only FOFA Search should not reach HTTP, got %d request(s)", got)
	}
}

func TestLeftover_QuakeGetQuotaEmptyKeyFailsLocally(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	a := NewQuakeAdapter(server.URL, "", "", 1, time.Second)
	_, err := a.GetQuota()
	if err == nil {
		t.Fatal("empty Quake key GetQuota must fail")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("GetQuota empty-key error = %v, want not configured", err)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("GetQuota empty key must not issue HTTP, got %d", got)
	}
}
