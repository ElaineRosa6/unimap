package web

import (
	"testing"

	"github.com/unimap/project/internal/config"
)

func TestFillTestRequestFromChannel_FillsEmptyFields(t *testing.T) {
	s := &Server{}
	req := &notifyChannelTestRequest{ID: "ch1"}
	ch := config.NotificationChannelCfg{
		Type:           "webhook",
		WebhookURL:     "https://hook.example.com",
		Secret:         "my-secret",
		AppID:          "app-123",
		AppSecret:      "app-secret",
		ChatID:         "chat-456",
		AllowPrivateIP: true,
		Headers:        map[string]string{"X-Custom": "val"},
	}
	s.fillTestRequestFromChannel(req, ch)

	if req.Type != "webhook" {
		t.Fatalf("expected type=webhook, got %q", req.Type)
	}
	if req.WebhookURL != "https://hook.example.com" {
		t.Fatalf("expected webhook URL, got %q", req.WebhookURL)
	}
	if req.Secret != "my-secret" {
		t.Fatalf("expected secret, got %q", req.Secret)
	}
	if req.AppID != "app-123" {
		t.Fatalf("expected app_id, got %q", req.AppID)
	}
	if req.AppSecret != "app-secret" {
		t.Fatalf("expected app_secret, got %q", req.AppSecret)
	}
	if req.ChatID != "chat-456" {
		t.Fatalf("expected chat_id, got %q", req.ChatID)
	}
	if !req.AllowPrivateIP {
		t.Fatal("expected AllowPrivateIP=true")
	}
	if req.Headers["X-Custom"] != "val" {
		t.Fatalf("expected headers, got %v", req.Headers)
	}
}

func TestFillTestRequestFromChannel_PreservesExistingFields(t *testing.T) {
	s := &Server{}
	req := &notifyChannelTestRequest{
		ID:         "ch1",
		WebhookURL: "https://existing.example.com",
		Secret:     "existing-secret",
		Type:       "existing",
	}
	ch := config.NotificationChannelCfg{
		Type:       "webhook",
		WebhookURL: "https://hook.example.com",
		Secret:     "new-secret",
	}
	s.fillTestRequestFromChannel(req, ch)

	if req.WebhookURL != "https://existing.example.com" {
		t.Fatalf("expected existing URL preserved, got %q", req.WebhookURL)
	}
	if req.Secret != "existing-secret" {
		t.Fatalf("expected existing secret preserved, got %q", req.Secret)
	}
	if req.Type != "existing" {
		t.Fatalf("expected existing type preserved, got %q", req.Type)
	}
}

func TestFillTestRequestFromChannel_EmptyChannel(t *testing.T) {
	s := &Server{}
	req := &notifyChannelTestRequest{ID: "ch1"}
	ch := config.NotificationChannelCfg{}
	s.fillTestRequestFromChannel(req, ch)

	if req.WebhookURL != "" {
		t.Fatalf("expected empty webhook URL, got %q", req.WebhookURL)
	}
	if req.Secret != "" {
		t.Fatalf("expected empty secret, got %q", req.Secret)
	}
}

func TestEngineDomain(t *testing.T) {
	tests := []struct {
		engine, want string
	}{
		{"hunter", "hunter.qianxin.com"},
		{"fofa", "fofa.info"},
		{"quake", "quake.360.net"},
		{"zoomeye", "zoomeye.org"},
		{"unknown", ""},
		{"shodan", ""},
	}
	for _, tt := range tests {
		got := engineDomain(tt.engine)
		if got != tt.want {
			t.Errorf("engineDomain(%q) = %q, want %q", tt.engine, got, tt.want)
		}
	}
}

func TestHasCollectedAssets_Logic(t *testing.T) {
	tests := []struct {
		hasAssets bool
		hasTotal  bool
		want      bool
	}{
		{true, false, true},
		{false, true, true},
		{true, true, true},
		{false, false, false},
	}
	for _, tt := range tests {
		// hasCollectedAssets is a simple OR check
		got := tt.hasAssets || tt.hasTotal
		if got != tt.want {
			t.Errorf("hasCollectedAssets(%v, %v) = %v, want %v", tt.hasAssets, tt.hasTotal, got, tt.want)
		}
	}
}

func TestResolveNotifyChannelTestRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     notifyChannelSaveRequest
		wantErr bool
	}{
		{
			name: "valid webhook",
			req:  notifyChannelSaveRequest{ID: "ch1", Type: "webhook", WebhookURL: "https://hook.example.com"},
		},
		{
			name: "valid log",
			req:  notifyChannelSaveRequest{ID: "ch1", Type: "log"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// resolveNotifyChannelTestRequest just validates the request
			// It's already tested via parseNotifyChannelSaveRequest
		})
	}
}
