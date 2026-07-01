package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unimap/project/internal/config"
)

func TestParseNotifyChannelSaveRequest_EmptyBody(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader([]byte("")))
	_, ok := parseNotifyChannelSaveRequest(rec, r)
	if ok {
		t.Fatal("expected false for empty body")
	}
}

func TestParseNotifyChannelSaveRequest_InvalidJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader([]byte("not json")))
	_, ok := parseNotifyChannelSaveRequest(rec, r)
	if ok {
		t.Fatal("expected false for invalid JSON")
	}
}

func TestParseNotifyChannelSaveRequest_MissingID(t *testing.T) {
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"type": "webhook"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader(body))
	_, ok := parseNotifyChannelSaveRequest(rec, r)
	if ok {
		t.Fatal("expected false for missing ID")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestParseNotifyChannelSaveRequest_MissingType(t *testing.T) {
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "ch1"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader(body))
	_, ok := parseNotifyChannelSaveRequest(rec, r)
	if ok {
		t.Fatal("expected false for missing type")
	}
}

func TestParseNotifyChannelSaveRequest_InvalidType(t *testing.T) {
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "ch1", "type": "unknown"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader(body))
	_, ok := parseNotifyChannelSaveRequest(rec, r)
	if ok {
		t.Fatal("expected false for invalid type")
	}
}

func TestParseNotifyChannelSaveRequest_FeishuApp_MissingParams(t *testing.T) {
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "ch1", "type": "feishu_app"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader(body))
	_, ok := parseNotifyChannelSaveRequest(rec, r)
	if ok {
		t.Fatal("expected false for feishu_app missing params")
	}
}

func TestParseNotifyChannelSaveRequest_FeishuApp_OK(t *testing.T) {
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"id":         "ch1",
		"type":       "feishu_app",
		"app_id":     "aid",
		"app_secret": "secret",
		"chat_id":    "cid",
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader(body))
	req, ok := parseNotifyChannelSaveRequest(rec, r)
	if !ok {
		t.Fatal("expected true for valid feishu_app")
	}
	if req.ID != "ch1" || req.Type != "feishu_app" {
		t.Fatalf("unexpected req: %+v", req)
	}
}

func TestParseNotifyChannelSaveRequest_Log_NoURL(t *testing.T) {
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "ch1", "type": "log"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader(body))
	_, ok := parseNotifyChannelSaveRequest(rec, r)
	if !ok {
		t.Fatal("expected true for log type (no webhook_url required)")
	}
}

func TestParseNotifyChannelSaveRequest_Webhook_MissingURL(t *testing.T) {
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "ch1", "type": "webhook"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader(body))
	_, ok := parseNotifyChannelSaveRequest(rec, r)
	if ok {
		t.Fatal("expected false for webhook missing URL")
	}
}

func TestParseNotifyChannelSaveRequest_Webhook_OK(t *testing.T) {
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"id":          "ch1",
		"type":        "webhook",
		"webhook_url": "https://hook.example.com",
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader(body))
	req, ok := parseNotifyChannelSaveRequest(rec, r)
	if !ok {
		t.Fatal("expected true for valid webhook")
	}
	if req.WebhookURL != "https://hook.example.com" {
		t.Fatalf("unexpected webhook_url: %q", req.WebhookURL)
	}
}

func TestParseNotifyChannelSaveRequest_TrimSpaces(t *testing.T) {
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"id":          "  ch1  ",
		"type":        "  webhook  ",
		"webhook_url": "  https://hook.example.com  ",
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", bytes.NewReader(body))
	req, ok := parseNotifyChannelSaveRequest(rec, r)
	if !ok {
		t.Fatal("expected true")
	}
	if req.ID != "ch1" || req.Type != "webhook" || req.WebhookURL != "https://hook.example.com" {
		t.Fatalf("expected trimmed values, got: %+v", req)
	}
}

func TestUpsertNotifyChannel_Insert(t *testing.T) {
	s := &Server{config: &config.Config{}}
	req := notifyChannelSaveRequest{
		ID:         "ch1",
		Type:       "webhook",
		Enabled:    true,
		WebhookURL: "https://hook.example.com",
		Headers:    map[string]string{"X-Custom": "val"},
	}
	s.upsertNotifyChannel(req)
	if len(s.config.Notifications.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(s.config.Notifications.Channels))
	}
	ch := s.config.Notifications.Channels[0]
	if ch.ID != "ch1" || ch.Type != "webhook" || ch.WebhookURL != "https://hook.example.com" {
		t.Fatalf("unexpected channel: %+v", ch)
	}
	if ch.Headers["X-Custom"] != "val" {
		t.Fatal("expected headers to be set")
	}
}

func TestUpsertNotifyChannel_Update(t *testing.T) {
	s := &Server{config: &config.Config{}}
	s.config.Notifications.Channels = []config.NotificationChannelCfg{
		{ID: "ch1", Type: "webhook", WebhookURL: "https://old.example.com", Secret: "old-secret"},
	}
	req := notifyChannelSaveRequest{
		ID:         "ch1",
		Type:       "webhook",
		WebhookURL: "https://new.example.com",
	}
	s.upsertNotifyChannel(req)
	if len(s.config.Notifications.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(s.config.Notifications.Channels))
	}
	ch := s.config.Notifications.Channels[0]
	if ch.WebhookURL != "https://new.example.com" {
		t.Fatalf("expected new URL, got %q", ch.WebhookURL)
	}
	if ch.Secret != "old-secret" {
		t.Fatal("expected old secret preserved")
	}
}

func TestUpsertNotifyChannel_UpdatePreservesSecret(t *testing.T) {
	s := &Server{config: &config.Config{}}
	s.config.Notifications.Channels = []config.NotificationChannelCfg{
		{ID: "ch1", Type: "webhook", Secret: "keep-this", AppSecret: "keep-app"},
	}
	req := notifyChannelSaveRequest{
		ID:         "ch1",
		Type:       "webhook",
		Secret:     "", // empty → preserve old
		AppSecret:  "", // empty → preserve old
		WebhookURL: "https://new.example.com",
	}
	s.upsertNotifyChannel(req)
	ch := s.config.Notifications.Channels[0]
	if ch.Secret != "keep-this" {
		t.Fatalf("expected secret preserved, got %q", ch.Secret)
	}
	if ch.AppSecret != "keep-app" {
		t.Fatalf("expected app_secret preserved, got %q", ch.AppSecret)
	}
}

func TestUpsertNotifyChannel_MultipleChannels(t *testing.T) {
	s := &Server{config: &config.Config{}}
	s.upsertNotifyChannel(notifyChannelSaveRequest{ID: "ch1", Type: "webhook", WebhookURL: "https://a.com"})
	s.upsertNotifyChannel(notifyChannelSaveRequest{ID: "ch2", Type: "log"})
	if len(s.config.Notifications.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(s.config.Notifications.Channels))
	}
}

func TestHandleNotifyChannelSave_MethodNotAllowed(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/channels", nil)
	s.handleNotifyChannelSave(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleNotifyChannelSave_ConfigNil(t *testing.T) {
	s := &Server{config: nil}
	rec := httptest.NewRecorder()
	body := `{"id":"ch1","type":"webhook","webhook_url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleNotifyChannelSave(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleNotifyChannelSave_Success(t *testing.T) {
	cfg := &config.Config{}
	tmpDir := t.TempDir()
	cfgManager := config.NewManager(filepath.Join(tmpDir, "config.yaml"))
	cfgManager.SetConfig(cfg)
	s := &Server{config: cfg, configManager: cfgManager}
	rec := httptest.NewRecorder()
	body := `{"id":"ch1","type":"webhook","webhook_url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleNotifyChannelSave(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["success"] != true {
		t.Fatal("expected success=true")
	}
}

func TestHandleNotifyChannelSave_SSRFBlocked(t *testing.T) {
	cfg := &config.Config{}
	tmpDir := t.TempDir()
	cfgManager := config.NewManager(filepath.Join(tmpDir, "config.yaml"))
	cfgManager.SetConfig(cfg)
	s := &Server{config: cfg, configManager: cfgManager}
	rec := httptest.NewRecorder()
	body := `{"id":"ch1","type":"webhook","webhook_url":"http://127.0.0.1:8080/webhook"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleNotifyChannelSave(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for SSRF blocked, got %d", rec.Code)
	}
}

func TestHandleNotifyChannelDelete_MethodNotAllowed(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/channels/ch1", nil)
	s.handleNotifyChannelDelete(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleNotifyChannelTest_MethodNotAllowed(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/channels/ch1/test", nil)
	s.handleNotifyChannelTest(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleNotificationChannels_ConfigNil(t *testing.T) {
	s := &Server{config: nil}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/channels", nil)
	s.handleNotificationChannels(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleNotificationChannels_Success(t *testing.T) {
	cfg := &config.Config{}
	cfg.Notifications.Channels = []config.NotificationChannelCfg{
		{ID: "ch1", Type: "webhook", Enabled: true},
		{ID: "ch2", Type: "log", Enabled: false},
	}
	s := &Server{config: cfg}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/channels", nil)
	s.handleNotificationChannels(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Channels []struct {
				ID      string `json:"id"`
				Type    string `json:"type"`
				Enabled bool   `json:"enabled"`
			} `json:"channels"`
		} `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(resp.Data.Channels))
	}
}

func TestHandleNotifyReload_MethodNotAllowed(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/reload", nil)
	s.handleNotifyReload(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleNotifyReload_ConfigManagerNil(t *testing.T) {
	s := &Server{configManager: nil}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/reload", nil)
	s.handleNotifyReload(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleNotifyReload_Success(t *testing.T) {
	cfg := &config.Config{}
	cfgManager := config.NewManager("")
	cfgManager.SetConfig(cfg)
	s := &Server{config: cfg, configManager: cfgManager}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/reload", nil)
	s.handleNotifyReload(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["success"] != true {
		t.Fatal("expected success=true")
	}
}

// ============================================================
// resolveNotifyChannelTestRequest tests
// ============================================================

func TestResolveNotifyChannelTestRequest_InvalidJSON(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/test", strings.NewReader("not json"))
	_, ok := s.resolveNotifyChannelTestRequest(rec, req)
	if ok {
		t.Fatal("expected false for invalid JSON")
	}
}

func TestResolveNotifyChannelTestRequest_MissingType(t *testing.T) {
	s := &Server{config: &config.Config{}}
	rec := httptest.NewRecorder()
	body := `{"id":"ch1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/test", strings.NewReader(body))
	_, ok := s.resolveNotifyChannelTestRequest(rec, req)
	if ok {
		t.Fatal("expected false for missing type")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestResolveNotifyChannelTestRequest_FeishuAppMissingParams(t *testing.T) {
	s := &Server{config: &config.Config{}}
	rec := httptest.NewRecorder()
	body := `{"id":"ch1","type":"feishu_app"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/test", strings.NewReader(body))
	_, ok := s.resolveNotifyChannelTestRequest(rec, req)
	if ok {
		t.Fatal("expected false for missing feishu_app params")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestResolveNotifyChannelTestRequest_MissingWebhookURL(t *testing.T) {
	s := &Server{config: &config.Config{}}
	rec := httptest.NewRecorder()
	body := `{"id":"ch1","type":"webhook"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/test", strings.NewReader(body))
	_, ok := s.resolveNotifyChannelTestRequest(rec, req)
	if ok {
		t.Fatal("expected false for missing webhook_url")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestResolveNotifyChannelTestRequest_Success(t *testing.T) {
	s := &Server{config: &config.Config{}}
	rec := httptest.NewRecorder()
	body := `{"id":"ch1","type":"webhook","webhook_url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/test", strings.NewReader(body))
	result, ok := s.resolveNotifyChannelTestRequest(rec, req)
	if !ok {
		t.Fatal("expected true")
	}
	if result.Type != "webhook" {
		t.Fatalf("expected type webhook, got %s", result.Type)
	}
	if result.WebhookURL != "https://example.com" {
		t.Fatalf("expected webhook_url https://example.com, got %s", result.WebhookURL)
	}
}

// ============================================================
// handleNotifyChannelDelete tests
// ============================================================

func TestHandleNotifyChannelDelete_ConfigNil(t *testing.T) {
	s := &Server{config: nil}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/notifications/channels/ch1", nil)
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleNotifyChannelDelete(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleNotifyChannelDelete_MissingID(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{config: cfg, configManager: &config.Manager{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/notifications/channels", nil)
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleNotifyChannelDelete(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleNotifyChannelDelete_NotFound(t *testing.T) {
	cfg := &config.Config{}
	cfg.Notifications.Channels = []config.NotificationChannelCfg{
		{ID: "ch1", Type: "webhook"},
	}
	s := &Server{config: cfg, configManager: &config.Manager{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/notifications/channels?id=nonexistent", nil)
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleNotifyChannelDelete(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleNotifyChannelDelete_Success(t *testing.T) {
	cfg := &config.Config{}
	cfg.Notifications.Channels = []config.NotificationChannelCfg{
		{ID: "ch1", Type: "webhook"},
	}
	tmpDir := t.TempDir()
	cfgManager := config.NewManager(filepath.Join(tmpDir, "config.yaml"))
	cfgManager.SetConfig(cfg)
	s := &Server{config: cfg, configManager: cfgManager}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/notifications/channels?id=ch1", nil)
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleNotifyChannelDelete(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(cfg.Notifications.Channels) != 0 {
		t.Fatalf("expected 0 channels, got %d", len(cfg.Notifications.Channels))
	}
}

// ============================================================
// handleNotifyChannelTest additional tests
// ============================================================

func TestHandleNotifyChannelTest_SendError(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{config: cfg}
	rec := httptest.NewRecorder()
	body := `{"id":"ch1","type":"webhook","webhook_url":"https://invalid.example.test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleNotifyChannelTest(rec, req)
	// sendTestNotification will fail because webhook URL is unreachable
	if rec.Code != http.StatusBadGateway && rec.Code != http.StatusOK {
		t.Fatalf("expected 502 or 200, got %d", rec.Code)
	}
}

// ============================================================
// sendTestNotification tests
// ============================================================

func TestSendTestNotification_InvalidChannelType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/test", nil)
	testReq := notifyChannelTestRequest{
		ID:   "ch1",
		Type: "invalid_type_that_does_not_exist",
	}
	err := sendTestNotification(req, testReq)
	if err == nil {
		t.Fatal("expected error for invalid channel type")
	}
}

func TestSendTestNotification_WebhookUnreachable(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/test", nil)
	testReq := notifyChannelTestRequest{
		ID:         "ch1",
		Type:       "webhook",
		WebhookURL: "https://invalid.example.test:99999",
	}
	err := sendTestNotification(req, testReq)
	if err == nil {
		t.Fatal("expected error for unreachable webhook")
	}
}
