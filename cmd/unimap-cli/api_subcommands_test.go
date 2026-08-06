package main

import (
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIRequestsUseBearerAuthentication(t *testing.T) {
	t.Setenv("UNIMAP_ADMIN_TOKEN", "")
	tokenFile := filepath.Join(t.TempDir(), "admin-token")
	if err := os.WriteFile(tokenFile, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := configureAPIAuth("ignored", tokenFile); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cliAdminToken = "" })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/scheduler/tasks" {
			t.Errorf("unexpected API contract: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer file-secret" {
			t.Errorf("missing bearer token: %q", got)
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	var tasks []cliSchedulerTask
	if err := doGETRequest(server.URL, "/api/v1/scheduler/tasks", 5, &tasks); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForScreenshotBatchPollsAsyncJob(t *testing.T) {
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/screenshot/batch/progress" || r.URL.Query().Get("job_id") != "job-1" {
			t.Errorf("unexpected progress request: %s %s", r.Method, r.URL.String())
		}
		polls++
		if polls == 1 {
			_, _ = w.Write([]byte(`{"status":"running"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"completed","total":2,"success":2}`))
	}))
	defer server.Close()

	result, err := waitForScreenshotBatch(server.URL, "job-1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success != 2 || polls != 2 {
		t.Fatalf("unexpected async result: %+v polls=%d", result, polls)
	}
}

func TestAPIClientRejectsCrossOriginRedirect(t *testing.T) {
	t.Setenv("UNIMAP_ADMIN_TOKEN", "redirect-secret")
	cliAdminToken = ""
	t.Cleanup(func() { cliAdminToken = "" })
	targetReached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetReached = true
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/stolen", http.StatusFound)
	}))
	defer source.Close()

	var out map[string]any
	if err := doGETRequest(source.URL, "/redirect", 5, &out); err == nil || !strings.Contains(err.Error(), "different origin") {
		t.Fatalf("expected cross-origin redirect rejection, got %v", err)
	}
	if targetReached {
		t.Fatal("cross-origin redirect target was reached")
	}
}

func TestSchedulerSubcommandContracts(t *testing.T) {
	t.Setenv("UNIMAP_ADMIN_TOKEN", "")
	cliAdminToken = "contract-token"
	t.Cleanup(func() { cliAdminToken = "" })
	type requestRecord struct{ method, path, query, auth string }
	records := make(chan requestRecord, 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		records <- requestRecord{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")}
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if r.URL.Path == "/api/v1/scheduler/tasks/create" {
			_, _ = w.Write([]byte(`{"id":"new-task","message":"task created"}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	defer server.Close()
	prefix := "/api/v1/scheduler"
	schedulerList(server.URL, prefix, 5, "table")
	schedulerRun([]string{"--id", "task-1"}, server.URL, prefix, 5, "table")
	schedulerCreate([]string{"--name", "daily", "--type", "query", "--cron", "0 * * * *"}, server.URL, prefix, 5, "table")
	schedulerToggle([]string{"--id", "task-1"}, "enable", server.URL, prefix, 5, "table")
	schedulerToggle([]string{"--id", "task-1"}, "disable", server.URL, prefix, 5, "table")
	schedulerDelete([]string{"--id", "task-1"}, server.URL, prefix, 5, "table")
	schedulerHistory([]string{"--task-id", "task-1", "--limit", "9"}, server.URL, prefix, 5, "table")

	want := []requestRecord{
		{http.MethodGet, prefix + "/tasks", "", "Bearer contract-token"},
		{http.MethodPost, prefix + "/tasks/run", "", "Bearer contract-token"},
		{http.MethodPost, prefix + "/tasks/create", "", "Bearer contract-token"},
		{http.MethodPost, prefix + "/tasks/enable", "", "Bearer contract-token"},
		{http.MethodPost, prefix + "/tasks/disable", "", "Bearer contract-token"},
		{http.MethodPost, prefix + "/tasks/delete", "", "Bearer contract-token"},
		{http.MethodGet, prefix + "/history", "task_id=task-1&limit=9", "Bearer contract-token"},
	}
	for index, expected := range want {
		actual := <-records
		if actual != expected {
			t.Fatalf("request %d mismatch: got=%+v want=%+v", index, actual, expected)
		}
	}
}

func TestSplitCSVText(t *testing.T) {
	got := splitCSVText(" https://a.com,https://b.com\nhttps://c.com ,, ")
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}
	if got[0] != "https://a.com" || got[1] != "https://b.com" || got[2] != "https://c.com" {
		t.Fatalf("unexpected values: %+v", got)
	}
}

func TestMaxInt(t *testing.T) {
	if maxInt(1, 2) != 2 {
		t.Fatalf("expected 2")
	}
	if maxInt(7, 3) != 7 {
		t.Fatalf("expected 7")
	}
}

func TestDoJSONRequestSuccessAndFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"hello":"world"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`bad request`))
	}))
	defer ts.Close()

	var okResp map[string]string
	if err := doJSONRequest(ts.URL, "/ok", 5, map[string]string{"a": "b"}, &okResp); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if okResp["hello"] != "world" {
		t.Fatalf("unexpected response: %+v", okResp)
	}

	var failResp map[string]string
	err := doJSONRequest(ts.URL, "/fail", 5, map[string]string{"a": "b"}, &failResp)
	if err == nil {
		t.Fatalf("expected non-2xx error")
	}
	if !strings.Contains(err.Error(), "status=400") {
		t.Fatalf("expected status in error, got: %v", err)
	}
}

func TestDoFormRequestSuccessAndFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			_ = r.ParseForm()
			if r.FormValue("query") != "app=\"nginx\"" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`invalid form`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"query":"app=\"nginx\""}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	}))
	defer ts.Close()

	values := neturl.Values{}
	values.Set("query", "app=\"nginx\"")

	var okResp map[string]string
	if err := doFormRequest(ts.URL, "/ok", 5, values, &okResp); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if okResp["query"] != "app=\"nginx\"" {
		t.Fatalf("unexpected response: %+v", okResp)
	}

	var failResp map[string]string
	err := doFormRequest(ts.URL, "/fail", 5, values, &failResp)
	if err == nil {
		t.Fatalf("expected non-2xx error")
	}
	if !strings.Contains(err.Error(), "status=500") {
		t.Fatalf("expected status in error, got: %v", err)
	}
}
