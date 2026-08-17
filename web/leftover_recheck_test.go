package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unimap/project/internal/config"
)

func TestLeftover_NoBackupRestoreRoute(t *testing.T) {
	r := NewRouter(&Server{})
	_ = r.RegisterRoutes()
	var sawCreate, sawList bool
	for _, route := range r.GetRoutes() {
		pattern := strings.ToLower(route.Pattern)
		if strings.Contains(pattern, "restore") {
			t.Fatalf("restore route is still absent leftover; unexpected route %s %s", route.Method, route.Pattern)
		}
		if route.Pattern == "/api/v1/backup/create" {
			sawCreate = true
		}
		if route.Pattern == "/api/v1/backup/list" {
			sawList = true
		}
	}
	if !sawCreate || !sawList {
		t.Fatalf("backup create/list routes missing: create=%v list=%v", sawCreate, sawList)
	}
}

func TestLeftover_HandleRegisterDoesNotPersistAdminToken(t *testing.T) {
	s := loopbackServer(newMockUserRepo())
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"username": "firstadmin", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	s.handleRegister(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := s.currentConfig().Web.Auth.AdminToken; got != "" {
		t.Fatalf("register leftover: first-admin path persisted admin_token=%q, want empty until lazy adminToken()", got)
	}
}

func TestLeftover_UnknownUsernameAfterBootstrapStill500(t *testing.T) {
	repo := newMockUserRepo()
	if _, err := repo.Create("admin", "hash", "admin"); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	cfg := &config.Config{}
	cfg.Web.Auth.Enabled = true
	cfg.Web.BindAddress = "127.0.0.1"
	s := &Server{config: cfg, userRepo: repo}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader("username=nobody&password=whatever&csrf_token=valid"))
	setTestRemoteAddr(req, "203.0.113.91:12345")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "valid"})
	s.handleLoginAPI(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("U-15 leftover: unknown username after bootstrap returned %d, want 500 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestLeftover_NonLoopbackEmptyDBRejectsAnonymousRegister(t *testing.T) {
	cfg := &config.Config{}
	cfg.Web.Auth.Enabled = true
	cfg.Web.BindAddress = "0.0.0.0"
	s := &Server{config: cfg, userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"username": "bootstrap", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	s.handleRegister(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("U-16 leftover: non-loopback anonymous register = %d, want 403", rec.Code)
	}
	if count, _ := s.userRepo.Count(); count != 0 {
		t.Fatalf("rejected register created %d users", count)
	}
}

func TestLeftover_SaveCookiesDoesNotReportLoginCheck(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{config: cfg}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cookies", strings.NewReader("cookie_quake=session=1"))
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleSaveCookies(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save cookies = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, leak := range []string{"login_status", "circuit", "recovery", "health"} {
		if strings.Contains(strings.ToLower(body), leak) {
			t.Fatalf("cookie save leftover changed: response mentioned %q: %s", leak, body)
		}
	}
}

func TestLeftover_QuotaUIStillStubbed(t *testing.T) {
	template := readTemplateContract(t, "quota.html")
	for _, want := range []string{
		`id="btn-quota-settings" disabled`,
		"配额自动刷新与告警设置尚未实现",
		`id="quota-trend-chart"`,
	} {
		if !strings.Contains(template, want) {
			t.Fatalf("quota leftover missing %q", want)
		}
	}
	script, err := os.ReadFile(filepath.Join("static", "js", "main.js"))
	if err != nil {
		t.Fatalf("read main.js: %v", err)
	}
	if !strings.Contains(string(script), "配额趋势图暂未实现") {
		t.Fatal("quota trend leftover text missing from shipped main.js")
	}
}

func TestLeftover_GUIEntryStillPresent(t *testing.T) {
	path := filepath.Join("..", "cmd", "unimap-gui", "main.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("GUI leftover/doc mismatch: cmd/unimap-gui/main.go missing: %v", err)
	}
	if !bytes.Contains(data, []byte("//go:build gui")) {
		t.Fatal("cmd/unimap-gui/main.go is not guarded by the gui build tag")
	}
}

func TestLeftover_GetConfigExposesNotificationSecrets(t *testing.T) {
	s := newServerForConfigTest()
	s.config.Notifications.Channels = []config.NotificationChannelCfg{{
		ID:         "ops",
		Type:       "webhook",
		Enabled:    true,
		WebhookURL: "https://example.invalid/hook/secret-token-value",
		Secret:     "notify-signing-secret-value",
	}}
	req := withAdminContext(httptest.NewRequest(http.MethodGet, "/api/v1/config", nil))
	w := httptest.NewRecorder()
	s.handleGetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get config = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "secret-token-value") && !strings.Contains(body, "notify-signing-secret-value") {
		t.Fatal("expected GET /api/v1/config leftover to include raw notification webhook or secret")
	}
}

func TestLeftover_CDPExtractorsExistForUngradedEngines(t *testing.T) {
	path := filepath.Join("..", "internal", "screenshot", "dom_selectors.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dom_selectors.go: %v", err)
	}
	src := string(data)
	for _, want := range []string{"extractFofaJS", "extractZoomEyeJS", "extractShodanJS"} {
		if !strings.Contains(src, want) {
			t.Fatalf("CDP leftover: shipped extractor %s missing", want)
		}
	}
}
