package adapter

// failover_test.go covers the shared backup-key failover machinery
// (keyfailover.go) plus end-to-end failover for every engine that gained
// second-key support: fofa, quake, zoomeye, shodan, daydaymap, censys.
// hunter has its own failover (see hunter_test.go) and is not duplicated here.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// --- generic helper ---

func TestActiveKeys(t *testing.T) {
	t.Run("backup empty", func(t *testing.T) {
		if got := activeKeys("k1", ""); len(got) != 1 || got[0] != "k1" {
			t.Fatalf("expected [k1], got %v", got)
		}
	})
	t.Run("backup equals primary", func(t *testing.T) {
		if got := activeKeys("k1", "k1"); len(got) != 1 || got[0] != "k1" {
			t.Fatalf("expected [k1], got %v", got)
		}
	})
	t.Run("backup differs", func(t *testing.T) {
		if got := activeKeys("k1", "k2"); len(got) != 2 || got[0] != "k1" || got[1] != "k2" {
			t.Fatalf("expected [k1 k2], got %v", got)
		}
	})
	t.Run("empty primary uses backup only", func(t *testing.T) {
		if got := activeKeys("", "k2"); len(got) != 1 || got[0] != "k2" {
			t.Fatalf("expected [k2], got %v", got)
		}
	})
	t.Run("both empty", func(t *testing.T) {
		if got := activeKeys("", ""); len(got) != 0 {
			t.Fatalf("expected empty, got %v", got)
		}
	})
}

func TestWithKeyFailover(t *testing.T) {
	t.Run("no failover when primary succeeds", func(t *testing.T) {
		var calls []int
		if err := withKeyFailover("test", 2, func(idx int) error {
			calls = append(calls, idx)
			return nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(calls) != 1 || calls[0] != 0 {
			t.Fatalf("expected only primary call, got %v", calls)
		}
	})
	t.Run("failover on key error", func(t *testing.T) {
		var calls []int
		if err := withKeyFailover("test", 2, func(idx int) error {
			calls = append(calls, idx)
			if idx == 0 {
				return &keyError{engine: "test", code: 401, msg: "unauthorized"}
			}
			return nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(calls) != 2 || calls[0] != 0 || calls[1] != 1 {
			t.Fatalf("expected [0 1], got %v", calls)
		}
	})
	t.Run("non-key error does not failover", func(t *testing.T) {
		var calls []int
		boom := errors.New("network down")
		err := withKeyFailover("test", 2, func(idx int) error {
			calls = append(calls, idx)
			return boom
		})
		if !errors.Is(err, boom) {
			t.Fatalf("expected original error, got %v", err)
		}
		if len(calls) != 1 {
			t.Fatalf("expected only primary call, got %v", calls)
		}
	})
	t.Run("all keys fail", func(t *testing.T) {
		var calls []int
		if err := withKeyFailover("test", 2, func(idx int) error {
			calls = append(calls, idx)
			return &keyError{engine: "test", code: 429, msg: "rate limited"}
		}); err == nil {
			t.Fatal("expected error")
		}
		if len(calls) != 2 {
			t.Fatalf("expected both keys tried, got %v", calls)
		}
	})
}

// --- per-engine key-error classifiers ---

func TestClassifyFailoverStatus(t *testing.T) {
	// (engine, classify, code, body) -> key error expected
	cases := []struct {
		name string
		fn   func() *keyError
		want bool
	}{
		{"fofa 401", func() *keyError { return classifyFofaStatus(401, "") }, true},
		{"fofa 403", func() *keyError { return classifyFofaStatus(403, "") }, true},
		{"fofa 820041", func() *keyError { return classifyFofaStatus(200, `820041 邮箱或key错误`) }, true},
		{"fofa 820003 rate limit", func() *keyError { return classifyFofaStatus(200, `820003 访问过于频繁`) }, true},
		{"fofa 820011 expired", func() *keyError { return classifyFofaStatus(200, `820011 会员到期`) }, true},
		{"fofa 820031 syntax", func() *keyError { return classifyFofaStatus(200, `820031 语法错误`) }, false},
		{"fofa generic", func() *keyError { return classifyFofaStatus(500, `internal error`) }, false},

		{"quake 401", func() *keyError { return classifyQuakeStatus(401, "") }, true},
		{"quake 429", func() *keyError { return classifyQuakeStatus(429, "") }, true},
		{"quake q2001 credits", func() *keyError { return classifyQuakeBusinessError("q2001", "积分不足") }, true},
		{"quake 积分不足", func() *keyError { return classifyQuakeBusinessError("", "您的积分不足") }, true},
		{"quake generic", func() *keyError { return classifyQuakeStatus(500, `internal`) }, false},

		{"zoomeye 401", func() *keyError { return classifyZoomEyeStatus(401, "") }, true},
		{"zoomeye 402", func() *keyError { return classifyZoomEyeStatus(402, "") }, true},
		{"zoomeye 429", func() *keyError { return classifyZoomEyeStatus(429, "") }, true},
		{"zoomeye generic", func() *keyError { return classifyZoomEyeStatus(500, `internal`) }, false},

		{"shodan 401", func() *keyError { return classifyShodanStatus(401, "") }, true},
		{"shodan 402", func() *keyError { return classifyShodanStatus(402, "") }, true},
		{"shodan 429", func() *keyError { return classifyShodanStatus(429, "") }, true},
		{"shodan not authorized", func() *keyError { return classifyShodanStatus(200, `not authorized`) }, true},
		{"shodan invalid api key", func() *keyError { return classifyShodanStatus(200, `invalid api key`) }, true},
		{"shodan generic", func() *keyError { return classifyShodanStatus(500, `boom`) }, false},

		{"daydaymap 401", func() *keyError { return classifyDayDayMapStatus(401, "") }, true},
		{"daydaymap 429", func() *keyError { return classifyDayDayMapStatus(429, "") }, true},
		{"daydaymap 积分不足", func() *keyError { return classifyDayDayMapStatus(200, `积分不足`) }, true},
		{"daydaymap 余额", func() *keyError { return classifyDayDayMapStatus(200, `余额不足`) }, true},
		{"daydaymap generic", func() *keyError { return classifyDayDayMapStatus(500, `boom`) }, false},

		{"censys 401", func() *keyError { return classifyCensysStatus(401, "") }, true},
		{"censys 429", func() *keyError { return classifyCensysStatus(429, "") }, true},
		{"censys quota exceeded", func() *keyError { return classifyCensysStatus(200, `{"error":"quota exceeded"}`) }, true},
		{"censys generic", func() *keyError { return classifyCensysStatus(500, `boom`) }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn()
			if tc.want && got == nil {
				t.Errorf("%s: expected key error, got nil", tc.name)
			}
			if !tc.want && got != nil {
				t.Errorf("%s: expected nil, got %v", tc.name, got)
			}
		})
	}
}

// --- end-to-end failover per engine ---

func TestFofaSearch_FailoverToBackup(t *testing.T) {
	var mu sync.Mutex
	var credsHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		credsHit = append(credsHit, r.URL.Query().Get("email")+"/"+r.URL.Query().Get("key"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("key") {
		case "primary":
			_, _ = w.Write([]byte(`{"error":true,"errmsg":"820041 邮箱或key错误"}`))
		case "backup":
			_, _ = w.Write([]byte(`{"error":false,"results":[["9.9.9.9","443","http"]],"total":1}`))
		default:
			t.Errorf("unexpected key: %q", r.URL.Query().Get("key"))
		}
	}))
	defer server.Close()

	a := NewFofaAdapter(server.URL, "primary", "primary-email", "backup", "backup-email", 0, 5*time.Second)
	res, err := a.Search(context.Background(), `title="t"`, 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res == nil || res.Error != "" || res.Total != 1 || len(res.RawData) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"primary-email/primary", "backup-email/backup"}
	if len(credsHit) != 2 || credsHit[0] != want[0] || credsHit[1] != want[1] {
		t.Fatalf("expected %v, got %v", want, credsHit)
	}
}

func TestQuakeSearch_FailoverToBackup(t *testing.T) {
	var mu sync.Mutex
	var keysHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keysHit = append(keysHit, r.Header.Get("X-QuakeToken"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("X-QuakeToken") {
		case "primary":
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		case "backup":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":[{"ip":"9.9.9.9","port":443}],"meta":{"pagination":{"total":1}}}`))
		default:
			t.Errorf("unexpected token: %q", r.Header.Get("X-QuakeToken"))
		}
	}))
	defer server.Close()

	a := NewQuakeAdapter(server.URL, "primary", "backup", 0, 5*time.Second)
	res, err := a.Search(context.Background(), `port=443`, 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res == nil || res.Error != "" || res.Total != 1 || len(res.RawData) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keysHit) != 2 || keysHit[0] != "primary" || keysHit[1] != "backup" {
		t.Fatalf("expected primary then backup token, got %v", keysHit)
	}
}

func TestZoomEyeSearch_FailoverToBackup(t *testing.T) {
	var mu sync.Mutex
	var keysHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keysHit = append(keysHit, r.Header.Get("API-KEY"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("API-KEY") {
		case "primary":
			_, _ = w.Write([]byte(`{"code":50000,"error":"credits_insufficient","message":"积分不足"}`))
		case "backup":
			_, _ = w.Write([]byte(`{"code":60000,"data":[{"ip":"9.9.9.9"}],"total":1}`))
		default:
			t.Errorf("unexpected key: %q", r.Header.Get("API-KEY"))
		}
	}))
	defer server.Close()

	a := NewZoomEyeAdapter(server.URL, "primary", "backup", 0, 5*time.Second)
	res, err := a.Search(context.Background(), `port=443`, 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res == nil || res.Error != "" || res.Total != 1 || len(res.RawData) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keysHit) != 2 || keysHit[0] != "primary" || keysHit[1] != "backup" {
		t.Fatalf("expected primary then backup key, got %v", keysHit)
	}
}

func TestShodanSearch_FailoverToBackup(t *testing.T) {
	var mu sync.Mutex
	var keysHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keysHit = append(keysHit, r.URL.Query().Get("key"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("key") {
		case "primary":
			http.Error(w, "Payment Required", http.StatusPaymentRequired)
		case "backup":
			_, _ = w.Write([]byte(`{"matches":[{"ip_str":"9.9.9.9","port":443}],"total":1}`))
		default:
			t.Errorf("unexpected key: %q", r.URL.Query().Get("key"))
		}
	}))
	defer server.Close()

	a := NewShodanAdapter(server.URL, "primary", "backup", 0, 5*time.Second)
	res, err := a.Search(context.Background(), `port=443`, 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res == nil || res.Error != "" || res.Total != 1 || len(res.RawData) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keysHit) != 2 || keysHit[0] != "primary" || keysHit[1] != "backup" {
		t.Fatalf("expected primary then backup key, got %v", keysHit)
	}
}

func TestDayDayMapSearch_FailoverToBackup(t *testing.T) {
	var mu sync.Mutex
	var keysHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keysHit = append(keysHit, r.Header.Get("api-key"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("api-key") {
		case "primary":
			_, _ = w.Write([]byte(`{"code":500,"msg":"积分不足"}`))
		case "backup":
			_, _ = w.Write([]byte(`{"code":200,"msg":"检索成功","data":{"list":[{"ip":"9.9.9.9","port":443}],"total":1}}`))
		default:
			t.Errorf("unexpected key: %q", r.Header.Get("api-key"))
		}
	}))
	defer server.Close()

	a := NewDayDayMapAdapter(server.URL, "primary", "backup", 0, 5*time.Second)
	res, err := a.Search(context.Background(), `port=443`, 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res == nil || res.Error != "" || res.Total != 1 || len(res.RawData) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keysHit) != 2 || keysHit[0] != "primary" || keysHit[1] != "backup" {
		t.Fatalf("expected primary then backup key, got %v", keysHit)
	}
}

func TestCensysSearch_FailoverToBackup(t *testing.T) {
	var mu sync.Mutex
	var credsHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		mu.Lock()
		credsHit = append(credsHit, user+"/"+pass)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch user {
		case "primary":
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case "backup":
			_, _ = w.Write([]byte(`{"result":{"resource":{"ip":"9.9.9.9","services":[{"port":443,"service_name":"http"}]}}}`))
		default:
			t.Errorf("unexpected user: %q", user)
		}
	}))
	defer server.Close()

	a := NewCensysAdapter(server.URL, "primary", "secret1", "backup", "secret2", 0, 5*time.Second)
	res, err := a.Search(context.Background(), "9.9.9.9", 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res == nil || res.Error != "" || res.Total != 1 || len(res.RawData) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"primary/secret1", "backup/secret2"}
	if len(credsHit) != 2 || credsHit[0] != want[0] || credsHit[1] != want[1] {
		t.Fatalf("expected %v, got %v", want, credsHit)
	}
}

// TestNoFailoverWhenPrimarySucceeds is a representative check that the backup
// credential is untouched when the primary succeeds (generic logic in
// withKeyFailover; exercised through one engine's full Search path).
func TestShodanSearch_NoFailoverWhenPrimarySucceeds(t *testing.T) {
	var mu sync.Mutex
	var keysHit []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keysHit = append(keysHit, r.URL.Query().Get("key"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"matches":[{"ip_str":"1.1.1.1","port":80}],"total":1}`))
	}))
	defer server.Close()

	a := NewShodanAdapter(server.URL, "primary", "backup", 0, 5*time.Second)
	res, err := a.Search(context.Background(), `port=80`, 1, 10)
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
