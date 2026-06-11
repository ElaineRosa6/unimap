package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/unimap/project/internal/config"
)

func newServerForConfigTest() *Server {
	cfg := &config.Config{}
	cfg.ICP.Enabled = true
	cfg.ICP.BaseURL = "http://localhost:16181"
	cfg.ICP.APIKey = "abcd1234efgh5678"
	cfg.ICP.Timeout = 30
	cfg.ICP.DefaultType = "web"
	cfg.Engines.Fofa.APIKey = "secret-fofa-key"
	cfg.Engines.Fofa.Email = "user@example.com"
	cfg.Engines.Hunter.APIKey = "secret-hunter-key"
	cfg.Screenshot.Engine = "cdp"
	cfg.Screenshot.Mode = "auto"
	cfg.Screenshot.Timeout = 30
	cfg.System.MaxConcurrent = 10
	cfg.System.CacheTTL = 3600
	cfg.Web.CORS.AllowedOrigins = []string{"http://localhost:8448"}
	return &Server{config: cfg}
}

func TestHandleGetConfig_MasksSecrets(t *testing.T) {
	s := newServerForConfigTest()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w := httptest.NewRecorder()
	s.handleGetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}

	var out struct {
		ICP     map[string]interface{}            `json:"icp"`
		Engines map[string]map[string]interface{} `json:"engines"`
	}
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	icpKey, _ := out.ICP["api_key"].(string)
	if icpKey == "" || icpKey == "abcd1234efgh5678" {
		t.Fatalf("expected masked icp api_key, got %q", icpKey)
	}
	if !strings.Contains(icpKey, "****") {
		t.Fatalf("expected '****' in masked icp api_key, got %q", icpKey)
	}

	fofaKey, _ := out.Engines["fofa"]["api_key"].(string)
	if fofaKey == "secret-fofa-key" {
		t.Fatalf("fofa api_key should be masked, got %q", fofaKey)
	}
	if !strings.Contains(fofaKey, "****") {
		t.Fatalf("expected '****' in masked fofa api_key, got %q", fofaKey)
	}
}

func TestHandleGetConfig_RejectsNonGET(t *testing.T) {
	s := newServerForConfigTest()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config", nil)
	w := httptest.NewRecorder()
	s.handleGetConfig(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func postConfig(t *testing.T, s *Server, payload map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config", bytes.NewReader(body))
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://localhost:8448")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSaveConfig(w, req)
	return w
}

func TestHandleSaveConfig_MissingSection(t *testing.T) {
	s := newServerForConfigTest()
	w := postConfig(t, s, map[string]interface{}{"section": "", "data": map[string]interface{}{}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing_section") {
		t.Fatalf("expected missing_section in body, got %q", w.Body.String())
	}
}

func TestHandleSaveConfig_UnsupportedSection(t *testing.T) {
	s := newServerForConfigTest()
	w := postConfig(t, s, map[string]interface{}{"section": "bogus", "data": map[string]interface{}{}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "unsupported_section") {
		t.Fatalf("expected unsupported_section in body, got %q", w.Body.String())
	}
}

func TestHandleSaveConfig_ICPSection_UpdatesFields(t *testing.T) {
	s := newServerForConfigTest()
	originalKey := s.config.ICP.APIKey
	w := postConfig(t, s, map[string]interface{}{
		"section": "icp",
		"data": map[string]interface{}{
			"enabled":      false,
			"base_url":     "http://example:18888",
			"timeout":      60,
			"default_type": "app",
			"api_key":      "", // empty → should NOT overwrite real key
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}
	if s.config.ICP.Enabled {
		t.Fatalf("expected ICP.Enabled=false after save")
	}
	if s.config.ICP.BaseURL != "http://example:18888" {
		t.Fatalf("expected new base_url, got %q", s.config.ICP.BaseURL)
	}
	if s.config.ICP.Timeout != 60 {
		t.Fatalf("expected new timeout=60, got %d", s.config.ICP.Timeout)
	}
	if s.config.ICP.DefaultType != "app" {
		t.Fatalf("expected default_type=app, got %q", s.config.ICP.DefaultType)
	}
	if s.config.ICP.APIKey != originalKey {
		t.Fatalf("api_key was overwritten by empty value: now=%q want=%q", s.config.ICP.APIKey, originalKey)
	}
}

func TestHandleSaveConfig_ICPSection_RealAPIKeyUpdates(t *testing.T) {
	s := newServerForConfigTest()
	w := postConfig(t, s, map[string]interface{}{
		"section": "icp",
		"data":    map[string]interface{}{"api_key": "new-real-key"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if s.config.ICP.APIKey != "new-real-key" {
		t.Fatalf("expected api_key updated to new-real-key, got %q", s.config.ICP.APIKey)
	}
}

func TestHandleSaveConfig_ICPSection_IgnoresMaskedSecret(t *testing.T) {
	s := newServerForConfigTest()
	original := s.config.ICP.APIKey
	w := postConfig(t, s, map[string]interface{}{
		"section": "icp",
		"data":    map[string]interface{}{"api_key": "abcd****5678"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if s.config.ICP.APIKey != original {
		t.Fatalf("masked secret was incorrectly accepted: now=%q want=%q", s.config.ICP.APIKey, original)
	}
}

func TestHandleSaveConfig_ScreenshotSection(t *testing.T) {
	s := newServerForConfigTest()
	w := postConfig(t, s, map[string]interface{}{
		"section": "screenshot",
		"data":    map[string]interface{}{"engine": "extension", "mode": "auto", "timeout": 45},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}
	if s.config.Screenshot.Engine != "extension" {
		t.Fatalf("expected engine=extension, got %q", s.config.Screenshot.Engine)
	}
	if s.config.Screenshot.Timeout != 45 {
		t.Fatalf("expected timeout=45, got %d", s.config.Screenshot.Timeout)
	}
}

func TestHandleSaveConfig_SystemSection(t *testing.T) {
	s := newServerForConfigTest()
	w := postConfig(t, s, map[string]interface{}{
		"section": "system",
		"data":    map[string]interface{}{"max_concurrent": 50, "cache_ttl": 7200},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", w.Code, w.Body.String())
	}
	if s.config.System.MaxConcurrent != 50 {
		t.Fatalf("expected max_concurrent=50, got %d", s.config.System.MaxConcurrent)
	}
	if s.config.System.CacheTTL != 7200 {
		t.Fatalf("expected cache_ttl=7200, got %d", s.config.System.CacheTTL)
	}
}

func TestHandleSaveConfig_RejectsUntrustedOrigin(t *testing.T) {
	s := newServerForConfigTest()
	body, _ := json.Marshal(map[string]interface{}{"section": "icp", "data": map[string]interface{}{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config", bytes.NewReader(body))
	req.Host = "localhost:8448"
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSaveConfig(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for untrusted origin, got %d (body=%q)", w.Code, w.Body.String())
	}
}

func TestApplyICPSection_ZeroTimeoutKept(t *testing.T) {
	cfg := &config.Config{}
	cfg.ICP.Timeout = 30
	applyICPSection(cfg, map[string]interface{}{"timeout": float64(0)})
	if cfg.ICP.Timeout != 30 {
		t.Fatalf("expected timeout unchanged when set to 0, got %d", cfg.ICP.Timeout)
	}
}

func TestIsMaskedSecret(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"abc1234567890def", false},
		{"abc1****0def", true},  // matches maskAPIKey output (4+****+4)
		{"****", true},          // pure asterisks still counted as masked
		{"abc****def", false},   // 3-char prefix doesn't match maskAPIKey format
		{"mykey****real", false}, // real key containing **** should not be rejected
	}
	for _, tc := range tests {
		got := isMaskedSecret(tc.in)
		if got != tc.want {
			t.Errorf("isMaskedSecret(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
