package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/service"
)

func buildTestServerWithScreenshotBase(baseDir string) *Server {
	cfg := &config.Config{}
	cfg.Screenshot.BaseDir = baseDir
	return &Server{
		config:        cfg,
		screenshotApp: service.NewScreenshotAppService(baseDir),
		screenshotMgr: &screenshot.Manager{},
		batchJobs:     newBatchJobStore(),
	}
}

func assertBatchURLAcceptedWithFailedResult(t *testing.T, s *Server, w *httptest.ResponseRecorder, wantErr string) {
	t.Helper()
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var startResp struct {
		JobID  string `json:"job_id"`
		Total  int    `json:"total"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &startResp); err != nil {
		t.Fatalf("unmarshal start response failed: %v", err)
	}
	if startResp.JobID == "" || startResp.Total != 1 || startResp.Status != "completed" {
		t.Fatalf("unexpected start response: %+v", startResp)
	}

	job := requestBatchJobProgress(t, s, startResp.JobID)
	if job.Status != batchJobCompleted || job.Total != 1 || job.Completed != 1 || job.Success != 0 || job.Failed != 1 {
		t.Fatalf("unexpected job progress: %+v", job)
	}
	if len(job.Results) != 1 || job.Results[0].Success || !strings.Contains(job.Results[0].Error, wantErr) {
		t.Fatalf("unexpected failed result: %+v", job.Results)
	}
}

func requestBatchJobProgress(t *testing.T, s *Server, jobID string) batchJob {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/batch/progress?job_id="+jobID, nil)
	progressW := httptest.NewRecorder()
	s.handleBatchScreenshotProgress(progressW, req)
	if progressW.Code != http.StatusOK {
		t.Fatalf("expected progress 200, got %d: %s", progressW.Code, progressW.Body.String())
	}
	var job batchJob
	if err := json.Unmarshal(progressW.Body.Bytes(), &job); err != nil {
		t.Fatalf("unmarshal progress response failed: %v", err)
	}
	return job
}

func TestBatchJobStoreCleanup(t *testing.T) {
	store := newBatchJobStore()
	store.create("running", 1)
	store.create("old-completed", 1)
	store.complete("old-completed", []screenshot.BatchScreenshotResult{{URL: "https://old.example", Success: true}}, 1, 0)
	store.create("old-failed", 1)
	store.fail("old-failed", errors.New("capture failed"))
	store.create("fresh", 1)
	store.complete("fresh", []screenshot.BatchScreenshotResult{{URL: "https://fresh.example", Success: true}}, 1, 0)

	oldEndedAt := time.Now().Add(-2 * time.Hour)
	store.jobs["old-completed"].EndedAt = &oldEndedAt
	store.jobs["old-failed"].EndedAt = &oldEndedAt

	store.cleanup(1 * time.Hour)

	if store.getSnapshot("old-completed") != nil {
		t.Fatal("expected old completed job to be removed")
	}
	if store.getSnapshot("old-failed") != nil {
		t.Fatal("expected old failed job to be removed")
	}
	if store.getSnapshot("fresh") == nil {
		t.Fatal("expected fresh completed job to remain")
	}
	if running := store.getSnapshot("running"); running == nil || running.Status != batchJobRunning {
		t.Fatalf("expected running job to remain, got %+v", running)
	}
}

func TestHandleBatchScreenshotProgress_NilStore(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/batch/progress?job_id=missing", nil)
	w := httptest.NewRecorder()

	s.handleBatchScreenshotProgress(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "job_not_found") {
		t.Fatalf("expected job_not_found response, got %q", w.Body.String())
	}
}

func TestClassifyBatchURLsPreservesOriginalIndices(t *testing.T) {
	urls := []string{"https://8.8.8.8", "://invalid", "http://127.0.0.1", "https://1.1.1.1"}

	valid, invalid := classifyBatchURLs(urls)

	if len(valid) != 2 || valid[0].Index != 0 || valid[0].URL != urls[0] || valid[1].Index != 3 || valid[1].URL != urls[3] {
		t.Fatalf("unexpected valid items: %+v", valid)
	}
	if len(invalid) != 2 {
		t.Fatalf("expected 2 invalid items, got %+v", invalid)
	}
	if invalid[0].Index != 1 || invalid[0].Result.URL != urls[1] || !strings.Contains(invalid[0].Result.Error, "invalid URL") {
		t.Fatalf("unexpected first invalid item: %+v", invalid[0])
	}
	if invalid[1].Index != 2 || invalid[1].Result.URL != urls[2] || !strings.Contains(invalid[1].Result.Error, "private/internal") {
		t.Fatalf("unexpected second invalid item: %+v", invalid[1])
	}
}

func TestMergeBatchURLResultsPreservesOriginalOrder(t *testing.T) {
	valid := []batchURLItem{{Index: 0, URL: "https://example.com"}, {Index: 2, URL: "https://example.org"}, {Index: 4, URL: "https://example.net"}}
	invalid := []batchURLResult{
		{Index: 1, Result: screenshot.BatchScreenshotResult{URL: "://invalid", Error: "invalid URL"}},
		{Index: 3, Result: screenshot.BatchScreenshotResult{URL: "http://127.0.0.1", Error: "url resolves to private/internal address"}},
	}
	captured := []screenshot.BatchScreenshotResult{
		{URL: "https://example.com", Success: true},
		{URL: "https://example.org", Error: "timeout"},
		{URL: "https://example.net", Success: true},
	}

	merged, success, failed := mergeBatchURLResults(5, valid, invalid, captured)

	if success != 2 || failed != 3 {
		t.Fatalf("expected success=2 failed=3, got success=%d failed=%d", success, failed)
	}
	wantURLs := []string{"https://example.com", "://invalid", "https://example.org", "http://127.0.0.1", "https://example.net"}
	for i, want := range wantURLs {
		if merged[i].URL != want {
			t.Fatalf("result %d URL: expected %q, got %q in %+v", i, want, merged[i].URL, merged)
		}
	}
	if !merged[0].Success || merged[2].Success || !strings.Contains(merged[2].Error, "timeout") || !merged[4].Success {
		t.Fatalf("unexpected merged success/error state: %+v", merged)
	}
}

func TestNormalizeScreenshotPathToken(t *testing.T) {
	tests := []struct {
		in   string
		ok   bool
		want string
	}{
		{in: "batch-001", ok: true, want: "batch-001"},
		{in: "", ok: false},
		{in: "..", ok: false},
		{in: "a/b", ok: false},
		{in: "a\\b", ok: false},
	}

	for _, tt := range tests {
		got, ok := normalizeScreenshotPathToken(tt.in)
		if ok != tt.ok {
			t.Fatalf("input %q expected ok=%v got %v", tt.in, tt.ok, ok)
		}
		if ok && got != tt.want {
			t.Fatalf("input %q expected %q got %q", tt.in, tt.want, got)
		}
	}
}

func TestHandleScreenshotBatchesAndFiles(t *testing.T) {
	baseDir := t.TempDir()
	batchDir := filepath.Join(baseDir, "batch-a")
	if err := os.MkdirAll(batchDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(batchDir, "x.png"), []byte("x"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	s := buildTestServerWithScreenshotBase(baseDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/batches", nil)
	w := httptest.NewRecorder()
	s.handleScreenshotBatches(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var batchResp struct {
		Success bool `json:"success"`
		Count   int  `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &batchResp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !batchResp.Success || batchResp.Count != 1 {
		t.Fatalf("unexpected batch response: %+v", batchResp)
	}

	reqFiles := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/batches/files?batch=batch-a", nil)
	wFiles := httptest.NewRecorder()
	s.handleScreenshotBatchFiles(wFiles, reqFiles)
	if wFiles.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wFiles.Code)
	}

	var filesResp struct {
		Success bool `json:"success"`
		Count   int  `json:"count"`
	}
	if err := json.Unmarshal(wFiles.Body.Bytes(), &filesResp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !filesResp.Success || filesResp.Count != 1 {
		t.Fatalf("unexpected files response: %+v", filesResp)
	}
}

func TestHandleScreenshotDeleteSafety(t *testing.T) {
	baseDir := t.TempDir()
	batchDir := filepath.Join(baseDir, "batch-z")
	if err := os.MkdirAll(batchDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(batchDir, "a.png"), []byte("x"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	s := buildTestServerWithScreenshotBase(baseDir)

	badReq := httptest.NewRequest(http.MethodDelete, "/api/v1/screenshot/file/delete?batch=../evil&file=a.png", nil)
	badReq.Header.Set("Origin", "http://localhost:8448")
	badW := httptest.NewRecorder()
	s.handleScreenshotFileDelete(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for traversal, got %d", badW.Code)
	}

	okReq := httptest.NewRequest(http.MethodDelete, "/api/v1/screenshot/file/delete?batch=batch-z&file=a.png", nil)
	okReq.Header.Set("Origin", "http://localhost:8448")
	okW := httptest.NewRecorder()
	s.handleScreenshotFileDelete(okW, okReq)
	if okW.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete, got %d", okW.Code)
	}
	if _, err := os.Stat(filepath.Join(batchDir, "a.png")); !os.IsNotExist(err) {
		t.Fatalf("expected file deleted, stat err=%v", err)
	}
}

// ============================================================
// handleScreenshot error path tests
// ============================================================

func TestHandleScreenshot_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot", nil)
	w := httptest.NewRecorder()
	s.handleScreenshot(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleScreenshot_MissingURL(t *testing.T) {
	s := &Server{}
	body := strings.NewReader(`{"url":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleScreenshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing_url") {
		t.Fatalf("expected 'missing_url' in body, got %q", w.Body.String())
	}
}

func TestHandleScreenshot_EmptyBody(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot", nil)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleScreenshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleScreenshot_PrivateIPBlocked(t *testing.T) {
	s := &Server{}
	body := strings.NewReader(`{"url":"http://127.0.0.1:8080"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleScreenshot(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "blocked_url") {
		t.Fatalf("expected 'blocked_url' in body, got %q", w.Body.String())
	}
}

func TestHandleScreenshot_PrivateIPBlockedLocalhost(t *testing.T) {
	s := &Server{}
	body := strings.NewReader(`{"url":"http://localhost:3000/admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleScreenshot(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "blocked_url") {
		t.Fatalf("expected 'blocked_url' in body, got %q", w.Body.String())
	}
}

// ============================================================
// handleSearchEngineScreenshot error path tests
// ============================================================

func TestHandleSearchEngineScreenshot_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/engine", nil)
	w := httptest.NewRecorder()
	s.handleSearchEngineScreenshot(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleSearchEngineScreenshot_NoScreenshotApp(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/engine?engine=fofa&query=test", nil)
	w := httptest.NewRecorder()
	s.handleSearchEngineScreenshot(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "screenshot_manager_unavailable") {
		t.Fatalf("expected 'screenshot_manager_unavailable' in body, got %q", w.Body.String())
	}
}

func TestHandleSearchEngineScreenshot_MissingQuery(t *testing.T) {
	s := buildTestServerWithScreenshotBase(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/engine?engine=fofa", nil)
	w := httptest.NewRecorder()
	s.handleSearchEngineScreenshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing_parameters") {
		t.Fatalf("expected 'missing_parameters' in body, got %q", w.Body.String())
	}
}

func TestHandleSearchEngineScreenshot_MissingEngine(t *testing.T) {
	s := buildTestServerWithScreenshotBase(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/engine?query=test", nil)
	w := httptest.NewRecorder()
	s.handleSearchEngineScreenshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing_parameters") {
		t.Fatalf("expected 'missing_parameters' in body, got %q", w.Body.String())
	}
}

// ============================================================
// handleBatchScreenshot error path tests
// ============================================================

func TestHandleBatchScreenshot_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/batch", nil)
	w := httptest.NewRecorder()
	s.handleBatchScreenshot(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleBatchScreenshot_NoScreenshotApp(t *testing.T) {
	s := &Server{}
	body := strings.NewReader(`{"query_id":"1","engines":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/batch", body)
	w := httptest.NewRecorder()
	s.handleBatchScreenshot(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "screenshot_manager_unavailable") {
		t.Fatalf("expected 'screenshot_manager_unavailable' in body, got %q", w.Body.String())
	}
}

func TestHandleBatchScreenshot_PrivateTargetURL(t *testing.T) {
	s := buildTestServerWithScreenshotBase(t.TempDir())
	body := strings.NewReader(`{"query_id":"1","targets":[{"url":"http://192.168.1.1:8080"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/batch", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleBatchScreenshot(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "blocked_url") {
		t.Fatalf("expected 'blocked_url' in body, got %q", w.Body.String())
	}
}

func TestHandleBatchScreenshot_PrivateTargetIPField(t *testing.T) {
	s := buildTestServerWithScreenshotBase(t.TempDir())
	body := strings.NewReader(`{"query_id":"1","targets":[{"ip":"10.0.0.1"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/batch", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleBatchScreenshot(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "blocked_url") {
		t.Fatalf("expected 'blocked_url' in body, got %q", w.Body.String())
	}
}

func TestHandleBatchScreenshot_InvalidJSON(t *testing.T) {
	s := buildTestServerWithScreenshotBase(t.TempDir())
	body := strings.NewReader(`not-json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/batch", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleBatchScreenshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ============================================================
// handleBatchURLsScreenshot error path tests
// ============================================================

func TestHandleBatchURLsScreenshot_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/batch-urls", nil)
	w := httptest.NewRecorder()
	s.handleBatchURLsScreenshot(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleBatchURLsScreenshot_NoScreenshotApp(t *testing.T) {
	s := &Server{}
	body := strings.NewReader(`{"urls":["https://example.com"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/batch-urls", body)
	w := httptest.NewRecorder()
	s.handleBatchURLsScreenshot(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "screenshot_manager_unavailable") {
		t.Fatalf("expected 'screenshot_manager_unavailable' in body, got %q", w.Body.String())
	}
}

func TestHandleBatchURLsScreenshot_PrivateIP(t *testing.T) {
	s := buildTestServerWithScreenshotBase(t.TempDir())
	body := strings.NewReader(`{"urls":["https://127.0.0.1:8080"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/batch-urls", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleBatchURLsScreenshot(w, req)

	assertBatchURLAcceptedWithFailedResult(t, s, w, "private/internal")
}

func TestHandleBatchURLsScreenshot_InvalidURL(t *testing.T) {
	s := buildTestServerWithScreenshotBase(t.TempDir())
	body := strings.NewReader(`{"urls":["://invalid"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/batch-urls", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleBatchURLsScreenshot(w, req)

	assertBatchURLAcceptedWithFailedResult(t, s, w, "invalid URL")
}

func TestHandleBatchURLsScreenshot_InvalidScheme(t *testing.T) {
	s := buildTestServerWithScreenshotBase(t.TempDir())
	body := strings.NewReader(`{"urls":["file:///etc/passwd"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/batch-urls", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleBatchURLsScreenshot(w, req)

	assertBatchURLAcceptedWithFailedResult(t, s, w, "http/https")
}

func TestHandleBatchURLsScreenshot_InvalidJSON(t *testing.T) {
	s := buildTestServerWithScreenshotBase(t.TempDir())
	body := strings.NewReader(`not-json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/batch-urls", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleBatchURLsScreenshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ============================================================
// handleScreenshotRouterStatus tests
// ============================================================

func TestHandleScreenshotRouterStatus(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/router/status", nil)
	w := httptest.NewRecorder()
	s.handleScreenshotRouterStatus(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 200 or 503, got %d", w.Code)
	}
}

// ============================================================
// resolveScreenshotBatchDir tests
// ============================================================

func TestResolveScreenshotBatchDir_Default(t *testing.T) {
	s := &Server{}
	got, ok := s.resolveScreenshotBatchDir("test-batch")
	if !ok {
		t.Fatal("expected ok=true for valid batch token")
	}
	if got == "" {
		t.Fatal("expected non-empty batch dir")
	}
	if !strings.Contains(got, "screenshots") {
		t.Fatalf("expected path containing 'screenshots', got %q", got)
	}
}

func TestResolveScreenshotBatchDir_InvalidBatch(t *testing.T) {
	s := &Server{}
	_, ok := s.resolveScreenshotBatchDir("")
	if ok {
		t.Fatal("expected ok=false for empty batch token")
	}
}

func TestResolveScreenshotBatchDir_TraversalBatch(t *testing.T) {
	s := &Server{}
	_, ok := s.resolveScreenshotBatchDir("../evil")
	if ok {
		t.Fatal("expected ok=false for traversal batch token")
	}
}

// ============================================================
// handleScreenshotBatchDelete tests
// ============================================================

func TestHandleScreenshotBatchDelete_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/batch/test", nil)
	w := httptest.NewRecorder()
	s.handleScreenshotBatchDelete(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// ============================================================
// handleScreenshotFileDelete tests
// ============================================================

func TestHandleScreenshotFileDelete_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/file/test.png", nil)
	w := httptest.NewRecorder()
	s.handleScreenshotFileDelete(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// ============================================================
// writeBatchScreenshotError tests
// ============================================================

func TestWriteBatchScreenshotError_NoURLs(t *testing.T) {
	w := httptest.NewRecorder()
	writeBatchScreenshotError(w, errors.New("no urls provided"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWriteBatchScreenshotError_TooMany(t *testing.T) {
	w := httptest.NewRecorder()
	writeBatchScreenshotError(w, errors.New("too many urls"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWriteBatchScreenshotError_Default(t *testing.T) {
	w := httptest.NewRecorder()
	writeBatchScreenshotError(w, errors.New("some other error"))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ============================================================
// handleSetScreenshotMode tests
// ============================================================

func TestHandleSetScreenshotMode_MethodNotAllowed(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/mode", nil)
	w := httptest.NewRecorder()
	s.handleSetScreenshotMode(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleSetScreenshotMode_InvalidMode(t *testing.T) {
	s := &Server{config: &config.Config{}}
	body := `{"mode":"invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/mode", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleSetScreenshotMode(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetScreenshotMode_ValidMode(t *testing.T) {
	s := &Server{config: &config.Config{}}
	body := `{"mode":"cdp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/mode", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleSetScreenshotMode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ============================================================
// clearAllEngineCookies tests
// ============================================================

func TestClearAllEngineCookies(t *testing.T) {
	cfg := &config.Config{}
	cfg.Engines.Fofa.Cookies = []config.Cookie{{Name: "test", Value: "val"}}
	cfg.Engines.Hunter.Cookies = []config.Cookie{{Name: "test", Value: "val"}}
	s := &Server{config: cfg}
	s.clearAllEngineCookies()
	if len(cfg.Engines.Fofa.Cookies) != 0 {
		t.Fatal("expected fofa cookies cleared")
	}
	if len(cfg.Engines.Hunter.Cookies) != 0 {
		t.Fatal("expected hunter cookies cleared")
	}
}
