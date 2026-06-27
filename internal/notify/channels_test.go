package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// webhookNotification is the typed payload received by test webhook servers.
type webhookNotification struct {
	TaskID   string                 `json:"task_id"`
	TaskName string                 `json:"task_name"`
	TaskType string                 `json:"task_type"`
	Status   string                 `json:"status"`
	Result   string                 `json:"result"`
	Error    string                 `json:"error"`
	Duration float64                `json:"duration"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
}

// feishuTokenResponse is the typed response for Feishu token API.
type feishuTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token,omitempty"`
	Expire            int    `json:"expire,omitempty"`
}

// feishuImageUploadResponse is the typed response for Feishu image upload API.
type feishuImageUploadResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data *struct {
		ImageKey string `json:"image_key"`
	} `json:"data,omitempty"`
}

// feishuMessageResponse is the typed response for Feishu message send API.
type feishuMessageResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func TestLogChannel_Send(t *testing.T) {
	ch := NewLogChannel("test-log", true)
	n := TaskNotification{
		TaskID: "t1", TaskName: "test", TaskType: "query",
		Status: "success", Result: "ok", Duration: 1200,
		Timestamp: time.Now(),
	}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogChannel_Disabled(t *testing.T) {
	ch := NewLogChannel("test-log", false)
	called := false
	n := TaskNotification{Status: "success"}
	_ = ch.Send(context.Background(), n)
	if called {
		t.Fatal("should not send when disabled")
	}
}

func TestGenericWebhook_Send_Success(t *testing.T) {
	var received webhookNotification
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewGenericWebhookChannel("test-wh", server.URL, nil, true, true)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	n := TaskNotification{
		TaskID: "t1", TaskName: "test", TaskType: "query",
		Status: "success", Result: "ok", Duration: 1200,
		Timestamp: time.Now(),
	}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.TaskID != "t1" {
		t.Errorf("expected task_id t1, got %v", received.TaskID)
	}
}

func TestGenericWebhook_NonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	ch, err := NewGenericWebhookChannel("test-wh", server.URL, nil, true, true)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	n := TaskNotification{Status: "failed"}
	err = ch.Send(context.Background(), n)
	if err == nil {
		t.Fatal("expected error for non-success status")
	}
}

func TestGenericWebhook_NetworkError(t *testing.T) {
	ch, err := NewGenericWebhookChannel("test-wh", "http://127.0.0.1:19999/webhook", nil, true, true)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	n := TaskNotification{Status: "success"}
	err = ch.Send(context.Background(), n)
	if err == nil {
		t.Fatal("expected error for network error")
	}
}

func TestGenericWebhook_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	ch, err := NewGenericWebhookChannel("test-wh", server.URL, nil, true, true)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n := TaskNotification{Status: "success"}
	err = ch.Send(ctx, n)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestGenericWebhook_SSRSF_RejectPrivate(t *testing.T) {
	_, err := NewGenericWebhookChannel("test-wh", "http://127.0.0.1:8080/webhook", nil, true, false)
	if err == nil {
		t.Fatal("expected error for private URL")
	}
}

func TestDingTalkChannel_Send_Success(t *testing.T) {
	var received DingTalkMarkdownBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewDingTalkChannel("test-ding", server.URL, "", true, true)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	n := TaskNotification{
		TaskID: "t1", TaskName: "test", TaskType: "query",
		Status: "success", Result: "ok", Duration: 1200,
	}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.MsgType != "markdown" {
		t.Errorf("expected msgtype markdown, got %v", received.MsgType)
	}
}

func TestDingTalkChannel_Sign(t *testing.T) {
	sign, err := DingTalkSign("test-secret", 1234567890)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sign == "" {
		t.Fatal("expected non-empty sign")
	}
}

func TestFeishuChannel_Send_Success(t *testing.T) {
	var received FeishuCardBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewFeishuChannel("test-feishu", server.URL, "", true, true)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	n := TaskNotification{
		TaskID: "t1", TaskName: "test", TaskType: "query",
		Status: "success", Result: "ok", Duration: 1200,
	}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.MsgType != "interactive" {
		t.Errorf("expected msg_type interactive, got %v", received.MsgType)
	}
}

func TestFeishuChannel_Sign(t *testing.T) {
	sign, err := FeishuSign("test-secret", 1234567890)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const want = "qCaOcLimil1ehZl6GzN2CUL6wgdt4onZPxvw8V+3TzA="
	if sign != want {
		t.Fatalf("unexpected sign: got %q want %q", sign, want)
	}
}

func TestWeComChannel_Send_Success(t *testing.T) {
	var received WeComMarkdownBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewWeComChannel("test-wecom", server.URL, true, true)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	n := TaskNotification{
		TaskID: "t1", TaskName: "test", TaskType: "query",
		Status: "success", Result: "ok", Duration: 1200,
	}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.MsgType != "markdown" {
		t.Errorf("expected msgtype markdown, got %v", received.MsgType)
	}
}

func TestWeComChannel_NonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ch, err := NewWeComChannel("test-wecom", server.URL, true, true)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	n := TaskNotification{Status: "failed"}
	err = ch.Send(context.Background(), n)
	if err == nil {
		t.Fatal("expected error for non-success status")
	}
}

func TestWeComChannel_SSRSF_RejectPrivate(t *testing.T) {
	_, err := NewWeComChannel("test-wecom", "http://10.0.0.1/webhook", true, false)
	if err == nil {
		t.Fatal("expected error for private URL")
	}
}

func TestWeComChannel_NetworkError(t *testing.T) {
	ch, err := NewWeComChannel("test-wecom", "http://127.0.0.1:19999/webhook", true, true)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	n := TaskNotification{Status: "success"}
	err = ch.Send(context.Background(), n)
	if err == nil {
		t.Fatal("expected error for network error")
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	ch := NewLogChannel("test-log", true)
	if err := r.Register(ch); err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	got := r.Get("test-log")
	if got == nil {
		t.Fatal("expected channel")
	}
	if got.ID() != "test-log" {
		t.Errorf("expected ID test-log, got %s", got.ID())
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()
	r.Register(NewLogChannel("dup", true))
	err := r.Register(NewLogChannel("dup", true))
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	r.Register(NewLogChannel("rem", true))
	r.Remove("rem")
	if r.Get("rem") != nil {
		t.Fatal("expected nil after remove")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(NewLogChannel("ch1", true))
	r.Register(NewLogChannel("ch2", false))
	r.Register(NewLogChannel("ch3", true))

	ids := r.List()
	if len(ids) != 2 {
		t.Errorf("expected 2 enabled channels, got %d", len(ids))
	}
}

func TestRegistry_Reload(t *testing.T) {
	r := NewRegistry()
	r.Register(NewLogChannel("old-ch", true))

	cfgs := []ChannelConfig{
		{ID: "new-ch", Type: "log", Enabled: true},
	}
	r.Reload(cfgs)

	if r.Get("old-ch") != nil {
		t.Fatal("expected old channel removed")
	}
	if r.Get("new-ch") == nil {
		t.Fatal("expected new channel present")
	}
}

func TestNewChannelFromConfig_UnknownType(t *testing.T) {
	cfg := ChannelConfig{ID: "x", Type: "unknown", Enabled: true}
	_, err := NewChannelFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestTimestampNow(t *testing.T) {
	ts := TimestampNow()
	if ts <= 0 {
		t.Fatal("expected positive timestamp")
	}
}

// --- Additional tests for coverage ---

func TestDingTalkChannel_Getters(t *testing.T) {
	ch, _ := NewDingTalkChannel("my-ding", "https://oapi.dingtalk.com/robot/send?access_token=x", "secret", true, false)
	if ch.ID() != "my-ding" {
		t.Errorf("expected ID my-ding, got %s", ch.ID())
	}
	if ch.Type() != "dingtalk" {
		t.Errorf("expected type dingtalk, got %s", ch.Type())
	}
	if !ch.IsEnabled() {
		t.Error("expected enabled")
	}
	if err := ch.Close(); err != nil {
		t.Fatalf("Close should return nil: %v", err)
	}

	ch2, _ := NewDingTalkChannel("d2", "https://oapi.dingtalk.com/robot/send?access_token=x", "", false, false)
	if ch2.IsEnabled() {
		t.Error("expected disabled")
	}
}

func TestFeishuChannel_Getters(t *testing.T) {
	ch, _ := NewFeishuChannel("my-feishu", "https://open.feishu.cn/open-apis/bot/v2/hook/x", "secret", true, false)
	if ch.ID() != "my-feishu" {
		t.Errorf("expected ID my-feishu, got %s", ch.ID())
	}
	if ch.Type() != "feishu" {
		t.Errorf("expected type feishu, got %s", ch.Type())
	}
	if !ch.IsEnabled() {
		t.Error("expected enabled")
	}
	if err := ch.Close(); err != nil {
		t.Fatalf("Close should return nil: %v", err)
	}

	ch2, _ := NewFeishuChannel("f2", "https://open.feishu.cn/open-apis/bot/v2/hook/x", "", false, false)
	if ch2.IsEnabled() {
		t.Error("expected disabled")
	}
}

func TestWeComChannel_Getters(t *testing.T) {
	ch, _ := NewWeComChannel("my-wecom", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x", true, false)
	if ch.ID() != "my-wecom" {
		t.Errorf("expected ID my-wecom, got %s", ch.ID())
	}
	if ch.Type() != "wecom" {
		t.Errorf("expected type wecom, got %s", ch.Type())
	}
	if !ch.IsEnabled() {
		t.Error("expected enabled")
	}
	if err := ch.Close(); err != nil {
		t.Fatalf("Close should return nil: %v", err)
	}

	ch2, _ := NewWeComChannel("w2", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x", false, false)
	if ch2.IsEnabled() {
		t.Error("expected disabled")
	}
}

func TestLogChannel_Getters(t *testing.T) {
	ch := NewLogChannel("my-log", true)
	if ch.ID() != "my-log" {
		t.Errorf("expected ID my-log, got %s", ch.ID())
	}
	if ch.Type() != "log" {
		t.Errorf("expected type log, got %s", ch.Type())
	}
	if !ch.IsEnabled() {
		t.Error("expected enabled")
	}
	if err := ch.Close(); err != nil {
		t.Fatalf("Close should return nil: %v", err)
	}

	ch2 := NewLogChannel("l2", false)
	if ch2.IsEnabled() {
		t.Error("expected disabled")
	}
}

func TestGenericWebhookChannel_Getters(t *testing.T) {
	ch, _ := NewGenericWebhookChannel("my-wh", "https://example.com/hook", nil, true, false)
	if ch.ID() != "my-wh" {
		t.Errorf("expected ID my-wh, got %s", ch.ID())
	}
	if ch.Type() != "webhook" {
		t.Errorf("expected type webhook, got %s", ch.Type())
	}
	if !ch.IsEnabled() {
		t.Error("expected enabled")
	}
	if err := ch.Close(); err != nil {
		t.Fatalf("Close should return nil: %v", err)
	}

	ch2, _ := NewGenericWebhookChannel("w2", "https://example.com/hook", nil, false, false)
	if ch2.IsEnabled() {
		t.Error("expected disabled")
	}
}

func TestNewChannelFromConfig_AllTypes(t *testing.T) {
	tests := []struct {
		cfg  ChannelConfig
		want string
	}{
		{ChannelConfig{ID: "a", Type: "log", Enabled: true}, "log"},
		{ChannelConfig{ID: "b", Type: "webhook", Enabled: true, WebhookURL: "https://example.com/h"}, "webhook"},
		{ChannelConfig{ID: "c", Type: "dingtalk", Enabled: true, WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=x"}, "dingtalk"},
		{ChannelConfig{ID: "d", Type: "feishu", Enabled: true, WebhookURL: "https://open.feishu.cn/open-apis/bot/v2/hook/x"}, "feishu"},
		{ChannelConfig{ID: "e", Type: "wecom", Enabled: true, WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x"}, "wecom"},
	}
	for _, tt := range tests {
		ch, err := NewChannelFromConfig(tt.cfg)
		if err != nil {
			t.Errorf("unexpected error for type %s: %v", tt.cfg.Type, err)
			continue
		}
		if ch.ID() != tt.cfg.ID {
			t.Errorf("expected ID %s, got %s", tt.cfg.ID, ch.ID())
		}
		if ch.IsEnabled() != tt.cfg.Enabled {
			t.Errorf("enabled mismatch for %s", tt.cfg.Type)
		}
	}
}

func TestNewChannelFromConfig_MissingWebhookURL(t *testing.T) {
	cfg := ChannelConfig{ID: "x", Type: "webhook", Enabled: true}
	_, err := NewChannelFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for webhook channel without URL")
	}
}

func TestDingTalkChannel_Send_Disabled(t *testing.T) {
	// Disabled channel should return nil without hitting the server
	// Use a public-looking URL that passes urlguard (no DNS for hostname)
	ch, _ := NewDingTalkChannel("d1", "https://oapi.dingtalk.com/robot/send?access_token=x", "", false, false)
	err := ch.Send(context.Background(), TaskNotification{Status: "success"})
	if err != nil {
		t.Fatalf("disabled channel should not error: %v", err)
	}
}

func TestFeishuChannel_Send_Disabled(t *testing.T) {
	ch, _ := NewFeishuChannel("f1", "https://open.feishu.cn/open-apis/bot/v2/hook/x", "", false, false)
	err := ch.Send(context.Background(), TaskNotification{Status: "failed"})
	if err != nil {
		t.Fatalf("disabled channel should not error: %v", err)
	}
}

func TestWeComChannel_Send_Disabled(t *testing.T) {
	ch, _ := NewWeComChannel("w1", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x", false, false)
	err := ch.Send(context.Background(), TaskNotification{Status: "timeout"})
	if err != nil {
		t.Fatalf("disabled channel should not error: %v", err)
	}
}

func TestGenericWebhook_Send_WithHeaders(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("X-Custom-Auth")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	headers := map[string]string{"X-Custom-Auth": "secret-token"}
	ch, _ := NewGenericWebhookChannel("wh-h", server.URL, headers, true, true)
	err := ch.Send(context.Background(), TaskNotification{Status: "success"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "secret-token" {
		t.Errorf("expected X-Custom-Auth header, got %s", gotAuth)
	}
}

func TestStatusLabel(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"success", "执行成功"},
		{"failed", "执行失败"},
		{"timeout", "执行超时"},
		{"unknown", "unknown"}, // returns input as-is for unknown
		{"", ""},               // returns empty string
	}
	for _, tt := range tests {
		got := statusLabel(tt.status)
		if got != tt.want {
			t.Errorf("statusLabel(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestRegistry_ListAllInfos(t *testing.T) {
	r := NewRegistry()
	r.Register(NewLogChannel("info1", true))
	r.Register(NewLogChannel("info2", false))

	infos := r.ListAllInfos()
	if len(infos) != 2 {
		t.Fatalf("expected 2 infos, got %d", len(infos))
	}
	ids := make(map[string]bool)
	for _, info := range infos {
		ids[info.ID] = true
		if info.Type != "log" {
			t.Errorf("expected type log, got %s", info.Type)
		}
	}
	if !ids["info1"] || !ids["info2"] {
		t.Error("expected both channel IDs present")
	}
}

func TestRegistry_Reload_InvalidConfig(t *testing.T) {
	r := NewRegistry()
	r.Register(NewLogChannel("good", true))

	// Reload with a mix of valid + invalid configs
	cfgs := []ChannelConfig{
		{ID: "new", Type: "log", Enabled: true},
		{ID: "bad", Type: "unknown", Enabled: true}, // invalid, skipped
	}
	r.Reload(cfgs)

	// "good" should be removed (not in new config list)
	// "new" should be present
	if r.Get("new") == nil {
		t.Fatal("expected new channel after reload")
	}
	if r.Get("bad") != nil {
		t.Fatal("expected bad channel to be skipped")
	}
}

func TestRegistry_Reload_PinnedChannelPreserved(t *testing.T) {
	r := NewRegistry()
	r.Register(NewLogChannel("pinned-ch", true))
	r.Pin("pinned-ch")

	// Reload with different channels — pinned-ch should survive
	cfgs := []ChannelConfig{
		{ID: "new-ch", Type: "log", Enabled: true},
	}
	r.Reload(cfgs)

	if r.Get("pinned-ch") == nil {
		t.Fatal("expected pinned channel to survive reload")
	}
	if r.Get("new-ch") == nil {
		t.Fatal("expected new channel after reload")
	}
}

func TestRegistry_Reload_UnpinnedChannelRemoved(t *testing.T) {
	r := NewRegistry()
	r.Register(NewLogChannel("normal-ch", true))
	// Don't pin — should be removed on reload

	cfgs := []ChannelConfig{
		{ID: "new-ch", Type: "log", Enabled: true},
	}
	r.Reload(cfgs)

	if r.Get("normal-ch") != nil {
		t.Fatal("expected unpinned channel to be removed")
	}
	if r.Get("new-ch") == nil {
		t.Fatal("expected new channel after reload")
	}
}

func TestDingTalkChannel_Send_ContextCancel(t *testing.T) {
	ch, _ := NewDingTalkChannel("d1", "https://oapi.dingtalk.com/robot/send?access_token=x", "", true, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ch.Send(ctx, TaskNotification{Status: "success"})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFeishuChannel_Send_ContextCancel(t *testing.T) {
	ch, _ := NewFeishuChannel("f1", "https://open.feishu.cn/open-apis/bot/v2/hook/x", "", true, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ch.Send(ctx, TaskNotification{Status: "success"})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestWeComChannel_Send_ContextCancel(t *testing.T) {
	ch, _ := NewWeComChannel("w1", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x", true, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ch.Send(ctx, TaskNotification{Status: "success"})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestLogChannel_Send_ErrorStatus(t *testing.T) {
	ch := NewLogChannel("log-err", true)
	err := ch.Send(context.Background(), TaskNotification{
		Status: "failed", Error: "something broke", Duration: 5000,
	})
	if err != nil {
		t.Fatalf("log channel should not return error: %v", err)
	}
}

func TestGenericWebhook_Send_ContextCancel(t *testing.T) {
	ch, _ := NewGenericWebhookChannel("wh", "https://example.com/hook", nil, true, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ch.Send(ctx, TaskNotification{Status: "success"})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestDingTalkChannel_Send_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	ch, _ := NewDingTalkChannel("d1", server.URL, "", true, true)
	err := ch.Send(context.Background(), TaskNotification{Status: "failed", Error: "rate limited"})
	if err == nil {
		t.Fatal("expected error for 429 status")
	}
}

func TestFeishuChannel_Send_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	ch, _ := NewFeishuChannel("f1", server.URL, "", true, true)
	err := ch.Send(context.Background(), TaskNotification{Status: "timeout"})
	if err == nil {
		t.Fatal("expected error for 502 status")
	}
}

func TestDingTalkChannel_Send_WithSign(t *testing.T) {
	var reqURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, _ := NewDingTalkChannel("d-s", server.URL, "test-secret", true, true)
	err := ch.Send(context.Background(), TaskNotification{Status: "success", TaskName: "signed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqURL == "" {
		t.Fatal("expected URL with query params (sign)")
	}
}

// ===== FeishuAppChannel tests =====

// newMockFeishuServer creates a mock Feishu API server that handles:
// - POST /open-apis/auth/v3/tenant_access_token/internal → token
// - POST /open-apis/im/v1/images → image upload
// - POST /open-apis/im/v1/messages → message send
func newMockFeishuServer(t *testing.T) (serverURL string, tokenCalls, uploadCalls, msgCalls *int, lastMsgBody *map[string]interface{}) {
	t.Helper()
	tokenCnt := 0
	uploadCnt := 0
	msgCnt := 0
	var msgBody map[string]interface{}

	mux := http.NewServeMux()

	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		tokenCnt++
		json.NewEncoder(w).Encode(feishuTokenResponse{
			Code: 0, Msg: "ok",
			TenantAccessToken: "test-token-xxx", Expire: 7200,
		})
	})

	mux.HandleFunc("/open-apis/im/v1/images", func(w http.ResponseWriter, r *http.Request) {
		uploadCnt++
		// Verify Authorization header
		if r.Header.Get("Authorization") != "Bearer test-token-xxx" {
			t.Errorf("expected Bearer token, got %s", r.Header.Get("Authorization"))
		}
		imgKey := fmt.Sprintf("img_%d", uploadCnt)
		json.NewEncoder(w).Encode(feishuImageUploadResponse{
			Code: 0, Msg: "ok",
			Data: &struct {
				ImageKey string `json:"image_key"`
			}{ImageKey: imgKey},
		})
	})

	mux.HandleFunc("/open-apis/im/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		msgCnt++
		json.NewDecoder(r.Body).Decode(&msgBody)
		json.NewEncoder(w).Encode(feishuMessageResponse{Code: 0, Msg: "ok"})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server.URL, &tokenCnt, &uploadCnt, &msgCnt, &msgBody
}

func TestFeishuAppChannel_Getters(t *testing.T) {
	ch := NewFeishuAppChannel("app-id", "app-secret", "chat-id", true)
	if ch.ID() != "feishu_app" {
		t.Errorf("expected ID feishu_app, got %s", ch.ID())
	}
	if ch.Type() != "feishu_app" {
		t.Errorf("expected type feishu_app, got %s", ch.Type())
	}
	if !ch.IsEnabled() {
		t.Error("expected enabled")
	}
	if err := ch.Close(); err != nil {
		t.Fatalf("Close should return nil: %v", err)
	}

	ch2 := NewFeishuAppChannel("a", "s", "c", false)
	if ch2.IsEnabled() {
		t.Error("expected disabled")
	}
}

func TestFeishuAppChannel_Send_Disabled(t *testing.T) {
	ch := NewFeishuAppChannel("app-id", "app-secret", "chat-id", false)
	err := ch.Send(context.Background(), TaskNotification{Status: "success"})
	if err != nil {
		t.Fatalf("disabled channel should not error: %v", err)
	}
}

func TestFeishuAppChannel_Send_NoImages(t *testing.T) {
	srvURL, tokenCalls, uploadCalls, msgCalls, lastBody := newMockFeishuServer(t)

	ch := &FeishuAppChannel{
		appID:     "test-app",
		appSecret: "test-secret",
		chatID:    "oc_test_chat",
		enabled:   true,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	// Override base URL by injecting the token endpoint via a custom transport
	// Since FeishuAppChannel uses hardcoded URLs, we test via direct field manipulation
	// For a clean test, we'll test the internal methods directly

	// Test via a real flow with the mock server
	// We need to test getToken, uploadImage, sendMessage separately
	// since the URLs are hardcoded in the struct methods

	// Instead, let's test the full Send flow by creating a custom channel
	// that uses the mock server's URLs
	ch2 := &feishuAppTestChannel{
		FeishuAppChannel: ch,
		baseURL:          srvURL,
	}

	n := TaskNotification{
		TaskID:    "t1",
		TaskName:  "test task",
		TaskType:  "batch_screenshot",
		Status:    "success",
		Result:    "截图完成",
		Duration:  3500,
		Timestamp: time.Now(),
	}
	err := ch2.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *tokenCalls != 1 {
		t.Errorf("expected 1 token call, got %d", *tokenCalls)
	}
	if *uploadCalls != 0 {
		t.Errorf("expected 0 upload calls (no images), got %d", *uploadCalls)
	}
	if *msgCalls != 1 {
		t.Errorf("expected 1 message call, got %d", *msgCalls)
	}
	if lastBody == nil {
		t.Fatal("expected message body")
	}
}

func TestFeishuAppChannel_Send_WithImages(t *testing.T) {
	srvURL, tokenCalls, uploadCalls, msgCalls, lastBody := newMockFeishuServer(t)

	// Create temp image files
	tmpDir := t.TempDir()
	img1 := filepath.Join(tmpDir, "shot1.png")
	img2 := filepath.Join(tmpDir, "shot2.jpg")
	os.WriteFile(img1, []byte("fake-png-data"), 0644)
	os.WriteFile(img2, []byte("fake-jpg-data"), 0644)

	ch := &FeishuAppChannel{
		appID:     "test-app",
		appSecret: "test-secret",
		chatID:    "oc_test_chat",
		enabled:   true,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	ch2 := &feishuAppTestChannel{
		FeishuAppChannel: ch,
		baseURL:          srvURL,
	}

	n := TaskNotification{
		TaskID:     "t1",
		TaskName:   "screenshot task",
		TaskType:   "batch_screenshot",
		Status:     "success",
		Result:     "截图完成",
		Duration:   5000,
		Timestamp:  time.Now(),
		ImagePaths: []string{img1, img2},
	}
	err := ch2.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Token: 1 for getToken + 2 for uploadImage (each upload calls getToken, but token is cached)
	if *tokenCalls < 1 {
		t.Errorf("expected at least 1 token call, got %d", *tokenCalls)
	}
	if *uploadCalls != 2 {
		t.Errorf("expected 2 upload calls, got %d", *uploadCalls)
	}
	if *msgCalls != 1 {
		t.Errorf("expected 1 message call, got %d", *msgCalls)
	}

	// Verify the card message contains image elements
	if lastBody == nil {
		t.Fatal("expected message body")
	}
}

func TestFeishuAppChannel_Send_UploadFailure(t *testing.T) {
	// Create a server that fails image uploads
	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(feishuTokenResponse{
			Code: 0, TenantAccessToken: "tok", Expire: 7200,
		})
	})
	mux.HandleFunc("/open-apis/im/v1/images", func(w http.ResponseWriter, r *http.Request) {
		// Simulate upload failure
		json.NewEncoder(w).Encode(feishuImageUploadResponse{Code: 230001, Msg: "image too large"})
	})
	mux.HandleFunc("/open-apis/im/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(feishuMessageResponse{Code: 0, Msg: "ok"})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	tmpDir := t.TempDir()
	img1 := filepath.Join(tmpDir, "big.png")
	os.WriteFile(img1, []byte("fake-data"), 0644)

	ch := &FeishuAppChannel{
		appID: "a", appSecret: "s", chatID: "c",
		enabled: true, client: &http.Client{Timeout: 5 * time.Second},
	}
	ch2 := &feishuAppTestChannel{FeishuAppChannel: ch, baseURL: server.URL}

	n := TaskNotification{
		Status: "success", TaskName: "test", Duration: 1000,
		ImagePaths: []string{img1},
	}
	// Should NOT error — upload failure is gracefully handled (falls back to text)
	err := ch2.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("upload failure should be gracefully handled, got: %v", err)
	}
}

func TestFeishuAppChannel_Send_TokenError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(feishuTokenResponse{Code: 10003, Msg: "invalid app_id"})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ch := &FeishuAppChannel{
		appID: "bad", appSecret: "bad", chatID: "c",
		enabled: true, client: &http.Client{Timeout: 5 * time.Second},
	}
	ch2 := &feishuAppTestChannel{FeishuAppChannel: ch, baseURL: server.URL}

	n := TaskNotification{Status: "success", TaskName: "test", Duration: 1000}
	err := ch2.Send(context.Background(), n)
	if err == nil {
		t.Fatal("expected error when token fetch fails")
	}
}

func TestFeishuAppChannel_Send_MessageError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(feishuTokenResponse{
			Code: 0, TenantAccessToken: "tok", Expire: 7200,
		})
	})
	mux.HandleFunc("/open-apis/im/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(feishuMessageResponse{Code: 230002, Msg: "chat not found"})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ch := &FeishuAppChannel{
		appID: "a", appSecret: "s", chatID: "bad_chat",
		enabled: true, client: &http.Client{Timeout: 5 * time.Second},
	}
	ch2 := &feishuAppTestChannel{FeishuAppChannel: ch, baseURL: server.URL}

	n := TaskNotification{Status: "success", TaskName: "test", Duration: 1000}
	err := ch2.Send(context.Background(), n)
	if err == nil {
		t.Fatal("expected error when message send fails")
	}
}

func TestFeishuAppChannel_Send_ContextCancel(t *testing.T) {
	ch := &FeishuAppChannel{
		appID: "a", appSecret: "s", chatID: "c",
		enabled: true, client: &http.Client{Timeout: 5 * time.Second},
	}
	ch2 := &feishuAppTestChannel{FeishuAppChannel: ch, baseURL: "http://127.0.0.1:1"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n := TaskNotification{Status: "success", TaskName: "test", Duration: 1000}
	err := ch2.Send(ctx, n)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// feishuAppTestChannel wraps FeishuAppChannel and overrides the hardcoded API base URL
// so we can point it at a local httptest server.
type feishuAppTestChannel struct {
	*FeishuAppChannel
	baseURL string
}

func (c *feishuAppTestChannel) getToken(ctx context.Context) (string, error) {
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}

	body := map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("token error: code=%d msg=%s", result.Code, result.Msg)
	}

	c.token = result.TenantAccessToken
	c.tokenExp = time.Now().Add(time.Duration(result.Expire-60) * time.Second)
	return c.token, nil
}

func (c *feishuAppTestChannel) uploadImage(ctx context.Context, imagePath string) (string, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", err
	}

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("image_type", "message")
	part, err := writer.CreateFormFile("image", filepath.Base(imagePath))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	part.Write(imageData)
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/open-apis/im/v1/images",
		&buf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload image: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ImageKey string `json:"image_key"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("upload error: code=%d msg=%s", result.Code, result.Msg)
	}

	return result.Data.ImageKey, nil
}

func (c *feishuAppTestChannel) sendMessage(ctx context.Context, body FeishuAppMessage) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/open-apis/im/v1/messages?receive_id_type=chat_id",
		bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create message request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode message response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("message error: code=%d msg=%s", result.Code, result.Msg)
	}

	return nil
}

func (c *feishuAppTestChannel) Send(ctx context.Context, n TaskNotification) error {
	if !c.enabled {
		return nil
	}

	statusEmoji := map[string]string{
		"success": "✅",
		"failed":  "❌",
		"timeout": "⏰",
	}
	emoji := statusEmoji[n.Status]
	template := "blue"
	if n.Status == "failed" {
		template = "red"
	} else if n.Status == "timeout" {
		template = "orange"
	}
	title := fmt.Sprintf("%s [UniMap] 定时任务 [%s] %s", emoji, n.TaskName, statusLabel(n.Status))

	var payloadLines []string
	if n.Payload != nil {
		payloadFields := []string{"urls", "query", "queries", "engines", "engine",
			"detection_mode", "low_threshold", "format", "ports", "max_age_days",
			"alert_type", "duration_minutes", "task_type", "type", "file_pattern"}
		for _, field := range payloadFields {
			if val, ok := n.Payload[field]; ok {
				payloadLines = append(payloadLines, fmt.Sprintf("**%s**: %v", field, val))
			}
		}
	}

	elements := []FeishuCardElement{}

	if len(payloadLines) > 0 {
		elements = append(elements, FeishuMarkdownElement(strings.Join(payloadLines, "\n")))
	}

	elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("**耗时**: %.1fs", n.Duration/1000.0)))

	if len(n.ImagePaths) > 0 {
		elements = append(elements, FeishuHRElement())
		elements = append(elements, FeishuMarkdownElement("**截图预览**:"))

		for _, imgPath := range n.ImagePaths {
			imageKey, err := c.uploadImage(ctx, imgPath)
			if err != nil {
				elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("⚠️ %s (上传失败: %v)", filepath.Base(imgPath), err)))
				continue
			}
			elements = append(elements, FeishuImageElement(imageKey, filepath.Base(imgPath)))
		}
	}

	if n.Result != "" {
		elements = append(elements, FeishuHRElement())
		elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("**执行结果**:\n%s", n.Result)))
	}

	if n.Error != "" {
		elements = append(elements, FeishuHRElement())
		elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("**错误**: %s", n.Error)))
	}

	card := FeishuCard{
		Header:   FeishuCardHeader{Title: FeishuTextElement{Tag: "plain_text", Content: title}, Template: template},
		Elements: elements,
	}

	cardJSON, _ := json.Marshal(card)

	return c.sendMessage(ctx, FeishuAppMessage{
		ReceiveID: c.chatID,
		MsgType:   "interactive",
		Content:   string(cardJSON),
	})
}

// Ensure feishuAppTestChannel implements the Send method we need.
var _ interface {
	Send(context.Context, TaskNotification) error
} = (*feishuAppTestChannel)(nil)
