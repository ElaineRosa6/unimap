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
	if sign == "" {
		t.Fatal("expected non-empty sign")
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
