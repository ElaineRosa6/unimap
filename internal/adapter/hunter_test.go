package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestHunterWaitForRate verifies the qps throttle enforces a minimum interval
// between consecutive requests so concurrent queries don't burst Hunter.
func TestHunterWaitForRate(t *testing.T) {
	t.Run("no throttle when qps<=0", func(t *testing.T) {
		h := NewHunterAdapter("http://example.com", "key", "", 0, time.Second)
		start := time.Now()
		for i := 0; i < 5; i++ {
			if err := h.waitForRate(context.Background()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
			t.Errorf("expected no throttle, but waited %v", elapsed)
		}
	})

	t.Run("enforces minimum interval at qps=10", func(t *testing.T) {
		h := NewHunterAdapter("http://example.com", "key", "", 10, time.Second)
		// minInterval = 100ms; 3 sequential calls => ~200ms total (first is free)
		start := time.Now()
		for i := 0; i < 3; i++ {
			if err := h.waitForRate(context.Background()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		elapsed := time.Since(start)
		if elapsed < 180*time.Millisecond {
			t.Errorf("expected >=180ms for 3 calls at qps=10, got %v", elapsed)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		h := NewHunterAdapter("http://example.com", "key", "", 1, time.Second)
		// First call reserves the slot immediately.
		if err := h.waitForRate(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Second call must wait ~1s; cancel quickly and confirm it returns the ctx error.
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		start := time.Now()
		if err := h.waitForRate(ctx); err == nil {
			t.Error("expected context error, got nil")
		}
		if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
			t.Errorf("expected fast cancellation, waited %v", elapsed)
		}
	})
}

// TestHunterSearch_FailoverToBackupOnRateLimit verifies that when the primary key
// is rate-limited (business code 429), Search retries once with the backup key and
// returns its results.
func TestHunterSearch_FailoverToBackupOnRateLimit(t *testing.T) {
	var mu sync.Mutex
	var keysHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keysHit = append(keysHit, r.URL.Query().Get("api-key"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("api-key") {
		case "primary":
			_, _ = w.Write([]byte(`{"code":429,"message":"too many requests","data":{"total":0,"arr":[]}}`))
		case "backup":
			_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":{"total":1,"arr":[{"ip":"9.9.9.9","port":443,"domain":"backup.example","web_title":"backup","status_code":200}]}}`))
		default:
			t.Errorf("unexpected api-key: %q", r.URL.Query().Get("api-key"))
		}
	}))
	defer server.Close()

	h := NewHunterAdapter(server.URL, "primary", "backup", 0, 5*time.Second)
	res, err := h.Search(context.Background(), "port=443", 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res == nil || res.Error != "" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Total != 1 || len(res.RawData) != 1 {
		t.Fatalf("expected 1 result, got total=%d raw=%d", res.Total, len(res.RawData))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keysHit) != 2 || keysHit[0] != "primary" || keysHit[1] != "backup" {
		t.Fatalf("expected primary then backup key, got %v", keysHit)
	}
}

// TestHunterSearch_NoFailoverWhenPrimarySucceeds verifies the backup key is not
// touched when the primary key succeeds.
func TestHunterSearch_NoFailoverWhenPrimarySucceeds(t *testing.T) {
	var mu sync.Mutex
	var keysHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keysHit = append(keysHit, r.URL.Query().Get("api-key"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":{"total":1,"arr":[{"ip":"1.1.1.1","port":80}]}}`))
	}))
	defer server.Close()

	h := NewHunterAdapter(server.URL, "primary", "backup", 0, 5*time.Second)
	res, err := h.Search(context.Background(), "port=80", 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res == nil || res.Error != "" || res.Total != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keysHit) != 1 || keysHit[0] != "primary" {
		t.Fatalf("expected only primary key, got %v", keysHit)
	}
}

// TestHunterSearch_FailoverOnHTTPStatus429 verifies failover also triggers when
// Hunter returns a raw HTTP 429 status rather than a business code 429.
func TestHunterSearch_FailoverOnHTTPStatus429(t *testing.T) {
	var mu sync.Mutex
	var keysHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keysHit = append(keysHit, r.URL.Query().Get("api-key"))
		mu.Unlock()
		switch r.URL.Query().Get("api-key") {
		case "primary":
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		case "backup":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":{"total":1,"arr":[{"ip":"8.8.8.8","port":443}]}}`))
		default:
			t.Errorf("unexpected api-key: %q", r.URL.Query().Get("api-key"))
		}
	}))
	defer server.Close()

	h := NewHunterAdapter(server.URL, "primary", "backup", 0, 5*time.Second)
	res, err := h.Search(context.Background(), "port=443", 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res == nil || res.Error != "" || res.Total != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keysHit) != 2 || keysHit[1] != "backup" {
		t.Fatalf("expected failover to backup, got %v", keysHit)
	}
}

// TestHunterSearch_AllKeysFail verifies the error surfaces when both keys fail.
func TestHunterSearch_AllKeysFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":429,"message":"too many requests","data":{"total":0,"arr":[]}}`))
	}))
	defer server.Close()

	h := NewHunterAdapter(server.URL, "primary", "backup", 0, 5*time.Second)
	res, err := h.Search(context.Background(), "port=443", 1, 10)
	if err != nil {
		t.Fatalf("Search returned unexpected err: %v", err)
	}
	if res == nil || res.Error == "" {
		t.Fatalf("expected error in result, got %+v", res)
	}
}

// TestHunterGetQuota_FailoverToBackup verifies quota lookup also fails over to the
// backup key when the primary key fails authentication.
func TestHunterGetQuota_FailoverToBackup(t *testing.T) {
	var mu sync.Mutex
	var keysHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keysHit = append(keysHit, r.URL.Query().Get("api-key"))
		mu.Unlock()
		switch r.URL.Query().Get("api-key") {
		case "primary":
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case "backup":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":{"rest_free_point":100,"day_free_point":200,"rest_equity_point":0}}`))
		default:
			t.Errorf("unexpected api-key: %q", r.URL.Query().Get("api-key"))
		}
	}))
	defer server.Close()

	h := NewHunterAdapter(server.URL, "primary", "backup", 0, 5*time.Second)
	quota, err := h.GetQuota()
	if err != nil {
		t.Fatalf("GetQuota: %v", err)
	}
	if quota == nil || quota.Remaining != 100 || quota.Total != 200 {
		t.Fatalf("unexpected quota: %+v", quota)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keysHit) != 2 || keysHit[0] != "primary" || keysHit[1] != "backup" {
		t.Fatalf("expected primary then backup key, got %v", keysHit)
	}
}

// TestHunterActiveAPIKeys verifies the key list dedups the backup key.
func TestHunterActiveAPIKeys(t *testing.T) {
	t.Run("dedup when backup equals primary", func(t *testing.T) {
		h := NewHunterAdapter("http://example.com", "key", "key", 0, time.Second)
		if got := h.activeAPIKeys(); len(got) != 1 || got[0] != "key" {
			t.Fatalf("expected single key, got %v", got)
		}
	})
	t.Run("single key when backup empty", func(t *testing.T) {
		h := NewHunterAdapter("http://example.com", "key", "", 0, time.Second)
		if got := h.activeAPIKeys(); len(got) != 1 || got[0] != "key" {
			t.Fatalf("expected single key, got %v", got)
		}
	})
	t.Run("two keys when backup differs", func(t *testing.T) {
		h := NewHunterAdapter("http://example.com", "key1", "key2", 0, time.Second)
		if got := h.activeAPIKeys(); len(got) != 2 || got[0] != "key1" || got[1] != "key2" {
			t.Fatalf("expected [key1 key2], got %v", got)
		}
	})
}
