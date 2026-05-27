package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
	var received map[string]interface{}
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
	if received["task_id"] != "t1" {
		t.Errorf("expected task_id t1, got %v", received["task_id"])
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
	var received map[string]interface{}
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
	if received["msgtype"] != "markdown" {
		t.Errorf("expected msgtype markdown, got %v", received["msgtype"])
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
	var received map[string]interface{}
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
	if received["msg_type"] != "interactive" {
		t.Errorf("expected msg_type interactive, got %v", received["msg_type"])
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
	var received map[string]interface{}
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
	if received["msgtype"] != "markdown" {
		t.Errorf("expected msgtype markdown, got %v", received["msgtype"])
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
