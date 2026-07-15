//go:build live_bridge_e2e

package web

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/scheduler"
	"github.com/unimap/project/internal/service"
)

// TestLiveBridgeSearchScreenshotNotification verifies the real extension path:
// the Bridge pulls a screenshot task, returns image bytes, and the scheduler
// sends that image through an enabled Feishu app notification channel. Set
// UNIMAP_LIVE_BRIDGE_ENGINE to one of the stable engines to select the target.
//
// It is deliberately excluded from the default suite because it uses the
// local Chrome extension, a logged-in search-engine session, and the real
// configured notification channel. It never prints configuration or tokens.
func TestLiveBridgeSearchScreenshotNotification(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("APPDATA", stateDir)
	t.Setenv("LOCALAPPDATA", stateDir)

	webRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve web root: %v", err)
	}
	t.Setenv("UNIMAP_WEB_ROOT", webRoot)

	cfgManager := config.NewManager(liveBridgeConfigPath(t))
	if err := cfgManager.Load(); err != nil {
		t.Fatalf("load live configuration: %v", err)
	}
	cfg := cfgManager.GetConfig().Clone()
	if cfg == nil {
		t.Fatal("live configuration is unavailable")
	}

	channelID := enabledFeishuAppChannelID(t, cfg)
	engine, query := liveBridgeEngine(t)
	cfg.Web.BindAddress = "127.0.0.1"
	cfg.Web.Port = 8448 // The extension's documented default loopback endpoint.
	cfg.Screenshot.BaseDir = filepath.Join(stateDir, "screenshots")
	cfg.History.DatabasePath = filepath.Join(stateDir, "history.db")
	cfg.ICP.DatabasePath = filepath.Join(stateDir, "icp_results.db")

	listener, err := net.Listen("tcp", "127.0.0.1:8448")
	if err != nil {
		t.Fatalf("127.0.0.1:8448 is required for the extension test: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("release Bridge test port: %v", err)
	}

	app := service.NewUnifiedServiceWithConfig(cfg)
	srv, err := NewServer(cfg.Web.Port, app, app.GetOrchestrator(), cfg, cfgManager)
	if err != nil {
		t.Fatalf("create loopback Bridge server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Errorf("shutdown loopback Bridge server: %v", err)
		}
		if err := app.Shutdown(); err != nil {
			t.Errorf("shutdown application service: %v", err)
		}
	})

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	waitForLiveBridge(t, srv, serverErr, 30*time.Second)
	channel := srv.notifyRegistry.Get(channelID)
	if channel == nil || !channel.IsEnabled() || channel.Type() != "feishu_app" {
		t.Fatal("the configured Feishu app image channel is not ready")
	}

	beforeNotify := liveNotifyMetric(t, srv, "feishu_app", "success")
	runAt := time.Now().Add(time.Hour)
	taskID := fmt.Sprintf("live-bridge-%s-e2e-%d", engine, time.Now().UnixNano())
	task := &scheduler.ScheduledTask{
		ID:           taskID,
		Name:         fmt.Sprintf("Live Bridge %s screenshot notification verification", engine),
		Type:         scheduler.TaskSearchScreenshot,
		Enabled:      false,
		ScheduleType: "once",
		RunAt:        &runAt,
		Payload: &model.TaskPayload{
			Query: query,
			Extra: map[string]any{
				"engine":   engine,
				"query_id": taskID,
			},
		},
		TimeoutSec: 120,
		Notifications: &scheduler.NotificationConfig{
			Enabled:    true,
			OnSuccess:  true,
			ChannelIDs: []string{channelID},
		},
	}
	if err := srv.scheduler.AddTask(task); err != nil {
		t.Fatalf("create live Bridge task: %v", err)
	}
	t.Cleanup(func() {
		if err := srv.scheduler.DeleteTask(taskID); err != nil {
			t.Errorf("remove live Bridge task: %v", err)
		}
	})
	if err := srv.scheduler.RunTaskNow(taskID); err != nil {
		t.Fatalf("run live Bridge task: %v", err)
	}

	record := waitForLiveTaskRecord(t, srv.scheduler, taskID, 130*time.Second)
	if record.Status != "success" {
		t.Fatalf("Bridge task did not succeed: status=%s", record.Status)
	}
	imagePath := liveScreenshotPath(t, record.Result)
	image, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("read returned Bridge screenshot: %v", err)
	}
	if len(image) == 0 {
		t.Fatal("returned Bridge screenshot is empty")
	}
	if artifactPath := preserveLiveScreenshot(t, engine, image); artifactPath != "" {
		t.Logf("LIVE_BRIDGE_E2E screenshot_artifact=%s", artifactPath)
	}

	waitForLiveNotificationMetric(t, srv, beforeNotify, 20*time.Second)
	sum := sha256.Sum256(image)
	t.Logf("LIVE_BRIDGE_E2E success engine=%s bridge_callback=true screenshot_bytes=%d screenshot_sha256_prefix=%x notification_type=feishu_app notification_success=true", engine, len(image), sum[:6])
}

// TestLiveBridgeScheduledQueryClosedLoop verifies the complete scheduled path:
// query -> Bridge structured collection + screenshot -> SQLite history -> image notification.
func TestLiveBridgeScheduledQueryClosedLoop(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("APPDATA", stateDir)
	t.Setenv("LOCALAPPDATA", stateDir)

	webRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve web root: %v", err)
	}
	t.Setenv("UNIMAP_WEB_ROOT", webRoot)

	cfgManager := config.NewManager(liveBridgeConfigPath(t))
	if err := cfgManager.Load(); err != nil {
		t.Fatalf("load live configuration: %v", err)
	}
	cfg := cfgManager.GetConfig().Clone()
	if cfg == nil {
		t.Fatal("live configuration is unavailable")
	}

	channelID := enabledFeishuAppChannelID(t, cfg)
	engine, query := liveBridgeEngine(t)
	cfg.Web.BindAddress = "127.0.0.1"
	cfg.Web.Port = 8448
	cfg.Screenshot.BaseDir = filepath.Join(stateDir, "screenshots")
	cfg.History.DatabasePath = filepath.Join(stateDir, "history.db")
	cfg.ICP.DatabasePath = filepath.Join(stateDir, "icp_results.db")

	listener, err := net.Listen("tcp", "127.0.0.1:8448")
	if err != nil {
		t.Fatalf("127.0.0.1:8448 is required for the extension test: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("release Bridge test port: %v", err)
	}

	app := service.NewUnifiedServiceWithConfig(cfg)
	srv, err := NewServer(cfg.Web.Port, app, app.GetOrchestrator(), cfg, cfgManager)
	if err != nil {
		t.Fatalf("create loopback Bridge server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Errorf("shutdown loopback Bridge server: %v", err)
		}
		if err := app.Shutdown(); err != nil {
			t.Errorf("shutdown application service: %v", err)
		}
	})

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	waitForLiveBridge(t, srv, serverErr, 30*time.Second)

	beforeNotify := liveNotifyMetric(t, srv, "feishu_app", "success")
	runAt := time.Now().Add(time.Hour)
	taskID := fmt.Sprintf("live-bridge-query-%s-e2e-%d", engine, time.Now().UnixNano())
	task := &scheduler.ScheduledTask{
		ID: taskID, Name: fmt.Sprintf("Live Bridge %s scheduled query closed-loop verification", engine),
		Type: scheduler.TaskQuery, Enabled: false, ScheduleType: "once", RunAt: &runAt,
		Payload: &model.TaskPayload{
			Query: query, Engines: []string{engine}, PageSize: 10,
			BrowserQuery: true, BrowserAction: "collect_and_capture", QueryID: taskID,
		},
		TimeoutSec: 180,
		Notifications: &scheduler.NotificationConfig{
			Enabled: true, OnSuccess: true, ChannelIDs: []string{channelID},
		},
	}
	if err := srv.scheduler.AddTask(task); err != nil {
		t.Fatalf("create live Bridge query task: %v", err)
	}
	t.Cleanup(func() { _ = srv.scheduler.DeleteTask(taskID) })
	if err := srv.scheduler.RunTaskNow(taskID); err != nil {
		t.Fatalf("run live Bridge query task: %v", err)
	}

	record := waitForLiveTaskRecord(t, srv.scheduler, taskID, 190*time.Second)
	if record.Status != "success" {
		t.Fatalf("closed-loop task did not succeed: status=%s error=%s", record.Status, record.Error)
	}
	imagePath := liveScreenshotPath(t, record.Result)
	image, err := os.ReadFile(imagePath)
	if err != nil || len(image) == 0 {
		t.Fatalf("read closed-loop Bridge screenshot: bytes=%d err=%v", len(image), err)
	}

	histories, total, err := srv.historyRepo.ListHistory("query", 10, 0)
	if err != nil {
		t.Fatalf("list persisted query history: %v", err)
	}
	if total != 1 || len(histories) != 1 {
		t.Fatalf("persisted query history count=%d/%d, want one", total, len(histories))
	}
	results, err := srv.historyRepo.GetResults(histories[0].ID)
	if err != nil || len(results) == 0 {
		t.Fatalf("persisted query results=%d err=%v", len(results), err)
	}

	waitForLiveNotificationMetric(t, srv, beforeNotify, 20*time.Second)
	t.Logf("LIVE_BRIDGE_CLOSED_LOOP success engine=%s persisted_results=%d screenshot_bytes=%d notification_success=true", engine, len(results), len(image))
}

func liveBridgeConfigPath(t *testing.T) string {
	t.Helper()
	for _, path := range []string{
		filepath.Join("..", "configs", "config.yaml"),
		filepath.Join("configs", "config.yaml"),
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Fatal("configs/config.yaml is required for the live Bridge test")
	return ""
}

func enabledFeishuAppChannelID(t *testing.T, cfg *config.Config) string {
	t.Helper()
	for _, channel := range cfg.Notifications.Channels {
		if channel.Enabled && strings.EqualFold(strings.TrimSpace(channel.Type), "feishu_app") && strings.TrimSpace(channel.ID) != "" {
			return channel.ID
		}
	}
	t.Fatal("an enabled feishu_app notification channel is required to verify image delivery")
	return ""
}

func liveBridgeEngine(t *testing.T) (engine, query string) {
	t.Helper()
	engine = strings.ToLower(strings.TrimSpace(os.Getenv("UNIMAP_LIVE_BRIDGE_ENGINE")))
	if engine == "" {
		engine = "fofa"
	}
	queries := map[string]string{
		"fofa":    `port="443"`,
		"hunter":  `port="443"`,
		"zoomeye": "port:443",
		"quake":   "port:443",
		"shodan":  "port:443",
	}
	query, ok := queries[engine]
	if !ok {
		t.Fatalf("UNIMAP_LIVE_BRIDGE_ENGINE=%q is unsupported; use fofa, hunter, zoomeye, quake, or shodan", engine)
	}
	return engine, query
}

// preserveLiveScreenshot optionally retains one screenshot for visual E2E
// diagnosis. By default the live test keeps all state in t.TempDir and leaves
// no files behind. The caller must explicitly choose an absolute directory.
func preserveLiveScreenshot(t *testing.T, engine string, image []byte) string {
	t.Helper()
	dir := strings.TrimSpace(os.Getenv("UNIMAP_LIVE_BRIDGE_ARTIFACT_DIR"))
	if dir == "" {
		return ""
	}
	if !filepath.IsAbs(dir) {
		t.Fatal("UNIMAP_LIVE_BRIDGE_ARTIFACT_DIR must be an absolute path")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("create live screenshot artifact directory: %v", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("live-bridge-%s-%d.png", engine, time.Now().UnixNano()))
	if err := os.WriteFile(path, image, 0600); err != nil {
		t.Fatalf("preserve live Bridge screenshot: %v", err)
	}
	return path
}

func waitForLiveBridge(t *testing.T, srv *Server, serverErr <-chan error, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-serverErr:
			t.Fatalf("loopback Bridge server stopped: %v", err)
		default:
		}
		snapshot := srv.buildBridgeDiagnosticSnapshot()
		if snapshot.Ready && snapshot.ExtensionOnline && snapshot.LiveClients > 0 {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	snapshot := srv.buildBridgeDiagnosticSnapshot()
	t.Fatalf("Bridge extension did not become ready (extension_online=%t live_clients=%d paired_clients=%d)", snapshot.ExtensionOnline, snapshot.LiveClients, snapshot.PairedClients)
}

func waitForLiveTaskRecord(t *testing.T, sched *scheduler.Scheduler, taskID string, timeout time.Duration) scheduler.ExecutionRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, record := range sched.GetHistory(100, "", "") {
			if record.TaskID == taskID {
				return record
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the live Bridge task record")
	return scheduler.ExecutionRecord{}
}

func liveScreenshotPath(t *testing.T, result string) string {
	t.Helper()
	for _, line := range strings.Split(result, "\n") {
		if before, after, ok := strings.Cut(line, "保存:"); ok && strings.TrimSpace(before) != "" {
			path := strings.TrimSpace(after)
			if path != "" {
				return path
			}
		}
	}
	t.Fatal("task result did not contain a screenshot path")
	return ""
}

func liveNotifyMetric(t *testing.T, srv *Server, channelType, status string) float64 {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8448/metrics", nil) // #nosec G107 -- fixed loopback verification endpoint
	if err != nil {
		t.Fatalf("create notification metrics request: %v", err)
	}
	if token := srv.adminToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("read notification metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read notification metrics: status=%d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read notification metrics body: %v", err)
	}
	wantType := `channel_type="` + channelType + `"`
	wantStatus := `status="` + status + `"`
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "unimap_scheduler_notify_total{") || !strings.Contains(line, wantType) || !strings.Contains(line, wantStatus) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		value, err := strconv.ParseFloat(fields[1], 64)
		if err == nil {
			return value
		}
	}
	return 0
}

func waitForLiveNotificationMetric(t *testing.T, srv *Server, before float64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if liveNotifyMetric(t, srv, "feishu_app", "success") > before {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("Feishu app notification did not report successful delivery")
}
