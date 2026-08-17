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

func TestLeftover_QuakeSearchEmptyKeyFailsWithoutHTTP(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	a := NewQuakeAdapter(server.URL, "", "", 1, time.Second)
	start := time.Now()
	result, err := a.Search(context.Background(), "port:80", 1, 5)
	if err != nil {
		t.Fatalf("Search must return an EngineResult, got err=%v", err)
	}
	if result == nil || !strings.Contains(result.Error, "not configured") {
		t.Fatalf("empty Quake API key must fail locally, got %#v", result)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("empty Quake key must not issue HTTP, got %d request(s)", got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("empty Quake key should fail fast, took %v", elapsed)
	}
}

func TestLeftover_HunterSearchBackupOnlyUsesBackupKey(t *testing.T) {
	var hits atomic.Int32
	var sawKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sawKey = r.URL.Query().Get("api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"total":0,"arr":[]}}`))
	}))
	t.Cleanup(server.Close)

	a := NewHunterAdapter(server.URL, "", "backup-only-key", 1, time.Second)
	result, err := a.Search(context.Background(), "ip=1.1.1.1", 1, 5)
	if err != nil {
		t.Fatalf("Search must return an EngineResult, got err=%v", err)
	}
	if result == nil || strings.Contains(result.Error, "not configured") {
		t.Fatalf("backup-only Hunter Search should try the backup key, got %#v", result)
	}
	if got := hits.Load(); got == 0 {
		t.Fatal("backup-only Hunter Search must issue HTTP with the backup key")
	}
	if sawKey != "backup-only-key" {
		t.Fatalf("Hunter backup-only request used key %q, want backup-only-key", sawKey)
	}
}

func TestLeftover_FofaSearchBackupOnlyUsesBackupKey(t *testing.T) {
	var hits atomic.Int32
	var sawKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sawKey = r.URL.Query().Get("key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":false,"size":0,"results":[]}`))
	}))
	t.Cleanup(server.Close)

	a := NewFofaAdapter(server.URL, "", "user@example.com", "backup-only-key", "backup@example.com", 1, time.Second)
	result, err := a.Search(context.Background(), "port=80", 1, 5)
	if err != nil {
		t.Fatalf("Search must return an EngineResult, got err=%v", err)
	}
	if result == nil || strings.Contains(result.Error, "not configured") {
		t.Fatalf("backup-only FOFA Search should try the backup key, got %#v", result)
	}
	if got := hits.Load(); got == 0 {
		t.Fatal("backup-only FOFA Search must issue HTTP with the backup key")
	}
	if sawKey != "backup-only-key" {
		t.Fatalf("FOFA backup-only request used key %q, want backup-only-key", sawKey)
	}
}

func TestLeftover_ShodanSearchBackupOnlyUsesBackupKey(t *testing.T) {
	var hits atomic.Int32
	var sawKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sawKey = r.URL.Query().Get("key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":0,"matches":[]}`))
	}))
	t.Cleanup(server.Close)

	a := NewShodanAdapter(server.URL, "", "backup-only-key", 1, time.Second)
	result, err := a.Search(context.Background(), "port:80", 1, 5)
	if err != nil {
		t.Fatalf("Search must return an EngineResult, got err=%v", err)
	}
	if result == nil || strings.Contains(result.Error, "not configured") {
		t.Fatalf("backup-only Shodan Search should try the backup key, got %#v", result)
	}
	if got := hits.Load(); got == 0 {
		t.Fatal("backup-only Shodan Search must issue HTTP with the backup key")
	}
	if sawKey != "backup-only-key" {
		t.Fatalf("Shodan backup-only request used key %q, want backup-only-key", sawKey)
	}
}

func TestLeftover_QuakeSearchBackupOnlyUsesBackupKey(t *testing.T) {
	var hits atomic.Int32
	var sawKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sawKey = r.Header.Get("X-QuakeToken")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":[],"meta":{"pagination":{"total":0,"count":0}}}`))
	}))
	t.Cleanup(server.Close)

	a := NewQuakeAdapter(server.URL, "", "backup-only-key", 1, time.Second)
	result, err := a.Search(context.Background(), "port:80", 1, 5)
	if err != nil {
		t.Fatalf("Search must return an EngineResult, got err=%v", err)
	}
	if result == nil || strings.Contains(result.Error, "not configured") {
		t.Fatalf("backup-only Quake Search should try the backup key, got %#v", result)
	}
	if got := hits.Load(); got == 0 {
		t.Fatal("backup-only Quake Search must issue HTTP with the backup key")
	}
	if sawKey != "backup-only-key" {
		t.Fatalf("Quake backup-only request used token %q, want backup-only-key", sawKey)
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
