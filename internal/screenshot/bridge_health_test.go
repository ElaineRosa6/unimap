package screenshot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ===== mockBridgeClient =====

type mockBridgeClient struct {
	mu          sync.Mutex
	submitCalls []BridgeTask
	submitURLs  map[string]string
	submitErr   error
	awaitResult BridgeResult
	awaitErr    error
	submitDelay time.Duration
}

type cancelAwareBridgeClient struct {
	calls    chan string
	release  chan struct{}
	canceled chan string
}

func (c *cancelAwareBridgeClient) SubmitTask(ctx context.Context, task BridgeTask) error {
	c.calls <- task.RequestID
	if task.RequestID == "first" && c.release != nil {
		select {
		case <-c.release:
		case <-ctx.Done():
			c.canceled <- task.RequestID
			return ctx.Err()
		}
	}
	if task.RequestID == "in-flight" {
		<-ctx.Done()
		c.canceled <- task.RequestID
		return ctx.Err()
	}
	return nil
}

func (c *cancelAwareBridgeClient) AwaitResult(_ context.Context, requestID string) (BridgeResult, error) {
	return BridgeResult{RequestID: requestID, Success: true, FinalURL: "https://example.com"}, nil
}

func (m *mockBridgeClient) SubmitTask(ctx context.Context, task BridgeTask) error {
	m.mu.Lock()
	m.submitCalls = append(m.submitCalls, task)
	if m.submitURLs == nil {
		m.submitURLs = make(map[string]string)
	}
	m.submitURLs[task.RequestID] = task.URL
	m.mu.Unlock()
	if m.submitDelay > 0 {
		select {
		case <-time.After(m.submitDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.submitErr
}

func (m *mockBridgeClient) AwaitResult(ctx context.Context, requestID string) (BridgeResult, error) {
	if m.awaitErr != nil {
		return BridgeResult{}, m.awaitErr
	}
	m.mu.Lock()
	result := m.awaitResult
	if result.Success && result.FinalURL == "" {
		result.FinalURL = m.submitURLs[requestID]
	}
	m.mu.Unlock()
	return result, nil
}

// ===== NewBridgeService =====

func TestNewBridgeService_Defaults(t *testing.T) {
	client := &mockBridgeClient{}
	svc := NewBridgeService(client, 0, 0)
	if svc.maxConcurrency != 5 {
		t.Errorf("maxConcurrency = %d, want 5", svc.maxConcurrency)
	}
	if svc.taskTimeout != 30*time.Second {
		t.Errorf("taskTimeout = %v, want 30s", svc.taskTimeout)
	}
	if svc.retry != 1 {
		t.Errorf("retry = %d, want 1", svc.retry)
	}
	if svc.client != client {
		t.Error("client not set")
	}
	if cap(svc.queue) != 5*8 {
		t.Errorf("queue cap = %d, want %d", cap(svc.queue), 5*8)
	}
}

func TestNewBridgeService_Custom(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 10, 60*time.Second)
	if svc.maxConcurrency != 10 {
		t.Errorf("maxConcurrency = %d, want 10", svc.maxConcurrency)
	}
	if svc.taskTimeout != 60*time.Second {
		t.Errorf("taskTimeout = %v, want 60s", svc.taskTimeout)
	}
}

func TestBridgeService_DefaultTimeoutForLongActions(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	if got := svc.defaultTimeoutForAction("collect"); got != collectBridgeTaskTimeout {
		t.Fatalf("collect timeout = %s, want %s", got, collectBridgeTaskTimeout)
	}
	if got := svc.defaultTimeoutForAction("collect_and_capture"); got != collectAndCaptureBridgeTaskTimeout {
		t.Fatalf("collect_and_capture timeout = %s, want %s", got, collectAndCaptureBridgeTaskTimeout)
	}

	svc = NewBridgeService(&mockBridgeClient{}, 5, 180*time.Second)
	if got := svc.defaultTimeoutForAction("collect_and_capture"); got != 180*time.Second {
		t.Fatalf("configured longer timeout should win, got %s", got)
	}
}

// ===== SetRetry =====

func TestBridgeService_SetRetry(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	svc.SetRetry(3)
	if svc.retry != 3 {
		t.Errorf("retry = %d, want 3", svc.retry)
	}
	svc.SetRetry(-1)
	if svc.retry != 0 {
		t.Errorf("retry = %d, want 0 after negative", svc.retry)
	}
}

// ===== Start/Stop =====

func TestBridgeService_StartStop(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 3, 30*time.Second)
	ctx := context.Background()

	svc.Start(ctx)
	if !svc.IsStarted() {
		t.Fatal("expected service to be started")
	}
	if svc.WorkerCount() != 3 {
		t.Errorf("WorkerCount = %d, want 3", svc.WorkerCount())
	}

	svc.Stop()
	if svc.IsStarted() {
		t.Error("expected service to be stopped after Stop()")
	}
}

func TestBridgeService_Stop_Idempotent(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 3, 30*time.Second)
	svc.Start(context.Background())
	svc.Stop()
	svc.Stop() // should not panic
}

func TestBridgeService_Start_Idempotent(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 3, 30*time.Second)
	ctx := context.Background()
	svc.Start(ctx)
	svc.Start(ctx) // second start should be no-op due to CompareAndSwap
	svc.Stop()
}

func TestBridgeService_Start_NilClient(t *testing.T) {
	var svc *BridgeService
	svc.Start(context.Background()) // should not panic
}

func TestBridgeService_Stop_Nil(t *testing.T) {
	var svc *BridgeService
	svc.Stop() // should not panic
}

func TestBridgeService_Submit_NilService(t *testing.T) {
	var svc *BridgeService
	_, err := svc.Submit(context.Background(), BridgeTask{RequestID: "1", URL: "http://test.com"})
	if err == nil {
		t.Error("expected error for nil service")
	}
}

// ===== QueueLen/WorkerCount/InFlight =====

func TestBridgeService_QueueLen_WorkerCount_InFlight_Nil(t *testing.T) {
	var svc *BridgeService
	if svc.QueueLen() != 0 {
		t.Errorf("QueueLen = %d, want 0", svc.QueueLen())
	}
	if svc.WorkerCount() != 0 {
		t.Errorf("WorkerCount = %d, want 0", svc.WorkerCount())
	}
	if svc.InFlight() != 0 {
		t.Errorf("InFlight = %d, want 0", svc.InFlight())
	}
	if svc.IsStarted() {
		t.Error("IsStarted = true, want false for nil service")
	}
}

// ===== Submit: validation =====

func TestBridgeService_Submit_EmptyRequestID(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	_, err := svc.Submit(context.Background(), BridgeTask{RequestID: "", URL: "http://test.com"})
	if err == nil {
		t.Fatal("expected error for empty request ID")
	}
	if !strings.Contains(err.Error(), "request id") {
		t.Errorf("error should mention request id: %v", err)
	}
}

func TestBridgeService_Submit_EmptyURL(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	_, err := svc.Submit(context.Background(), BridgeTask{RequestID: "123", URL: ""})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error should mention url: %v", err)
	}
}

func TestBridgeService_Submit_NotStarted(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	_, err := svc.Submit(context.Background(), BridgeTask{RequestID: "123", URL: "http://test.com"})
	if err == nil {
		t.Fatal("expected error for not-started service")
	}
	if !strings.Contains(err.Error(), "not started") {
		t.Errorf("error should mention not started: %v", err)
	}
}

// ===== Submit: success =====

func TestBridgeService_Submit_Success(t *testing.T) {
	client := &mockBridgeClient{
		awaitResult: BridgeResult{RequestID: "req-1", Success: true},
	}
	svc := NewBridgeService(client, 2, 5*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	result, err := svc.Submit(context.Background(), BridgeTask{RequestID: "req-1", URL: "http://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.RequestID != "req-1" {
		t.Errorf("RequestID = %q, want %q", result.RequestID, "req-1")
	}
}

func TestBridgeService_Submit_ClientError(t *testing.T) {
	client := &mockBridgeClient{
		submitErr: fmt.Errorf("network error"),
	}
	svc := NewBridgeService(client, 2, 5*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	_, err := svc.Submit(context.Background(), BridgeTask{RequestID: "req-2", URL: "http://example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ===== Submit: timeout =====

func TestBridgeService_Submit_Timeout(t *testing.T) {
	client := &mockBridgeClient{
		submitDelay: 10 * time.Second, // will be blocked by context timeout
	}
	svc := NewBridgeService(client, 2, 100*time.Millisecond)
	svc.Start(context.Background())
	defer svc.Stop()

	_, err := svc.Submit(context.Background(), BridgeTask{RequestID: "req-3", URL: "http://example.com"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestBridgeService_Submit_Canceled(t *testing.T) {
	client := &mockBridgeClient{}
	svc := NewBridgeService(client, 2, 30*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := svc.Submit(ctx, BridgeTask{RequestID: "req-4", URL: "http://example.com"})
	if err == nil {
		t.Fatal("expected canceled error")
	}
}

func TestBridgeService_GetCookiesValidatesHTTPSOriginAndPreservesBareDomain(t *testing.T) {
	client := &mockBridgeClient{awaitResult: BridgeResult{Success: true}}
	svc := NewBridgeService(client, 1, time.Second)
	validated := ""
	svc.urlValidator = func(_ context.Context, rawURL string) error {
		validated = rawURL
		return nil
	}
	svc.Start(t.Context())
	defer svc.Stop()

	if _, err := svc.Submit(t.Context(), BridgeTask{
		RequestID: "cookie-read", URL: "fofa.info", Action: "get_cookies",
	}); err != nil {
		t.Fatalf("get_cookies submit: %v", err)
	}
	if validated != "https://fofa.info" {
		t.Fatalf("validated URL = %q", validated)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.submitCalls) != 1 || client.submitCalls[0].URL != "fofa.info" {
		t.Fatalf("extension task did not preserve bare domain: %#v", client.submitCalls)
	}
}

func TestBridgeService_CanceledQueuedTaskNeverStarts(t *testing.T) {
	client := &cancelAwareBridgeClient{
		calls:    make(chan string, 3),
		release:  make(chan struct{}),
		canceled: make(chan string, 3),
	}
	svc := NewBridgeService(client, 1, 5*time.Second)
	svc.urlValidator = func(context.Context, string) error { return nil }
	svc.Start(t.Context())
	defer svc.Stop()

	firstDone := make(chan error, 1)
	go func() {
		_, err := svc.Submit(t.Context(), BridgeTask{RequestID: "first", URL: "https://example.com"})
		firstDone <- err
	}()
	if got := <-client.calls; got != "first" {
		t.Fatalf("first client call = %q", got)
	}

	queuedCtx, cancelQueued := context.WithCancel(t.Context())
	queuedDone := make(chan error, 1)
	go func() {
		_, err := svc.Submit(queuedCtx, BridgeTask{RequestID: "queued", URL: "https://example.com"})
		queuedDone <- err
	}()
	deadline := time.Now().Add(time.Second)
	for svc.QueueLen() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancelQueued()
	if err := <-queuedDone; err == nil || !errors.Is(err, ErrBridgeTaskCanceled) {
		t.Fatalf("queued Submit error = %v, want canceled", err)
	}
	close(client.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Submit failed: %v", err)
	}
	select {
	case got := <-client.calls:
		t.Fatalf("canceled queued task reached client: %q", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBridgeService_CancelPropagatesToInFlightClient(t *testing.T) {
	client := &cancelAwareBridgeClient{
		calls:    make(chan string, 1),
		canceled: make(chan string, 1),
	}
	svc := NewBridgeService(client, 1, 5*time.Second)
	svc.urlValidator = func(context.Context, string) error { return nil }
	svc.Start(t.Context())
	defer svc.Stop()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := svc.Submit(ctx, BridgeTask{RequestID: "in-flight", URL: "https://example.com"})
		done <- err
	}()
	if got := <-client.calls; got != "in-flight" {
		t.Fatalf("client call = %q", got)
	}
	cancel()
	if err := <-done; err == nil || !errors.Is(err, ErrBridgeTaskCanceled) {
		t.Fatalf("Submit error = %v, want canceled", err)
	}
	select {
	case got := <-client.canceled:
		if got != "in-flight" {
			t.Fatalf("canceled request = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not observe caller cancellation")
	}
}

func TestBridgeServiceValidateFinalURL(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 1, time.Second)
	svc.urlValidator = func(context.Context, string) error { return nil }
	if err := svc.validateFinalURL(t.Context(), "https://quake.360.net/search", ""); err == nil {
		t.Fatal("missing final URL must fail")
	}
	if err := svc.validateFinalURL(t.Context(), "https://quake.360.net/search", "https://internal.example/result"); err == nil {
		t.Fatal("cross-host final URL must fail")
	}
	if err := svc.validateFinalURL(t.Context(), "https://quake.360.net/search", "http://quake.360.net/result"); err == nil {
		t.Fatal("HTTPS to HTTP final URL downgrade must fail")
	}
	if err := svc.validateFinalURL(t.Context(), "https://quake.360.net/search", "https://quake.360.net:8443/result"); err == nil {
		t.Fatal("cross-port final URL must fail")
	}
	if err := svc.validateFinalURL(t.Context(), "https://quake.360.net/search", "https://quake.360.net/result"); err != nil {
		t.Fatalf("same-host final URL rejected: %v", err)
	}
}

func TestBridgeServicePreservesPreNavigationFailureWithoutFinalURL(t *testing.T) {
	client := &mockBridgeClient{awaitResult: BridgeResult{
		Success: false,
		Error:   "extension could not create tab",
	}}
	svc := NewBridgeService(client, 1, time.Second)
	svc.urlValidator = func(context.Context, string) error { return nil }
	svc.Start(t.Context())
	defer svc.Stop()

	result, err := svc.Submit(t.Context(), BridgeTask{
		RequestID: "pre-navigation-failure",
		URL:       "https://quake.360.net/search",
		Action:    "collect",
	})
	if err != nil {
		t.Fatalf("pre-navigation Bridge failure was replaced: %v", err)
	}
	if result.Error != "extension could not create tab" {
		t.Fatalf("result error = %q", result.Error)
	}
}

// ===== isRetryableBridgeError =====

func TestIsRetryableBridgeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"ErrBridgeTimeout", ErrBridgeTimeout, true},
		{"ErrBridgeSubmitFailed", ErrBridgeSubmitFailed, true},
		{"ErrBridgeInternalError", ErrBridgeInternalError, false},
		{"generic timeout string", fmt.Errorf("operation timeout exceeded"), true},
		{"generic connection string", fmt.Errorf("connection refused"), true},
		{"unrelated error", fmt.Errorf("invalid input"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableBridgeError(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableBridgeError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ===== executeWithRetry =====

func TestBridgeService_ExecuteWithRetry_Success(t *testing.T) {
	client := &mockBridgeClient{
		awaitResult: BridgeResult{RequestID: "req-1", Success: true},
	}
	svc := NewBridgeService(client, 5, 5*time.Second)
	result, err := svc.executeWithRetry(context.Background(), BridgeTask{RequestID: "req-1", URL: "http://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestBridgeService_ExecuteWithRetry_NonRetryable(t *testing.T) {
	// AwaitResult returns ErrBridgeInternalError, which is NOT retryable
	// (executeOnce wraps it without ErrBridgeSubmitFailed or ErrBridgeTimeout)
	client := &mockBridgeClient{
		awaitResult: BridgeResult{},
		awaitErr:    fmt.Errorf("%w: bad response", ErrBridgeInternalError),
	}
	svc := NewBridgeService(client, 5, 5*time.Second)
	svc.retry = 3
	result, err := svc.executeWithRetry(context.Background(), BridgeTask{RequestID: "req-5", URL: "http://example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
	// Non-retryable errors should bail immediately (1 attempt only)
	client.mu.Lock()
	calls := len(client.submitCalls)
	client.mu.Unlock()
	if calls != 1 {
		t.Errorf("expected 1 submit call (no retry), got %d", calls)
	}
	_ = result
}

// ===== CDPHealthChecker =====

func TestCDPHealthChecker_Mode(t *testing.T) {
	c := &CDPHealthChecker{}
	if c.Mode() != "cdp" {
		t.Errorf("Mode() = %q, want %q", c.Mode(), "cdp")
	}
}

func TestCDPHealthChecker_Override(t *testing.T) {
	ov := true
	c := &CDPHealthChecker{Override: &ov}
	ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true from override")
	}
}

func TestCDPHealthChecker_EmptyRemoteURL_LocalChromeFound(t *testing.T) {
	c := &CDPHealthChecker{
		RemoteDebugURL:    "",
		LocalChromeFinder: func() string { return "/usr/bin/google-chrome" },
	}
	ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true when local Chrome is available")
	}
}

func TestCDPHealthChecker_EmptyRemoteURL_NoLocalChrome(t *testing.T) {
	c := &CDPHealthChecker{
		RemoteDebugURL:    "",
		LocalChromeFinder: func() string { return "" },
	}
	ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false when no remote URL and no local Chrome found")
	}
}

func TestCDPHealthChecker_HTTPCall_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := &CDPHealthChecker{RemoteDebugURL: server.URL}
	ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for 200 OK")
	}
}

func TestCDPHealthChecker_HTTPCall_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := &CDPHealthChecker{RemoteDebugURL: server.URL}
	ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for 500 error")
	}
}

func TestCDPHealthChecker_ConnectionRefused(t *testing.T) {
	c := &CDPHealthChecker{RemoteDebugURL: "http://127.0.0.1:1"}
	ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for connection refused")
	}
}

// ===== ExtensionHealthChecker =====

func TestExtensionHealthChecker_Mode(t *testing.T) {
	e := &ExtensionHealthChecker{}
	if e.Mode() != "extension" {
		t.Errorf("Mode() = %q, want %q", e.Mode(), "extension")
	}
}

func TestExtensionHealthChecker_Override(t *testing.T) {
	ov := true
	e := &ExtensionHealthChecker{Override: &ov}
	ok, err := e.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true from override")
	}
}

func TestExtensionHealthChecker_NilBridge(t *testing.T) {
	e := &ExtensionHealthChecker{BridgeService: nil}
	ok, err := e.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for nil bridge")
	}
}

func TestExtensionHealthChecker_NotStarted(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	e := &ExtensionHealthChecker{BridgeService: svc}
	ok, err := e.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for not-started bridge")
	}
}

func TestExtensionHealthChecker_MockBypass(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	e := &ExtensionHealthChecker{BridgeService: svc, IsMock: true, LiveClient: func() bool { return true }}
	ok, err := e.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for mock bypass with live client")
	}
}

func TestExtensionHealthChecker_QueueOverload(t *testing.T) {
	// Create a bridge with small queue, fill it up
	client := &mockBridgeClient{submitDelay: 10 * time.Second}
	svc := NewBridgeService(client, 2, 5*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	// Submit many tasks to fill the queue
	e := &ExtensionHealthChecker{BridgeService: svc, LiveClient: func() bool { return true }}
	ok, err := e.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true when queue is not overloaded")
	}
}

func TestExtensionHealthChecker_StartedAndOK(t *testing.T) {
	client := &mockBridgeClient{}
	svc := NewBridgeService(client, 5, 30*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	e := &ExtensionHealthChecker{BridgeService: svc, LiveClient: func() bool { return true }}
	ok, err := e.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for started healthy bridge with live client")
	}
}

// ===== validatePath =====

func TestValidatePath(t *testing.T) {
	base := t.TempDir()

	t.Run("inside base dir", func(t *testing.T) {
		sub := filepath.Join(base, "subdir", "file.txt")
		if err := validatePath(base, sub); err != nil {
			t.Errorf("expected valid path: %v", err)
		}
	})

	t.Run("base dir itself", func(t *testing.T) {
		if err := validatePath(base, base); err != nil {
			t.Errorf("expected base dir itself to be valid: %v", err)
		}
	})
}

// ===== safeJoinPath =====

func TestSafeJoinPath(t *testing.T) {
	base := t.TempDir()

	t.Run("valid join", func(t *testing.T) {
		result, err := safeJoinPath(base, []string{"query_123", "screenshot.png"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(base, "query_123", "screenshot.png")
		if result != expected {
			t.Errorf("result = %q, want %q", result, expected)
		}
	})

	t.Run("empty elems", func(t *testing.T) {
		result, err := safeJoinPath(base, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != base {
			t.Errorf("result = %q, want %q", result, base)
		}
	})

	t.Run("sanitizes path traversal", func(t *testing.T) {
		// ".." should be sanitized, not interpreted as parent dir
		result, err := safeJoinPath(base, []string{"query", "..", "etc", "passwd"})
		if err != nil {
			t.Fatalf("expected no error (path is sanitized), got: %v", err)
		}
		// Verify the result is still inside base
		rel, err := filepath.Rel(base, result)
		if err != nil {
			t.Fatalf("failed to compute relative path: %v", err)
		}
		if strings.HasPrefix(rel, "..") {
			t.Errorf("safeJoinPath allowed path traversal: %q", result)
		}
	})
}

// ===== urlBase64 =====

func TestUrlBase64(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"hello"},
		{"http://example.com"},
		{""},
	}
	for _, tt := range tests {
		got := urlBase64(tt.input)
		// Verify no standard URL-unsafe chars (the function does URL encoding)
		if strings.ContainsAny(got, "+/") {
			t.Errorf("urlBase64(%q) contains +/: %q", tt.input, got)
		}
		if got == "" && tt.input != "" {
			t.Errorf("urlBase64(%q) returned empty", tt.input)
		}
	}
}

// ===== normalizeURL (router.go) =====

func TestNormalizeURL_Router(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://example.com", "http://example.com"},
		{"https://example.com", "https://example.com"},
		{"example.com", "http://example.com"},
		{"example.com/path", "http://example.com/path"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizeURL(tt.input)
		if got != tt.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ===== buildSearchEngineURL (router.go) =====

func TestBuildSearchEngineURL_Router(t *testing.T) {
	got := buildSearchEngineURL("fofa", "test query")
	// Should contain the engine name
	if !strings.Contains(got, "fofa") {
		t.Errorf("expected engine name in URL: %s", got)
	}
	// The query is base64-encoded in the URL
	if !strings.Contains(got, "qbase64=") {
		t.Errorf("expected qbase64 param in URL: %s", got)
	}
}

func TestBuildSearchEngineURL_Router_AllEngines(t *testing.T) {
	tests := []struct {
		engine   string
		query    string
		contains string
	}{
		{"fofa", "test", "fofa.info"},
		{"hunter", "test", "hunter.qianxin.com"},
		{"quake", "test", "quake.360.net"},
		{"zoomeye", "test", "zoomeye.org"},
		{"shodan", "test", "shodan.io"},
		{"censys", "test", "platform.censys.io"},
		{"daydaymap", "test", "daydaymap.com"},
	}
	for _, tt := range tests {
		t.Run(tt.engine, func(t *testing.T) {
			got := buildSearchEngineURL(tt.engine, tt.query)
			if got == "" {
				t.Fatalf("buildSearchEngineURL(%q) returned empty URL", tt.engine)
			}
			if !strings.Contains(got, tt.contains) {
				t.Errorf("buildSearchEngineURL(%q) = %q, want contains %q", tt.engine, got, tt.contains)
			}
		})
	}
	// Unknown engine returns empty
	if got := buildSearchEngineURL("nonexistent", "test"); got != "" {
		t.Errorf("buildSearchEngineURL(nonexistent) = %q, want empty", got)
	}
}
