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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web&search=example.com", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=bogus&search=example.com", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web&search=", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web&search="+long, nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web&search=example.com", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web&search=example.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Total   int  `json:"total"`
		Groups  []struct {
			Type    string `json:"type"`
			Label   string `json:"label"`
			Total   int    `json:"total"`
			Results []struct {
				Domain string `json:"domain"`
			} `json:"results"`
		} `json:"groups"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || resp.Total != 1 || len(resp.Groups) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	g := resp.Groups[0]
	if g.Type != "web" || g.Total != 1 || len(g.Results) != 1 || g.Results[0].Domain != "example.com" {
		t.Fatalf("unexpected group: %+v", g)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?search=example.com", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/health", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/health", nil)
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/icp/query", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d (body=%q)", w.Code, w.Body.String())
	}
}

func TestHandleICPQuery_MultiType(t *testing.T) {
	// Mock sidecar that responds to both /query/web and /query/app
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch {
		case strings.HasPrefix(r.URL.Path, "/query/web"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200, "msg": "ok",
				"params": map[string]interface{}{
					"total": 2,
					"list": []map[string]interface{}{
						{"domain": "web1.com", "licence": "京ICP证001"},
						{"domain": "web2.com", "licence": "京ICP证002"},
					},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/query/app"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200, "msg": "ok",
				"params": map[string]interface{}{
					"total": 1,
					"list": []map[string]interface{}{
						{"domain": "app1.com", "licence": "京ICP证003"},
					},
				},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer mock.Close()

	s := newServerWithICP(true, mock.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web,app&search=example.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Total   int  `json:"total"`
		Groups  []struct {
			Type    string `json:"type"`
			Total   int    `json:"total"`
			Results []struct {
				Domain string `json:"domain"`
			} `json:"results"`
		} `json:"groups"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
	if resp.Total != 3 {
		t.Fatalf("expected total=3, got %d", resp.Total)
	}
	if len(resp.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(resp.Groups))
	}
	if resp.Groups[0].Type != "web" || resp.Groups[0].Total != 2 {
		t.Fatalf("unexpected web group: %+v", resp.Groups[0])
	}
	if resp.Groups[1].Type != "app" || resp.Groups[1].Total != 1 {
		t.Fatalf("unexpected app group: %+v", resp.Groups[1])
	}
}

func TestHandleICPQuery_InvalidTypeInCommaList(t *testing.T) {
	s := newServerWithICP(true, "http://localhost:16181")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web,bogus&search=example.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_type") {
		t.Fatalf("expected invalid_type in body, got %q", w.Body.String())
	}
}

func TestHandleICPQuery_MultiTypePartialFailure(t *testing.T) {
	// Mock sidecar: web succeeds, app returns error
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/query/web"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200, "msg": "ok",
				"params": map[string]interface{}{"total": 1, "list": []map[string]interface{}{{"domain": "ok.com"}}},
			})
		default:
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("sidecar error"))
		}
	}))
	defer mock.Close()

	s := newServerWithICP(true, mock.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web,app&search=example.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	// 整体仍返回 200（部分成功），web 组有结果，app 组有 error
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Total   int  `json:"total"`
		Groups  []struct {
			Type    string `json:"type"`
			Total   int    `json:"total"`
			Error   string `json:"error"`
			Results []interface{} `json:"results"`
		} `json:"groups"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true (partial), got %+v", resp)
	}
	if resp.Total != 1 {
		t.Fatalf("expected total=1 (only web succeeded), got %d", resp.Total)
	}
	if resp.Groups[0].Error != "" {
		t.Fatalf("web group should have no error, got %q", resp.Groups[0].Error)
	}
	if resp.Groups[1].Error == "" {
		t.Fatalf("app group should have error")
	}
}
