package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/model"
	"slices"
)

func defaultICPConfig() adapter.ICPConfig {
	return adapter.ICPConfig{
		Enabled:     true,
		BaseURL:     "http://localhost:16181",
		APIKey:      "test-key",
		Timeout:     30,
		DefaultType: "web",
	}
}

func disabledICPConfig() adapter.ICPConfig {
	return adapter.ICPConfig{Enabled: false}
}

func missingBaseURLConfig() adapter.ICPConfig {
	return adapter.ICPConfig{Enabled: true, BaseURL: ""}
}

// --- Type tests ---

func TestICPQueryRunner_Type(t *testing.T) {
	r := NewICPQueryRunner(defaultICPConfig, nil, nil)
	if r.Type() != TaskICPQuery {
		t.Errorf("expected %s, got %s", TaskICPQuery, r.Type())
	}
}

// --- Validation tests ---

func TestICPQueryRunner_MissingQueries(t *testing.T) {
	r := NewICPQueryRunner(defaultICPConfig, nil, nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{})
	if err == nil {
		t.Fatal("expected error for missing queries")
	}
	if !strings.Contains(err.Error(), "missing 'queries' or 'query'") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestICPQueryRunner_DisabledByConfig(t *testing.T) {
	r := NewICPQueryRunner(disabledICPConfig, nil, nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{Queries: []string{"test"}})
	if err == nil {
		t.Fatal("expected error for disabled config")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestICPQueryRunner_MissingBaseURL(t *testing.T) {
	r := NewICPQueryRunner(missingBaseURLConfig, nil, nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{Queries: []string{"test"}})
	if err == nil {
		t.Fatal("expected error for missing base URL")
	}
	if !strings.Contains(err.Error(), "base_url not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestICPQueryRunner_InvalidType(t *testing.T) {
	r := NewICPQueryRunner(defaultICPConfig, nil, nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{
		Queries: []string{"test"},
		Type:    "zzz",
	})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid ICP query type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestICPQueryRunner_TooManyQueries(t *testing.T) {
	r := NewICPQueryRunner(defaultICPConfig, nil, nil)
	queries := make([]string, 101)
	for i := range queries {
		queries[i] = fmt.Sprintf("query%d", i)
	}
	_, err := r.Execute(context.Background(), &model.TaskPayload{Queries: queries})
	if err == nil {
		t.Fatal("expected error for too many queries")
	}
	if !strings.Contains(err.Error(), "too many queries") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestICPQueryRunner_PageSizeCapped(t *testing.T) {
	srv := newMockICPServer(t, map[string]icpMockResp{
		"test": {Total: 5, Results: 5},
	})
	defer srv.Close()

	cfg := adapter.ICPConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Timeout: 5, DefaultType: "web",
	}
	r := NewICPQueryRunner(func() adapter.ICPConfig { return cfg }, nil, nil)

	result, err := r.Execute(context.Background(), &model.TaskPayload{
		Queries:     []string{"test"},
		PageSizeICP: 200,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "共") {
		t.Errorf("page_size should be capped to 100, got: %s", result)
	}
}

// --- Payload defaults tests ---

func TestICPQueryRunner_PayloadDefaults(t *testing.T) {
	srv := newMockICPServer(t, map[string]icpMockResp{
		"test": {Total: 10, Results: 10},
	})
	defer srv.Close()

	cfg := adapter.ICPConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Timeout: 5, DefaultType: "web",
	}
	r := NewICPQueryRunner(func() adapter.ICPConfig { return cfg }, nil, nil)

	// Only pass queries, verify defaults for type/page/page_size
	result, err := r.Execute(context.Background(), &model.TaskPayload{
		Queries: []string{"test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "web") {
		t.Errorf("expected default types=web, got: %s", result)
	}
	if !strings.Contains(result, "1/1") {
		t.Errorf("expected success, got: %s", result)
	}
	if !strings.Contains(result, "10 条记录") {
		t.Errorf("expected 10 records, got: %s", result)
	}
}

func TestICPQueryRunner_SingleQueryString(t *testing.T) {
	srv := newMockICPServer(t, map[string]icpMockResp{
		"example.com": {Total: 3, Results: 3},
	})
	defer srv.Close()

	cfg := adapter.ICPConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Timeout: 5, DefaultType: "web",
	}
	r := NewICPQueryRunner(func() adapter.ICPConfig { return cfg }, nil, nil)

	result, err := r.Execute(context.Background(), &model.TaskPayload{Query: "example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "1/1") {
		t.Errorf("expected single query success, got: %s", result)
	}
}

func TestICPQueryRunner_ConfigDefaultType(t *testing.T) {
	srv := newMockICPServer(t, map[string]icpMockResp{
		"test": {Total: 1, Results: 1},
	})
	defer srv.Close()

	cfg := adapter.ICPConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Timeout: 5, DefaultType: "app",
	}
	r := NewICPQueryRunner(func() adapter.ICPConfig { return cfg }, nil, nil)

	// When Type is empty, parseICPTypes falls back to hardcoded "web"
	result, err := r.Execute(context.Background(), &model.TaskPayload{
		Queries: []string{"test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "web") {
		t.Errorf("expected types=web default fallback, got: %s", result)
	}
}

// --- Success / failure integration tests ---

type icpMockResp struct {
	Total   int
	Results int
}

func newMockICPServer(t *testing.T, responses map[string]icpMockResp) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/query/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		search := r.URL.Query().Get("search")
		resp, ok := responses[search]
		if !ok {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200, "msg": "success",
				"params": map[string]interface{}{
					"list": []interface{}{}, "total": 0,
				},
			})
			return
		}
		items := make([]map[string]interface{}, resp.Results)
		for i := range items {
			items[i] = map[string]interface{}{
				"domain":     fmt.Sprintf("%s-%d.com", search, i),
				"licence":    "京ICP备12345678号",
				"unitName":   "测试公司",
				"natureName": "企业",
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200, "msg": "success",
			"params": map[string]interface{}{
				"list": items, "total": resp.Total,
			},
		})
	})
	return httptest.NewServer(mux)
}

func newMockICPErrorServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/query/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		search := r.URL.Query().Get("search")
		if search == "fail500" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if search == "fail_api" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 429, "msg": "rate limit",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200, "msg": "success",
			"params": map[string]interface{}{
				"list": []interface{}{
					map[string]interface{}{
						"domain":   search + ".com",
						"licence":  "京ICP备00000000号",
						"unitName": "公司",
					},
				},
				"total": 1,
			},
		})
	})
	return httptest.NewServer(mux)
}

func TestICPQueryRunner_SingleQuerySuccess(t *testing.T) {
	srv := newMockICPServer(t, map[string]icpMockResp{
		"example.com": {Total: 23, Results: 3},
	})
	defer srv.Close()

	cfg := adapter.ICPConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Timeout: 5, DefaultType: "web",
	}
	r := NewICPQueryRunner(func() adapter.ICPConfig { return cfg }, nil, nil)

	result, err := r.Execute(context.Background(), &model.TaskPayload{
		Queries:     []string{"example.com"},
		Type:        "web",
		PageSizeICP: 40,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "1/1") {
		t.Errorf("expected 1/1 succeeded, got: %s", result)
	}
	if !strings.Contains(result, "23 条记录") {
		t.Errorf("expected total 23 records, got: %s", result)
	}
}

func TestICPQueryRunner_MultiQueryPartialFailure(t *testing.T) {
	srv := newMockICPErrorServer()
	defer srv.Close()

	cfg := adapter.ICPConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Timeout: 5, DefaultType: "web",
	}
	r := NewICPQueryRunner(func() adapter.ICPConfig { return cfg }, nil, nil)

	result, err := r.Execute(context.Background(), &model.TaskPayload{
		Queries: []string{"ok1", "fail_api", "ok2"},
		Type:    "web",
		Extra:   map[string]any{"fail_fast": false},
	})
	// Partial failure: nil error, result string has errors info
	if err != nil {
		t.Fatalf("partial failure should return nil error, got: %v", err)
	}
	if !strings.Contains(result, "2/3") {
		t.Errorf("expected 2/3 succeeded, got: %s", result)
	}
	if !strings.Contains(result, "❌") {
		t.Errorf("expected error marker, got: %s", result)
	}
	if !strings.Contains(result, "rate limit") {
		t.Errorf("expected error detail, got: %s", result)
	}
}

func TestICPQueryRunner_FailFast(t *testing.T) {
	srv := newMockICPErrorServer()
	defer srv.Close()

	cfg := adapter.ICPConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Timeout: 5, DefaultType: "web",
	}
	r := NewICPQueryRunner(func() adapter.ICPConfig { return cfg }, nil, nil)

	result, err := r.Execute(context.Background(), &model.TaskPayload{
		Queries: []string{"fail500", "ok1", "ok2"},
		Type:    "web",
		Extra:   map[string]any{"fail_fast": true},
	})
	// All failed (0 succeeded with fail_fast on first query) → error
	if err == nil {
		t.Fatal("expected error when all queries failed")
	}
	if !strings.Contains(result, "0/3") {
		t.Errorf("expected 0/3 succeeded, got: %s", result)
	}
}

func TestICPQueryRunner_AllFail(t *testing.T) {
	srv := newMockICPErrorServer()
	defer srv.Close()

	cfg := adapter.ICPConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Timeout: 5, DefaultType: "web",
	}
	r := NewICPQueryRunner(func() adapter.ICPConfig { return cfg }, nil, nil)

	result, err := r.Execute(context.Background(), &model.TaskPayload{
		Queries: []string{"fail500", "fail_api"},
	})
	if err == nil {
		t.Fatal("expected error when all queries failed")
	}
	if !strings.Contains(result, "0/2") {
		t.Errorf("expected 0/2 succeeded, got: %s", result)
	}
}

func TestICPQueryRunner_ContextCancel(t *testing.T) {
	srv := newMockICPServer(t, map[string]icpMockResp{
		"test1": {Total: 5, Results: 5},
		"test2": {Total: 5, Results: 5},
	})
	defer srv.Close()

	cfg := adapter.ICPConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Timeout: 5, DefaultType: "web",
	}
	r := NewICPQueryRunner(func() adapter.ICPConfig { return cfg }, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := r.Execute(ctx, &model.TaskPayload{
		Queries: []string{"test1", "test2"},
	})
	if err == nil {
		t.Fatal("expected context canceled error")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled, got: %v", err)
	}
}

func TestICPQueryRunner_ContextTimeout(t *testing.T) {
	srv := newMockICPServer(t, map[string]icpMockResp{
		"test": {Total: 1, Results: 1},
	})
	defer srv.Close()

	cfg := adapter.ICPConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Timeout: 5, DefaultType: "web",
	}
	r := NewICPQueryRunner(func() adapter.ICPConfig { return cfg }, nil, nil)

	// Use a very short timeout — should still succeed since mock responds fast
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := r.Execute(ctx, &model.TaskPayload{
		Queries: []string{"test"},
	})
	if err != nil {
		t.Fatalf("unexpected error with reasonable timeout: %v", err)
	}
	if !strings.Contains(result, "1/1") {
		t.Errorf("expected success, got: %s", result)
	}
}

// --- Scheduler integration tests ---

func TestAllTaskTypes_ContainsICP(t *testing.T) {
	if !slices.Contains(AllTaskTypes(), TaskICPQuery) {
		t.Error("AllTaskTypes() should contain TaskICPQuery")
	}
}

func TestTaskTypeLabel_ICP(t *testing.T) {
	label := TaskTypeLabel(TaskICPQuery)
	if label != "ICP 备案查询" {
		t.Errorf("expected 'ICP 备案查询', got %q", label)
	}
}

func TestDefaultTemplates_ContainsICPTemplates(t *testing.T) {
	templates := DefaultTemplates()
	icpCount := 0
	for _, tmpl := range templates {
		if tmpl.Type == TaskICPQuery {
			icpCount++
		}
	}
	if icpCount < 2 {
		t.Errorf("expected at least 2 ICP templates, got %d", icpCount)
	}
}

func TestDefaultTemplates_ICPTemplateFields(t *testing.T) {
	templates := DefaultTemplates()
	var daily, weekly *TaskTemplate
	for i := range templates {
		if templates[i].Type == TaskICPQuery {
			if templates[i].ID == "tmpl_daily_icp_company_watch" {
				daily = &templates[i]
			}
			if templates[i].ID == "tmpl_weekly_icp_domain_scan" {
				weekly = &templates[i]
			}
		}
	}
	if daily == nil {
		t.Fatal("daily ICP template not found")
	}
	if weekly == nil {
		t.Fatal("weekly ICP template not found")
	}
	if daily.TimeoutSec != 600 {
		t.Errorf("daily template timeout expected 600, got %d", daily.TimeoutSec)
	}
	if weekly.TimeoutSec != 1800 {
		t.Errorf("weekly template timeout expected 1800, got %d", weekly.TimeoutSec)
	}
	if daily.CronExpr != "0 0 9 * * *" {
		t.Errorf("daily cron expected '0 0 9 * * *', got %q", daily.CronExpr)
	}
	if weekly.CronExpr != "0 0 3 * * 1" {
		t.Errorf("weekly cron expected '0 0 3 * * 1', got %q", weekly.CronExpr)
	}
}

// --- extractBool test ---

func TestExtractBool(t *testing.T) {
	tests := []struct {
		name    string
		payload *model.TaskPayload
		key     string
		def     bool
		want    bool
	}{
		{"key not exists returns default", &model.TaskPayload{}, "missing", false, false},
		{"key not exists returns true default", &model.TaskPayload{}, "missing", true, true},
		{"value is true", &model.TaskPayload{Extra: map[string]any{"flag": true}}, "flag", false, true},
		{"value is false", &model.TaskPayload{Extra: map[string]any{"flag": false}}, "flag", true, false},
		{"value is other type returns default", &model.TaskPayload{Extra: map[string]any{"flag": "yes"}}, "flag", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBool(tt.payload, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("extractBool() = %v, want %v", got, tt.want)
			}
		})
	}
}
