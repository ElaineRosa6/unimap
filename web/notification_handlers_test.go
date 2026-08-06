package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		ID:          "ch1",
		Type:        "webhook",
		Enabled:     true,
		WebhookURL:  "https://hook.example.com",
		Headers:     map[string]string{"X-Custom": "val"},
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
		ID:          "ch1",
		Type:        "webhook",
		Secret:      "", // empty → preserve old
		AppSecret:   "", // empty → preserve old
		WebhookURL:  "https://new.example.com",
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
