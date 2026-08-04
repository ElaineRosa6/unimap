//go:build live_bridge_e2e || live_tamper_e2e

package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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
	"github.com/unimap/project/internal/history"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/notify"
	"github.com/unimap/project/internal/scheduler"
	"github.com/unimap/project/internal/screenshot"
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
	configureLiveBridgeExtension(cfg)

	channelID := enabledFeishuAppChannelID(t, cfg)
	engine, query := liveBridgeEngine(t)
	if nativeQuery := strings.TrimSpace(os.Getenv("UNIMAP_LIVE_BRIDGE_NATIVE_QUERY")); nativeQuery != "" {
		query = nativeQuery
	}
	port := liveBridgePort(t)
	cfg.Web.BindAddress = "127.0.0.1"
	cfg.Web.Port = port
	cfg.Screenshot.BaseDir = filepath.Join(stateDir, "screenshots")
	cfg.Screenshot.Timeout = 60
	cfg.History.DatabasePath = filepath.Join(stateDir, "history.db")
	cfg.ICP.DatabasePath = filepath.Join(stateDir, "icp_results.db")

	listenAddress := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		t.Fatalf("%s is required for the extension test: %v", listenAddress, err)
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

	waitForLiveNotificationMetric(t, srv, beforeNotify, 60*time.Second)
	sum := sha256.Sum256(image)
	t.Logf("LIVE_BRIDGE_E2E success engine=%s bridge_callback=true screenshot_bytes=%d screenshot_sha256_prefix=%x notification_type=feishu_app notification_success=true", engine, len(image), sum[:6])
}

// TestLiveBridgeStructuredCollection isolates the real Extension DOM/network
// collection path from API credentials, screenshots, persistence, and
// notifications. It is the first diagnostic to run when a closed-loop test
// reports an empty browser result.
func TestLiveBridgeStructuredCollection(t *testing.T) {
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
	configureLiveBridgeExtension(cfg)
	cfg.Screenshot.AutoCapture.Enabled = false
	cfg.Screenshot.AutoCapture.CaptureSearchResults = false
	engine, query := liveBridgeEngine(t)
	cfg.Web.BindAddress = "127.0.0.1"
	cfg.Web.Port = liveBridgePort(t)
	cfg.Screenshot.BaseDir = filepath.Join(stateDir, "screenshots")
	cfg.History.DatabasePath = filepath.Join(stateDir, "history.db")

	app := service.NewUnifiedServiceWithConfig(cfg)
	srv, err := NewServer(cfg.Web.Port, app, app.GetOrchestrator(), cfg, cfgManager)
	if err != nil {
		t.Fatalf("create loopback Bridge server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		_ = app.Shutdown()
	})
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	waitForLiveBridge(t, srv, serverErr, 30*time.Second)

	outcome := <-srv.runBrowserQueryAsync(t.Context(), query, []string{engine}, true, "collect", "live-structured", nil)
	if len(outcome.Errors) > 0 {
		t.Fatalf("Bridge structured collection errors: %s", strings.Join(outcome.Errors, "; "))
	}
	if len(outcome.CollectedResults) != 1 {
		t.Fatalf("Bridge collected result envelopes=%d, want one", len(outcome.CollectedResults))
	}
	result := outcome.CollectedResults[0]
	if len(result.Assets) == 0 {
		t.Fatalf("Bridge returned zero assets: method=%s selector=%q rows=%d extraction_error=%q login_wall=%t",
			result.ExtractionMethod, result.RowSelectorUsed, result.RowsFound, result.ExtractionError, result.IsLoginWall)
	}
	t.Logf("LIVE_BRIDGE_STRUCTURED success engine=%s assets=%d total=%d method=%s rows=%d", engine, len(result.Assets), result.Total, result.ExtractionMethod, result.RowsFound)
}

// TestLiveBridgeCookieHandoffToCDP proves the fallback requested by the
// browser workflow: read the authenticated UI profile through the loopback
// Extension Bridge, hand cookies and origin storage to CDP without logging
// values, then collect and capture.
func TestLiveBridgeCookieHandoffToCDP(t *testing.T) {
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
	configureLiveBridgeExtension(cfg)
	cfg.Screenshot.ChromeRemoteDebugURL = strings.TrimSpace(os.Getenv("UNIMAP_LIVE_CDP_URL"))
	cfg.Screenshot.ChromeUserDataDir = filepath.Join(stateDir, "cdp-profile")
	cfg.Screenshot.Timeout = 60
	if proxy := strings.TrimSpace(os.Getenv("UNIMAP_LIVE_CDP_SOCKS5_PROXY")); proxy != "" {
		cfg.Screenshot.ProxyServer = proxy
	}
	headless := !strings.EqualFold(strings.TrimSpace(os.Getenv("UNIMAP_LIVE_CDP_HEADFUL")), "true")
	cfg.Screenshot.Headless = &headless
	cfg.Screenshot.BaseDir = filepath.Join(stateDir, "screenshots")
	cfg.History.DatabasePath = filepath.Join(stateDir, "history.db")
	cfg.Web.BindAddress = "127.0.0.1"
	cfg.Web.Port = liveBridgePort(t)
	engine, _ := liveBridgeEngine(t)

	app := service.NewUnifiedServiceWithConfig(cfg)
	srv, err := NewServer(cfg.Web.Port, app, app.GetOrchestrator(), cfg, cfgManager)
	if err != nil {
		t.Fatalf("create Bridge/CDP handoff server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		_ = app.Shutdown()
	})
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	waitForLiveBridge(t, srv, serverErr, 30*time.Second)
	if srv.screenshotMgr == nil {
		t.Fatal("CDP manager is unavailable")
	}

	handoffCtx, handoffCancel := context.WithTimeout(t.Context(), 30*time.Second)
	cookieCount, err := srv.syncExtensionCookiesToCDP(handoffCtx, engine)
	handoffCancel()
	if err != nil {
		t.Fatalf("Bridge-to-CDP cookie handoff: %v", err)
	}
	storage := srv.screenshotMgr.GetBrowserStorage(engine)
	credentialCount := cookieCount + len(storage.Local) + len(storage.Session)
	if credentialCount == 0 {
		t.Fatal("Bridge-to-CDP handoff returned no cookies or origin storage")
	}

	query := liveNativeBrowserQuery(engine)
	collectCtx, collectCancel := context.WithTimeout(t.Context(), 90*time.Second)
	results, imagePath, err := srv.screenshotMgr.CollectAndCaptureSearchEngineResult(collectCtx, engine, query, "live-cookie-handoff")
	collectCancel()
	if err != nil {
		t.Fatalf("CDP collect_and_capture after cookie handoff: %v", err)
	}
	if image, readErr := os.ReadFile(imagePath); readErr == nil {
		if artifact := preserveLiveScreenshot(t, engine+"-cdp", image); artifact != "" {
			t.Logf("LIVE_BRIDGE_CDP_SCREENSHOT artifact=%s", artifact)
		}
	}
	image, err := os.ReadFile(imagePath)
	if err != nil || len(image) < 8 || string(image[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("CDP screenshot after cookie handoff is not a PNG: bytes=%d err=%v", len(image), err)
	}
	if len(results) != 1 {
		t.Fatalf("CDP returned an unexpected result envelope after credential handoff: %#v", results)
	}
	if results[0].BrowserChallenge {
		t.Logf("LIVE_BRIDGE_CDP_HANDOFF challenge_detected engine=%s cookies=%d storage_entries=%d extraction_error=%s png_bytes=%d", engine, cookieCount, credentialCount-cookieCount, results[0].ExtractionError, len(image))
		if srv.screenshotRouter == nil {
			t.Fatal("screenshot router is unavailable for browser-challenge fallback")
		}
		srv.screenshotRouter.SetMode(screenshot.ModeAuto)
		fallbackCtx, fallbackCancel := context.WithTimeout(t.Context(), 120*time.Second)
		fallbackResults, fallbackImagePath, fallbackErr := srv.screenshotRouter.CollectAndCaptureSearchEngineResult(fallbackCtx, engine, query, "live-cookie-handoff-fallback")
		fallbackCancel()
		if fallbackErr != nil {
			t.Fatalf("automatic Extension fallback after CDP browser challenge: %v", fallbackErr)
		}
		if len(fallbackResults) != 1 || len(fallbackResults[0].Assets) == 0 {
			t.Fatalf("automatic Extension fallback returned no assets: %#v", fallbackResults)
		}
		fallbackImage, readFallbackErr := os.ReadFile(fallbackImagePath)
		if readFallbackErr != nil || len(fallbackImage) < 8 || string(fallbackImage[:8]) != "\x89PNG\r\n\x1a\n" {
			t.Fatalf("automatic Extension fallback screenshot is not a PNG: bytes=%d err=%v", len(fallbackImage), readFallbackErr)
		}
		t.Logf("LIVE_BRIDGE_CDP_FALLBACK success engine=%s assets=%d png_bytes=%d", engine, len(fallbackResults[0].Assets), len(fallbackImage))
		return
	}
	if len(results[0].Assets) == 0 {
		t.Fatalf("CDP returned no assets after credential handoff: %#v", results)
	}
	t.Logf("LIVE_BRIDGE_CDP_HANDOFF success engine=%s cookies=%d storage_entries=%d assets=%d png_bytes=%d", engine, cookieCount, credentialCount-cookieCount, len(results[0].Assets), len(image))
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
	configureLiveBridgeExtension(cfg)

	channelID := enabledFeishuAppChannelID(t, cfg)
	engine, query := liveBridgeEngine(t)
	port := liveBridgePort(t)
	cfg.Web.BindAddress = "127.0.0.1"
	cfg.Web.Port = port
	cfg.Screenshot.BaseDir = filepath.Join(stateDir, "screenshots")
	cfg.History.DatabasePath = filepath.Join(stateDir, "history.db")
	cfg.ICP.DatabasePath = filepath.Join(stateDir, "icp_results.db")

	listenAddress := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		t.Fatalf("%s is required for the extension test: %v", listenAddress, err)
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
	recorder := &recordingNotifyChannel{delegate: channel, results: make(chan error, 1)}
	srv.notifyRegistry.Remove(channelID)
	if err := srv.notifyRegistry.Register(recorder); err != nil {
		t.Fatalf("instrument Feishu app channel: %v", err)
	}

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
	if artifactPath := preserveLiveScreenshot(t, engine, image); artifactPath != "" {
		t.Logf("LIVE_BRIDGE_CLOSED_LOOP screenshot_artifact=%s", artifactPath)
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
	if !strings.Contains(record.Result, "📋 查询结果明细") {
		t.Fatalf("scheduled notification body has no query detail section:\n%s", record.Result)
	}
	if !recordContainsPersistedAsset(t, record.Result, results) {
		t.Fatalf("scheduled notification body has no persisted asset detail:\n%s", record.Result)
	}

	select {
	case notifyErr := <-recorder.results:
		if notifyErr != nil {
			t.Fatalf("Feishu app notification failed: %v", notifyErr)
		}
	case <-time.After(70 * time.Second):
		t.Fatal("timed out waiting for Feishu app notification result")
	}
	waitForLiveNotificationMetric(t, srv, beforeNotify, 5*time.Second)
	t.Logf("LIVE_BRIDGE_CLOSED_LOOP success engine=%s persisted_results=%d notification_details=true screenshot_bytes=%d notification_success=true", engine, len(results), len(image))
}

type recordingNotifyChannel struct {
	delegate notify.NotifyChannel
	results  chan error
}

func (c *recordingNotifyChannel) ID() string      { return c.delegate.ID() }
func (c *recordingNotifyChannel) Type() string    { return c.delegate.Type() }
func (c *recordingNotifyChannel) IsEnabled() bool { return c.delegate.IsEnabled() }
func (c *recordingNotifyChannel) Close() error    { return nil }
func (c *recordingNotifyChannel) Send(ctx context.Context, message notify.TaskNotification) error {
	err := c.delegate.Send(ctx, message)
	select {
	case c.results <- err:
	default:
	}
	return err
}

// TestLiveAPIScheduledQueryNotificationDetails verifies that the non-Bridge
// scheduled query path persists API assets and sends those assets, not only a
// count, through the configured notification channel.
func TestLiveAPIScheduledQueryNotificationDetails(t *testing.T) {
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
	engine := strings.ToLower(strings.TrimSpace(os.Getenv("UNIMAP_LIVE_API_ENGINE")))
	if engine == "" {
		engine = "fofa"
	}
	cfg.Web.BindAddress = "127.0.0.1"
	cfg.Web.Port = liveBridgePort(t)
	cfg.Screenshot.BaseDir = filepath.Join(stateDir, "screenshots")
	cfg.History.DatabasePath = filepath.Join(stateDir, "history.db")
	cfg.ICP.DatabasePath = filepath.Join(stateDir, "icp_results.db")

	app := service.NewUnifiedServiceWithConfig(cfg)
	srv, err := NewServer(cfg.Web.Port, app, app.GetOrchestrator(), cfg, cfgManager)
	if err != nil {
		t.Fatalf("create live API query server: %v", err)
	}
	// Production cmd/unimap-web registers configured adapters before NewServer.
	// This package-level E2E constructs the service directly, so mirror that
	// registration step without exposing any credential values.
	srv.registerCoreEngineAdapters(cfg)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Errorf("shutdown live API query server: %v", err)
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
	waitForLiveServer(t, srv, serverErr, 30*time.Second)

	beforeNotify := liveNotifyMetric(t, srv, "feishu_app", "success")
	runAt := time.Now().Add(time.Hour)
	taskID := fmt.Sprintf("live-api-query-%s-e2e-%d", engine, time.Now().UnixNano())
	task := &scheduler.ScheduledTask{
		ID: taskID, Name: fmt.Sprintf("Live API %s scheduled query detail verification", engine),
		Type: scheduler.TaskQuery, Enabled: false, ScheduleType: "once", RunAt: &runAt,
		Payload: &model.TaskPayload{
			Query: `port="443"`, Engines: []string{engine}, PageSize: 10,
			NotificationDetailLimit: 10,
		},
		TimeoutSec: 120,
		Notifications: &scheduler.NotificationConfig{
			Enabled: true, OnSuccess: true, ChannelIDs: []string{channelID},
		},
	}
	if err := srv.scheduler.AddTask(task); err != nil {
		t.Fatalf("create live API query task: %v", err)
	}
	t.Cleanup(func() { _ = srv.scheduler.DeleteTask(taskID) })
	if err := srv.scheduler.RunTaskNow(taskID); err != nil {
		t.Fatalf("run live API query task: %v", err)
	}
	record := waitForLiveTaskRecord(t, srv.scheduler, taskID, 130*time.Second)
	if record.Status != "success" {
		t.Fatalf("live API query task did not succeed: status=%s error=%s", record.Status, record.Error)
	}
	histories, total, err := srv.historyRepo.ListHistory("query", 10, 0)
	if err != nil || total != 1 || len(histories) != 1 {
		t.Fatalf("persisted API query history count=%d/%d err=%v, want one", total, len(histories), err)
	}
	results, err := srv.historyRepo.GetResults(histories[0].ID)
	if err != nil || len(results) == 0 {
		t.Fatalf("persisted API query results=%d err=%v", len(results), err)
	}
	if !strings.Contains(record.Result, "📋 查询结果明细") || !recordContainsPersistedAsset(t, record.Result, results) {
		t.Fatalf("API query notification body has no persisted asset details:\n%s", record.Result)
	}
	waitForLiveNotificationMetric(t, srv, beforeNotify, 20*time.Second)
	t.Logf("LIVE_API_QUERY success engine=%s persisted_results=%d notification_details=true notification_success=true", engine, len(results))
}

func recordContainsPersistedAsset(t *testing.T, result string, rows []history.OperationResult) bool {
	t.Helper()
	for _, row := range rows {
		var asset model.UnifiedAsset
		if err := json.Unmarshal([]byte(row.Data), &asset); err != nil {
			t.Fatalf("decode persisted query asset: %v", err)
		}
		for _, needle := range []string{asset.URL, asset.Host, asset.IP} {
			if needle != "" && strings.Contains(result, needle) {
				return true
			}
		}
	}
	return false
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

func liveBridgePort(t *testing.T) int {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("UNIMAP_LIVE_BRIDGE_PORT"))
	if raw == "" {
		return 8448
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1024 || port > 65535 {
		t.Fatalf("UNIMAP_LIVE_BRIDGE_PORT=%q must be an integer from 1024 through 65535", raw)
	}
	return port
}

func liveNativeBrowserQuery(engine string) string {
	queries := map[string]string{
		"fofa":      `port="443"`,
		"hunter":    `port="443"`,
		"zoomeye":   `port:443`,
		"quake":     `port:443`,
		"shodan":    `port:443`,
		"censys":    `host.services.port=443`,
		"daydaymap": `ip.port="443"`,
	}
	return queries[engine]
}

// configureLiveBridgeExtension keeps the live test independent from production
// pairing credentials while still exercising the real Extension-only path.
// The unpacked extension uses "dev-pair" for automatic local test pairing.
func configureLiveBridgeExtension(cfg *config.Config) {
	cfg.Screenshot.Enabled = true
	cfg.Screenshot.Engine = "extension"
	cfg.Screenshot.Mode = "extension"
	cfg.Screenshot.Extension.Enabled = true
	cfg.Screenshot.Extension.PairingRequired = true
	cfg.Screenshot.Extension.PairCode = "dev-pair"
	fallback := false
	cfg.Screenshot.Fallback = &fallback
	cfg.Screenshot.Extension.FallbackToCDP = false
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
		"fofa":      `port="443"`,
		"hunter":    `port="443"`,
		"zoomeye":   `port="443"`,
		"quake":     `port="443"`,
		"shodan":    `port="443"`,
		"censys":    `port="443"`,
		"daydaymap": `port="443"`,
	}
	query, ok := queries[engine]
	if !ok {
		t.Fatalf("UNIMAP_LIVE_BRIDGE_ENGINE=%q is unsupported; use fofa, hunter, zoomeye, quake, shodan, censys, or daydaymap", engine)
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

func waitForLiveServer(t *testing.T, srv *Server, serverErr <-chan error, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-serverErr:
			t.Fatalf("live server stopped: %v", err)
		default:
		}
		liveURL := fmt.Sprintf("http://127.0.0.1:%d/health/live", srv.port)
		req, err := http.NewRequest(http.MethodGet, liveURL, nil) // #nosec G107 -- fixed loopback verification endpoint
		if err == nil {
			resp, requestErr := http.DefaultClient.Do(req)
			if requestErr == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("timed out waiting for live server")
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
	metricsURL := fmt.Sprintf("http://127.0.0.1:%d/metrics", srv.port)
	req, err := http.NewRequest(http.MethodGet, metricsURL, nil) // #nosec G107 -- fixed loopback verification endpoint
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
	t.Fatalf("Feishu app notification did not report successful delivery (success=%.0f failure=%.0f)",
		liveNotifyMetric(t, srv, "feishu_app", "success"), liveNotifyMetric(t, srv, "feishu_app", "failed"))
}
