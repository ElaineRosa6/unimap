package web

import (
	"context"
	"encoding/json"
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
	"github.com/unimap/project/internal/tamper"
)

func TestHandleTamperHistoryDelete(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if chdirErr := os.Chdir(tmpDir); chdirErr != nil {
		t.Fatalf("chdir failed: %v", chdirErr)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	url := "https://example.com"
	storage := tamper.NewHashStorage("./hash_store")
	if saveErr := storage.SaveCheckRecord(url, &tamper.CheckRecord{
		URL:       url,
		CheckType: "normal",
		Timestamp: time.Now().Unix(),
	}); saveErr != nil {
		t.Fatalf("save record failed: %v", saveErr)
	}

	recordsBase := filepath.Join("hash_store", "records")
	entries, err := os.ReadDir(recordsBase)
	if err != nil {
		t.Fatalf("read records base failed: %v", err)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("expected exactly one url directory, got %d", len(entries))
	}
	recordsDir := filepath.Join(recordsBase, entries[0].Name())

	s := &Server{tamperApp: service.NewTamperAppService("./hash_store", nil)}

	missingReq := httptest.NewRequest(http.MethodDelete, "/api/v1/tamper/history/delete", nil)
	missingReq.Header.Set("Origin", "http://localhost:8448")
	missingW := httptest.NewRecorder()
	s.handleTamperHistoryDelete(missingW, missingReq)
	if missingW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing url, got %d", missingW.Code)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tamper/history/delete?url=https://example.com", nil)
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleTamperHistoryDelete(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Success bool   `json:"success"`
		URL     string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !resp.Success || resp.URL != url {
		t.Fatalf("unexpected response: %+v", resp)
	}

	if _, err := os.Stat(recordsDir); !os.IsNotExist(err) {
		t.Fatalf("expected records dir removed, stat err=%v", err)
	}
}

func TestHandleTamperHistoryExport(t *testing.T) {
	dir := t.TempDir()
	url := "https://export.example.test"
	storage := tamper.NewHashStorage(dir)
	if err := storage.SaveCheckRecord(url, &tamper.CheckRecord{URL: url, CheckType: "normal", Timestamp: time.Now().Unix()}); err != nil {
		t.Fatal(err)
	}
	s := &Server{tamperApp: service.NewTamperAppService(dir, nil)}
	req := httptest.NewRequest(http.MethodGet, "/api/tamper/history/export?url=https://export.example.test", nil)
	w := httptest.NewRecorder()
	s.handleTamperHistoryExport(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "tamper-history.json") {
		t.Fatalf("unexpected content disposition %q", got)
	}
	if !strings.Contains(w.Body.String(), url) {
		t.Fatalf("export missing URL: %s", w.Body.String())
	}
}

func TestHandleTamperHistoryExportCombinesFiltersAndLimit(t *testing.T) {
	dir := t.TempDir()
	targetURL := "https://keep.example.test/admin"
	storage := tamper.NewHashStorage(dir)
	records := []*tamper.CheckRecord{
		{ID: "keep", URL: targetURL, CheckType: "normal", DetectionMode: "security", Tampered: true, Timestamp: 300},
		{ID: "wrong-mode", URL: targetURL, CheckType: "normal", DetectionMode: "balanced", Tampered: true, Timestamp: 200},
		{ID: "wrong-url", URL: "https://other.example.test/admin", CheckType: "normal", DetectionMode: "security", Tampered: true, Timestamp: 100},
	}
	for _, record := range records {
		if err := storage.SaveCheckRecord(record.URL, record); err != nil {
			t.Fatalf("save check record %s: %v", record.ID, err)
		}
	}

	s := &Server{tamperApp: service.NewTamperAppService(dir, nil)}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/tamper/history/export?url="+targetURL+"&type=tampered&mode=security&q=keep&limit=1", nil)
	w := httptest.NewRecorder()
	s.handleTamperHistoryExport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var exported []struct {
		ID            string `json:"id"`
		URL           string `json:"url"`
		Status        string `json:"status"`
		DetectionMode string `json:"detection_mode"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &exported); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	if len(exported) != 1 || exported[0].ID != "keep" || exported[0].URL != targetURL || exported[0].Status != "tampered" || exported[0].DetectionMode != "security" {
		t.Fatalf("unexpected filtered export: %+v", exported)
	}
}

func TestHandleTamperHistoryExportReturnsStableErrorWhenStorageFails(t *testing.T) {
	dir := t.TempDir()
	blockedBaseDir := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blockedBaseDir, []byte("block directory creation"), 0o600); err != nil {
		t.Fatalf("create blocking file: %v", err)
	}

	s := &Server{tamperApp: service.NewTamperAppService(blockedBaseDir, nil)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tamper/history/export", nil)
	w := httptest.NewRecorder()
	s.handleTamperHistoryExport(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	var response apiErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode API error: %v", err)
	}
	if response.Success || response.Error.Code != "export_history_failed" || response.Error.Message != "export history failed" {
		t.Fatalf("unexpected API error: %+v", response)
	}
}

func TestHandleTamperHistoryExportRejectsWrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tamper/history/export", nil)
	w := httptest.NewRecorder()
	s.handleTamperHistoryExport(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleTamperBaselineDeleteMethodContract(t *testing.T) {
	s := &Server{tamperApp: service.NewTamperAppService("./hash_store", nil)}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tamper/baseline/delete?url=https://example.com", nil)
	w := httptest.NewRecorder()
	s.handleTamperBaselineDelete(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for non-DELETE method, got %d", w.Code)
	}
}

func TestHandleTamperBaselineDeleteByQueryParam(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	targetURL := "https://example.com"
	detector := tamper.NewDetector(tamper.DetectorConfig{BaseDir: "./hash_store"})
	if err := detector.SaveBaseline(targetURL, &tamper.PageHashResult{URL: targetURL, FullHash: "baseline-hash"}); err != nil {
		t.Fatalf("save baseline failed: %v", err)
	}

	if urls, err := detector.ListBaselines(); err != nil {
		t.Fatalf("list baselines failed: %v", err)
	} else if len(urls) != 1 {
		t.Fatalf("expected 1 baseline before delete, got %d", len(urls))
	}

	s := &Server{tamperApp: service.NewTamperAppService("./hash_store", nil)}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tamper/baseline/delete?url=https://example.com", nil)
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleTamperBaselineDelete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if urls, err := detector.ListBaselines(); err != nil {
		t.Fatalf("list baselines after delete failed: %v", err)
	} else if len(urls) != 0 {
		t.Fatalf("expected 0 baselines after delete, got %d", len(urls))
	}
}

// ============================================================
// handleTamperCheck supplementary tests
// ============================================================

func TestHandleTamperCheck_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tamper/check", nil)
	w := httptest.NewRecorder()
	s.handleTamperCheck(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleTamperCheck_InvalidJSON(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tamper/check", strings.NewReader("not-json"))
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleTamperCheck(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleTamperCheck_EmptyURLs(t *testing.T) {
	s := &Server{}
	body := strings.NewReader(`{"urls":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tamper/check", body)
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleTamperCheck(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty URLs, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no_urls_provided") {
		t.Fatalf("expected 'no_urls_provided' in body, got %q", w.Body.String())
	}
}

// ============================================================
// handleTamperBaseline supplementary tests
// ============================================================

func TestHandleTamperBaseline_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tamper/baseline", nil)
	w := httptest.NewRecorder()
	s.handleTamperBaseline(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleTamperBaseline_EmptyURLs(t *testing.T) {
	s := &Server{tamperApp: service.NewTamperAppService("./hash_store", nil)}
	body := strings.NewReader(`{"urls":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tamper/baseline", body)
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleTamperBaseline(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ============================================================
// handleTamperBaselineList tests
// ============================================================

func TestHandleTamperBaselineList_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tamper/baseline/list", nil)
	w := httptest.NewRecorder()
	s.handleTamperBaselineList(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleTamperBaselineList_Empty(t *testing.T) {
	s := &Server{tamperApp: service.NewTamperAppService("./hash_store", nil)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tamper/baseline/list", nil)
	w := httptest.NewRecorder()
	s.handleTamperBaselineList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ============================================================
// handleTamperHistory tests
// ============================================================

func TestHandleTamperHistory_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tamper/history", nil)
	w := httptest.NewRecorder()
	s.handleTamperHistory(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleTamperHistory_Empty(t *testing.T) {
	s := &Server{tamperApp: service.NewTamperAppService("./hash_store", nil)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tamper/history", nil)
	w := httptest.NewRecorder()
	s.handleTamperHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleTamperHistory_UsesOffsetForStoragePagination(t *testing.T) {
	dir := t.TempDir()
	targetURL := "https://pagination.example.test"
	storage := tamper.NewHashStorage(dir)
	for _, timestamp := range []int64{100, 200, 300} {
		if err := storage.SaveCheckRecord(targetURL, &tamper.CheckRecord{
			URL:       targetURL,
			CheckType: "normal",
			Timestamp: timestamp,
		}); err != nil {
			t.Fatalf("save check record: %v", err)
		}
	}

	s := &Server{tamperApp: service.NewTamperAppService(dir, nil)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tamper/history?limit=1&offset=1", nil)
	w := httptest.NewRecorder()
	s.handleTamperHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var response struct {
		Records []struct {
			Timestamp int64 `json:"timestamp"`
		} `json:"records"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Records) != 1 || response.Records[0].Timestamp != 200 {
		t.Fatalf("expected second newest record (timestamp 200), got %+v", response.Records)
	}
}

// ============================================================
// newTamperDetector tests
// ============================================================

func TestNewTamperDetector_NilMgr(t *testing.T) {
	s := &Server{}
	ctx := context.Background()
	detector, cleanup, err := s.newTamperDetector(ctx, "normal")
	if err != nil {
		t.Fatalf("expected no error with nil screenshotMgr, got %v", err)
	}
	if detector == nil {
		t.Fatal("expected non-nil detector")
	}
	if cleanup == nil {
		t.Fatal("expected non-nil cleanup")
	}
	cleanup()
}

// ============================================================
// tamperAllocatorFactory tests
// ============================================================

func TestTamperAllocatorFactory_NilMgr(t *testing.T) {
	s := &Server{}
	factory := s.tamperAllocatorFactory("")
	if factory != nil {
		t.Fatal("expected nil factory when screenshotMgr is nil")
	}
}

func TestTamperAllocatorFactory_WithProxy(t *testing.T) {
	s := &Server{config: &config.Config{}, screenshotMgr: &screenshot.Manager{}}
	factory := s.tamperAllocatorFactory("http://proxy:8080")
	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
	ctx, cancel, err := factory(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	cancel()
	_ = ctx
}

func TestTamperAllocatorFactory_NoProxy(t *testing.T) {
	s := &Server{config: &config.Config{}, screenshotMgr: &screenshot.Manager{}}
	factory := s.tamperAllocatorFactory("")
	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
	ctx, cancel, err := factory(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	cancel()
	_ = ctx
}
