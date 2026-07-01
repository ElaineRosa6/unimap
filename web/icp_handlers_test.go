package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/config"
	icpdb "github.com/unimap/project/internal/icp/database"
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
			Type    string        `json:"type"`
			Total   int           `json:"total"`
			Error   string        `json:"error"`
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

// mockICPRepo records SaveRun/SaveResults calls for assertion.
type mockICPRepo struct {
	mu          sync.Mutex
	savedRuns   []*icpdb.ICPQueryRun
	savedResult map[int64][]adapter.ICPResult
	nextID      int64
}

func newMockICPRepo() *mockICPRepo {
	return &mockICPRepo{savedResult: make(map[int64][]adapter.ICPResult)}
}

func (m *mockICPRepo) SaveRun(run *icpdb.ICPQueryRun) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	run.ID = m.nextID
	m.savedRuns = append(m.savedRuns, run)
	return run.ID, nil
}

func (m *mockICPRepo) SaveResults(runID int64, results []adapter.ICPResult, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.savedResult[runID] = results
	return nil
}

func (m *mockICPRepo) GetRunsByTaskID(string, int) ([]*icpdb.ICPQueryRun, error) { return nil, nil }
func (m *mockICPRepo) GetRunsByKeyword(string, string, int) ([]*icpdb.ICPQueryRun, error) {
	return nil, nil
}
func (m *mockICPRepo) GetResultsByRunID(int64) ([]*icpdb.ICPResultRow, error) { return nil, nil }
func (m *mockICPRepo) GetLatestResults(string, string) ([]*icpdb.ICPResultRow, error) {
	return nil, nil
}
func (m *mockICPRepo) GetPreviousResults(string, string, time.Time) ([]*icpdb.ICPResultRow, error) {
	return nil, nil
}
func (m *mockICPRepo) CleanupOldRuns(time.Time) (int64, error) { return 0, nil }

func TestHandleICPQuery_PersistsToRepo(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"msg":  "ok",
			"params": map[string]interface{}{
				"total": 1,
				"list": []map[string]interface{}{
					{"domain": "example.com", "licence": "京ICP证000001号", "unitName": "Example Co"},
				},
			},
		})
	}))
	defer mock.Close()

	repo := newMockICPRepo()
	s := newServerWithICP(true, mock.URL)
	s.icpRepo = repo

	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web&search=example.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.savedRuns) != 1 {
		t.Fatalf("expected 1 SaveRun call, got %d", len(repo.savedRuns))
	}
	run := repo.savedRuns[0]
	if run.TaskID != "manual" {
		t.Fatalf("expected TaskID=manual, got %q", run.TaskID)
	}
	if run.QueryKeyword != "example.com" {
		t.Fatalf("expected QueryKeyword=example.com, got %q", run.QueryKeyword)
	}
	if run.QueryType != "web" {
		t.Fatalf("expected QueryType=web, got %q", run.QueryType)
	}
	if run.TotalRecords != 1 || run.ResultCount != 1 {
		t.Fatalf("expected total=1 count=1, got total=%d count=%d", run.TotalRecords, run.ResultCount)
	}
	if len(repo.savedResult[run.ID]) != 1 {
		t.Fatalf("expected 1 result saved for run %d, got %d", run.ID, len(repo.savedResult[run.ID]))
	}
	if repo.savedResult[run.ID][0].Domain != "example.com" {
		t.Fatalf("expected saved domain=example.com, got %q", repo.savedResult[run.ID][0].Domain)
	}
}

func TestHandleICPQuery_MultiTypePersistsPerType(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch {
		case strings.HasPrefix(r.URL.Path, "/query/web"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200, "msg": "ok",
				"params": map[string]interface{}{"total": 1, "list": []map[string]interface{}{{"domain": "web.com"}}},
			})
		case strings.HasPrefix(r.URL.Path, "/query/app"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200, "msg": "ok",
				"params": map[string]interface{}{"total": 1, "list": []map[string]interface{}{{"domain": "app.com"}}},
			})
		}
	}))
	defer mock.Close()

	repo := newMockICPRepo()
	s := newServerWithICP(true, mock.URL)
	s.icpRepo = repo

	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web,app&search=test.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.savedRuns) != 2 {
		t.Fatalf("expected 2 SaveRun calls (one per type), got %d", len(repo.savedRuns))
	}
	types := map[string]bool{}
	for _, run := range repo.savedRuns {
		types[run.QueryType] = true
	}
	if !types["web"] || !types["app"] {
		t.Fatalf("expected both web and app runs saved, got %v", types)
	}
}

func TestHandleICPQuery_SkipsErrorGroups(t *testing.T) {
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

	repo := newMockICPRepo()
	s := newServerWithICP(true, mock.URL)
	s.icpRepo = repo

	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/query?type=web,app&search=test.com", nil)
	w := httptest.NewRecorder()
	s.handleICPQuery(w, req)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.savedRuns) != 1 {
		t.Fatalf("expected 1 SaveRun (only successful web group), got %d", len(repo.savedRuns))
	}
	if repo.savedRuns[0].QueryType != "web" {
		t.Fatalf("expected only web run saved, got %q", repo.savedRuns[0].QueryType)
	}
}

// ============================================================
// compareICPResults tests
// ============================================================

func TestCompareICPResults_EmptyPrevious(t *testing.T) {
	latest := []*icpdb.ICPResultRow{{Domain: "a.com", Licence: "ICP1"}}
	changes := compareICPResults(latest, nil)
	if changes != nil {
		t.Fatalf("expected nil changes for empty previous, got %v", changes)
	}
}

func TestCompareICPResults_NewDomain(t *testing.T) {
	latest := []*icpdb.ICPResultRow{{Domain: "new.com", Licence: "ICP1"}}
	previous := []*icpdb.ICPResultRow{{Domain: "old.com", Licence: "ICP1"}}
	changes := compareICPResults(latest, previous)
	// new.com is new (+1), old.com is removed (+1) = 2 changes
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes (new + removed), got %d", len(changes))
	}
	found := false
	for _, c := range changes {
		if c.Field == "_new" && c.Domain == "new.com" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected _new change for new.com")
	}
}

func TestCompareICPResults_FieldChange(t *testing.T) {
	latest := []*icpdb.ICPResultRow{{Domain: "a.com", Licence: "new-licence", UnitName: "New Co"}}
	previous := []*icpdb.ICPResultRow{{Domain: "a.com", Licence: "old-licence", UnitName: "New Co"}}
	changes := compareICPResults(latest, previous)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Field != "licence" {
		t.Fatalf("expected field licence, got %s", changes[0].Field)
	}
	if changes[0].Old != "old-licence" {
		t.Fatalf("expected old value old-licence, got %s", changes[0].Old)
	}
	if changes[0].New != "new-licence" {
		t.Fatalf("expected new value new-licence, got %s", changes[0].New)
	}
}

func TestCompareICPResults_NoChanges(t *testing.T) {
	latest := []*icpdb.ICPResultRow{{Domain: "a.com", Licence: "ICP1"}}
	previous := []*icpdb.ICPResultRow{{Domain: "a.com", Licence: "ICP1"}}
	changes := compareICPResults(latest, previous)
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes, got %d", len(changes))
	}
}

func TestCompareICPResults_EmptyDomain(t *testing.T) {
	latest := []*icpdb.ICPResultRow{{Domain: "", Licence: "ICP1"}}
	previous := []*icpdb.ICPResultRow{{Domain: "a.com", Licence: "ICP1"}}
	changes := compareICPResults(latest, previous)
	// Empty domain in latest is skipped, but previous has a.com which won't match
	if len(changes) > 2 {
		t.Fatalf("expected at most 2 changes, got %d", len(changes))
	}
}

// ============================================================
// handleICPCompare tests
// ============================================================

func TestHandleICPCompare_RepoNil(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/compare?keyword=test.com", nil)
	w := httptest.NewRecorder()
	s.handleICPCompare(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHandleICPCompare_MissingKeyword(t *testing.T) {
	s := &Server{icpRepo: newMockICPRepo()}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/compare", nil)
	w := httptest.NewRecorder()
	s.handleICPCompare(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleICPCompare_Success(t *testing.T) {
	repo := newMockICPRepo()
	s := &Server{icpRepo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/compare?keyword=a.com&type=web", nil)
	w := httptest.NewRecorder()
	s.handleICPCompare(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["success"] != true {
		t.Fatal("expected success=true")
	}
}

// ============================================================
// handleICPHistory tests
// ============================================================

func TestHandleICPHistory_RepoNil(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/history?task_id=123", nil)
	w := httptest.NewRecorder()
	s.handleICPHistory(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHandleICPHistory_MissingParam(t *testing.T) {
	s := &Server{icpRepo: newMockICPRepo()}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/history", nil)
	w := httptest.NewRecorder()
	s.handleICPHistory(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleICPHistory_MethodNotAllowed(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/icp/history?task_id=123", nil)
	w := httptest.NewRecorder()
	s.handleICPHistory(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// ============================================================
// handleICPHistoryResults tests
// ============================================================

func TestHandleICPHistoryResults_RepoNil(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/history/1/results", nil)
	w := httptest.NewRecorder()
	s.handleICPHistoryResults(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHandleICPHistoryResults_InvalidID(t *testing.T) {
	s := &Server{icpRepo: newMockICPRepo()}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/icp/history/abc/results", nil)
	w := httptest.NewRecorder()
	s.handleICPHistoryResults(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
