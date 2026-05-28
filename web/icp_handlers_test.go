package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/unimap/project/internal/config"
)

func newServerWithICP(enabled bool, baseURL string) *Server {
	cfg := &config.Config{}
	cfg.ICP.Enabled = enabled
	cfg.ICP.BaseURL = baseURL
	cfg.ICP.DefaultType = "web"
	cfg.ICP.Timeout = 30
	return &Server{config: cfg}
}

func TestHandleICPQuery_Disabled(t *testing.T) {
	s := newServerWithICP(false, "http://localhost:16181")
	req := httptest.NewRequest(http.MethodGet, "/api/icp/query?type=web&search=example.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body=%q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "icp_disabled") {
		t.Fatalf("expected icp_disabled in body, got %q", w.Body.String())
	}
}

func TestHandleICPQuery_InvalidType(t *testing.T) {
	s := newServerWithICP(true, "http://localhost:16181")
	req := httptest.NewRequest(http.MethodGet, "/api/icp/query?type=bogus&search=example.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_type") {
		t.Fatalf("expected invalid_type in body, got %q", w.Body.String())
	}
}

func TestHandleICPQuery_MissingSearch(t *testing.T) {
	s := newServerWithICP(true, "http://localhost:16181")
	req := httptest.NewRequest(http.MethodGet, "/api/icp/query?type=web&search=", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "missing_search") {
		t.Fatalf("expected missing_search in body, got %q", w.Body.String())
	}
}

func TestHandleICPQuery_SearchTooLong(t *testing.T) {
	s := newServerWithICP(true, "http://localhost:16181")
	long := strings.Repeat("a", 300)
	req := httptest.NewRequest(http.MethodGet, "/api/icp/query?type=web&search="+long, nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "search_too_long") {
		t.Fatalf("expected search_too_long in body, got %q", w.Body.String())
	}
}

func TestHandleICPQuery_BaseURLEmpty(t *testing.T) {
	s := newServerWithICP(true, "")
	req := httptest.NewRequest(http.MethodGet, "/api/icp/query?type=web&search=example.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body=%q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "icp_not_configured") {
		t.Fatalf("expected icp_not_configured in body, got %q", w.Body.String())
	}
}

func TestHandleICPQuery_SuccessThroughMockBackend(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/query/web") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Mirror the icpAPIResponse shape used by adapter.ICPSearch.
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"msg":  "ok",
			"params": map[string]interface{}{
				"total": 1,
				"list": []map[string]interface{}{
					{
						"domain":   "example.com",
						"licence":  "京ICP证000001号",
						"unitName": "Example Co",
						"cityName": "北京",
					},
				},
			},
		})
	}))
	defer mock.Close()

	s := newServerWithICP(true, mock.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/icp/query?type=web&search=example.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Total   int  `json:"total"`
		Results []struct {
			Domain string `json:"domain"`
		} `json:"results"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || resp.Total != 1 || len(resp.Results) != 1 || resp.Results[0].Domain != "example.com" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestHandleICPQuery_DefaultTypeFallback(t *testing.T) {
	called := false
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if !strings.HasPrefix(r.URL.Path, "/query/web") {
			t.Errorf("expected /query/web (from default_type), got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200, "msg": "ok",
			"params": map[string]interface{}{"total": 0, "list": []interface{}{}},
		})
	}))
	defer mock.Close()

	s := newServerWithICP(true, mock.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/icp/query?search=example.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)
	if !called {
		t.Fatalf("mock backend was not called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleICPHealth_Success(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>ICP备案批量查询系统</body></html>"))
	}))
	defer mock.Close()

	s := newServerWithICP(true, mock.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/icp/health", nil)
	w := httptest.NewRecorder()
	s.handleICPHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool   `json:"success"`
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got body=%q", w.Body.String())
	}
	if resp.BaseURL != mock.URL {
		t.Fatalf("expected base_url=%q, got %q", mock.URL, resp.BaseURL)
	}
}

func TestHandleICPHealth_SidecarDown(t *testing.T) {
	s := newServerWithICP(true, "http://localhost:19999")
	req := httptest.NewRequest(http.MethodGet, "/api/icp/health", nil)
	w := httptest.NewRecorder()
	s.handleICPHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ICP health check failed") {
		t.Fatalf("expected health check failure, got %q", w.Body.String())
	}
}

func TestHandleICPQuery_RejectsNonGET(t *testing.T) {
	s := newServerWithICP(true, "http://localhost:16181")
	req := httptest.NewRequest(http.MethodPost, "/api/icp/query", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d (body=%q)", w.Code, w.Body.String())
	}
}
