package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/screenshot"
)

// ============================================================
// handleImportCookieJSON tests (supplementary, non-duplicate)
// ============================================================

func TestHandleImportCookieJSON_MissingEngine(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{config: cfg}
	body := strings.NewReader("engine=&cookie_json=[]")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cookies", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.handleImportCookieJSON(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid_request") {
		t.Fatalf("expected 'invalid_request' in body, got %q", w.Body.String())
	}
}

func TestHandleImportCookieJSON_InvalidJSON(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{config: cfg}
	body := strings.NewReader("engine=fofa&cookie_json=not-json")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cookies", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.handleImportCookieJSON(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid_cookie_json") {
		t.Fatalf("expected 'invalid_cookie_json' in body, got %q", w.Body.String())
	}
}

func TestHandleImportCookieJSON_EmptyCookieSet(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{config: cfg}
	body := strings.NewReader("engine=fofa&cookie_json=[]")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cookies", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.handleImportCookieJSON(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "empty_cookie_set") {
		t.Fatalf("expected 'empty_cookie_set' in body, got %q", w.Body.String())
	}
}

func TestHandleImportCookieJSON_UnsupportedEngine(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{config: cfg}
	body := strings.NewReader(`engine=unknown&cookie_json=[{"name":"test","value":"val","domain":".example.com"}]`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cookies", body)
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.handleImportCookieJSON(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "unsupported_engine") {
		t.Fatalf("expected 'unsupported_engine' in body, got %q", w.Body.String())
	}
}

// ============================================================
// applyCookiesFromRequest tests
// ============================================================

func TestApplyCookiesFromRequest_NilConfig(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Should not panic with nil config
	s.applyCookiesFromRequest(req)
}

// ============================================================
// parseEnginesParam supplementary tests
// ============================================================

func TestParseEnginesParam_WhitespaceTrimmed(t *testing.T) {
	u := "/?engines=" + url.QueryEscape(" fofa , hunter ")
	req := httptest.NewRequest(http.MethodGet, u, nil)
	got := parseEnginesParam(req)
	for _, e := range got {
		if e != strings.TrimSpace(e) {
			t.Errorf("engine %q should be trimmed", e)
		}
	}
}

func TestParseEnginesParam_EmptyEntriesRemoved(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?engines=fofa,,hunter,", nil)
	got := parseEnginesParam(req)
	for _, e := range got {
		if e == "" {
			t.Fatal("expected empty entries to be removed")
		}
	}
}

// ============================================================
// validateQueryInput supplementary tests
// ============================================================

func TestValidateQueryInput_Empty(t *testing.T) {
	err := validateQueryInput("")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestValidateQueryInput_Valid(t *testing.T) {
	err := validateQueryInput("port:80")
	if err != nil {
		t.Fatalf("expected no error for valid query, got %v", err)
	}
}

// ============================================================
// cookiesToHeader tests
// ============================================================

func TestCookiesToHeader_Empty(t *testing.T) {
	got := cookiesToHeader(nil)
	if got != "" {
		t.Fatalf("expected empty string for nil cookies, got %q", got)
	}
}

// ============================================================
// convertConfigCookies tests
// ============================================================

func TestConvertConfigCookies_Nil(t *testing.T) {
	got := convertConfigCookies(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty slice for nil, got %d cookies", len(got))
	}
}

func TestConvertConfigCookies_Empty(t *testing.T) {
	got := convertConfigCookies([]config.Cookie{})
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d cookies", len(got))
	}
}

func TestConvertConfigCookies_Single(t *testing.T) {
	input := []config.Cookie{
		{Name: "session", Value: "abc123", Domain: ".example.com", Path: "/", Secure: true},
	}
	got := convertConfigCookies(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(got))
	}
	if got[0].Name != "session" || got[0].Value != "abc123" {
		t.Fatalf("unexpected cookie: %+v", got[0])
	}
}

// ============================================================
// handleVerifyCookies supplementary tests
// ============================================================

func TestHandleVerifyCookies_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cookies/verify", nil)
	w := httptest.NewRecorder()
	s.handleVerifyCookies(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleVerifyCookies_InvalidQuery(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{config: cfg}
	body := strings.NewReader("query=")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cookies/verify", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.handleVerifyCookies(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid_query") {
		t.Fatalf("expected 'invalid_query' in body, got %q", w.Body.String())
	}
}

// ============================================================
// handleCookieLoginStatus tests
// ============================================================

func TestHandleCookieLoginStatus_Success(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cookies/login-status", nil)
	w := httptest.NewRecorder()
	s.handleCookieLoginStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("expected 'success':true in body, got %q", w.Body.String())
	}
}

func TestHandleCookieLoginStatus_WrongMethod(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cookies/login-status", nil)
	w := httptest.NewRecorder()
	s.handleCookieLoginStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

type loginStatusMockBridge struct{}

func (m *loginStatusMockBridge) SubmitTask(ctx context.Context, task screenshot.BridgeTask) error {
	return nil
}
func (m *loginStatusMockBridge) AwaitResult(ctx context.Context, requestID string) (screenshot.BridgeResult, error) {
	return screenshot.BridgeResult{RequestID: requestID, Success: true}, nil
}

func TestHandleCookieLoginStatus_ExtPaired_NotLoggedIn(t *testing.T) {
	svc := screenshot.NewBridgeService(&loginStatusMockBridge{}, 1, 5*time.Second)
	now := time.Now().Unix()
	s := &Server{
		bridge: &BridgeState{
			Service: svc,
			Tokens: map[string]int64{
				"token1": now + 300, // live token
			},
			LastSeen: map[string]int64{
				"token1": now, // seen just now
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cookies/login-status", nil)
	w := httptest.NewRecorder()
	s.handleCookieLoginStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	data, ok := resp["engines"].([]interface{})
	if !ok || len(data) == 0 {
		t.Fatalf("expected non-empty engines array, got: %v", resp["engines"])
	}
	first := data[0].(map[string]interface{})
	if first["logged_in"] != false {
		t.Errorf("expected logged_in=false for ext_paired, got %v", first["logged_in"])
	}
	if first["reason"] != "ext_connected" {
		t.Errorf("expected reason=ext_connected, got %v", first["reason"])
	}
	if first["ext_paired"] != true {
		t.Errorf("expected ext_paired=true, got %v", first["ext_paired"])
	}
}

// ============================================================
// hasCookies tests
// ============================================================

func TestHasCookies_Nil(t *testing.T) {
	if hasCookies(nil) {
		t.Fatal("expected false for nil cookies")
	}
}

func TestHasCookies_Empty(t *testing.T) {
	if hasCookies([]config.Cookie{}) {
		t.Fatal("expected false for empty cookies")
	}
}

func TestHasCookies_WithName(t *testing.T) {
	cookies := []config.Cookie{{Name: "session", Value: "abc"}}
	if !hasCookies(cookies) {
		t.Fatal("expected true when cookie name is present")
	}
}

func TestHasCookies_AllEmptyNames(t *testing.T) {
	cookies := []config.Cookie{{Name: "", Value: "abc"}}
	if hasCookies(cookies) {
		t.Fatal("expected false when all names are empty")
	}
}

func TestEngineCookiesSupportsEveryBrowserEngine(t *testing.T) {
	engines := []string{"fofa", "hunter", "zoomeye", "quake", "shodan", "censys", "daydaymap"}
	for _, engine := range engines {
		t.Run(engine, func(t *testing.T) {
			cfg := &config.Config{}
			want := []config.Cookie{{Name: "session", Value: "fixture", Domain: ".example.test", Path: "/"}}
			setEngineCookies(cfg, engine, want)
			got := engineCookies(cfg, engine)
			if len(got) != 1 || got[0].Name != "session" {
				t.Fatalf("cookies for %s were not stored: %#v", engine, got)
			}
		})
	}
}

func TestEngineCredentialURLUsesReachableCanonicalOrigins(t *testing.T) {
	if got := engineCredentialURL("censys"); got != "https://platform.censys.io/" {
		t.Fatalf("censys credential URL=%q", got)
	}
	if got := engineCredentialURL("daydaymap"); got != "https://www.daydaymap.com/home" {
		t.Fatalf("daydaymap credential URL=%q", got)
	}
}

func TestBridgeCookiesFromResultPreservesCookieAttributes(t *testing.T) {
	result := screenshot.BridgeResult{StructuredCollectedData: &model.BridgeCollectedData{
		Cookies: []model.BrowserCookie{{
			Name: "session", Value: "fixture", Domain: ".fofa.info", Path: "/",
			HTTPOnly: true, Secure: true,
		}},
	}}

	cookies, err := bridgeCookiesFromResult("fofa", result)
	if err != nil {
		t.Fatalf("bridgeCookiesFromResult: %v", err)
	}
	if len(cookies) != 1 || !cookies[0].HTTPOnly || !cookies[0].Secure || cookies[0].Value != "fixture" {
		t.Fatalf("cookie attributes were not preserved: %#v", cookies)
	}
}

func TestBridgeCollectedDataKeepsBrowserStorageTyped(t *testing.T) {
	data := model.BridgeCollectedData{Storage: &model.BrowserStorage{
		Local: map[string]string{"token": "fixture"}, Session: map[string]string{"nonce": "one"},
	}}
	if data.Storage.Local["token"] != "fixture" || data.Storage.Session["nonce"] != "one" {
		t.Fatalf("typed browser storage was lost: %#v", data.Storage)
	}
}

// ============================================================
// verifyEngineSession unsupported engine test
// ============================================================

func TestVerifyEngineSession_UnknownEngine(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{config: cfg}
	ok, _, hint, err := s.verifyEngineSession(context.TODO(), "cdp", "unknown_engine", "test")
	if ok {
		t.Fatal("expected false for unknown engine")
	}
	// CDP mode returns specific hints for missing cookies vs unsupported engine
	if err == nil {
		t.Fatal("expected error for unknown engine")
	}
	_ = hint // hint varies by implementation
}

// ============================================================
// activeBridgeLiveTokens / extPaired contract tests
// ============================================================

func TestActiveBridgeLiveTokens_StaleLastSeen_Zero(t *testing.T) {
	now := time.Now().Unix()
	s := &Server{
		bridge: &BridgeState{
			Tokens:   map[string]int64{"token1": now + 300},
			LastSeen: map[string]int64{"token1": now - 70}, // stale (>60s window)
		},
	}
	if got := s.activeBridgeLiveTokens(); got != 0 {
		t.Fatalf("expected 0 live tokens with stale LastSeen, got %d", got)
	}
}

func TestActiveBridgeLiveTokens_RecentLastSeen_One(t *testing.T) {
	now := time.Now().Unix()
	s := &Server{
		bridge: &BridgeState{
			Tokens:   map[string]int64{"token1": now + 300},
			LastSeen: map[string]int64{"token1": now},
		},
	}
	if got := s.activeBridgeLiveTokens(); got != 1 {
		t.Fatalf("expected 1 live token with recent LastSeen, got %d", got)
	}
}

// ============================================================
// handleImportCookieJSON tests
// ============================================================

func TestHandleImportCookieJSON_MethodNotAllowed(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cookie/import", nil)
	w := httptest.NewRecorder()
	s.handleImportCookieJSON(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleImportCookieJSON_ConfigNil(t *testing.T) {
	s := &Server{config: nil}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cookie/import", nil)
	req.Header.Set("Origin", "http://localhost:8448")
	w := httptest.NewRecorder()
	s.handleImportCookieJSON(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// ============================================================
// judgeLoginByCookieNames tests
// ============================================================

func TestJudgeLoginByCookieNames_Shodan(t *testing.T) {
	if !judgeLoginByCookieNames("shodan", map[string]string{"dotcom_user": "admin"}) {
		t.Error("expected logged_in for shodan with dotcom_user cookie")
	}
	if judgeLoginByCookieNames("shodan", map[string]string{}) {
		t.Error("expected not logged_in for shodan with empty cookies")
	}
}

func TestJudgeLoginByCookieNames_Censys(t *testing.T) {
	// Censys is API-key based; no browser session cookie marker.
	if judgeLoginByCookieNames("censys", map[string]string{"anything": "value"}) {
		t.Error("expected not logged_in for censys (API-key only)")
	}
}

func TestJudgeLoginByCookieNames_DayDayMap(t *testing.T) {
	// DayDayMap is API-key based; no browser session cookie marker.
	if judgeLoginByCookieNames("daydaymap", map[string]string{"anything": "value"}) {
		t.Error("expected not logged_in for daydaymap (API-key only)")
	}
}
