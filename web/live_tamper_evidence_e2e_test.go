//go:build live_tamper_e2e

package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/scheduler"
	"github.com/unimap/project/internal/service"
	"github.com/unimap/project/internal/utils/urlguard"
)

// TestLiveTamperEvidenceNotificationAndRestart verifies the controlled external
// acceptance path:
// reset page -> establish baseline -> mutate page -> detect tampering ->
// guarded CDP evidence screenshot -> Feishu image notification -> server
// restart -> persisted baseline and scheduled task remain usable.
//
// The target and control endpoints must belong to an explicitly authorized
// test environment. Tokens and response bodies are never logged.
func TestLiveTamperEvidenceNotificationAndRestart(t *testing.T) {
	targetURL := requireLivePublicHTTPSURL(t, "UNIMAP_LIVE_TAMPER_URL")
	resetURL := requireLivePublicHTTPSURL(t, "UNIMAP_LIVE_TAMPER_RESET_URL")
	mutateURL := requireLivePublicHTTPSURL(t, "UNIMAP_LIVE_TAMPER_MUTATE_URL")
	controlToken := strings.TrimSpace(os.Getenv("UNIMAP_LIVE_TAMPER_CONTROL_TOKEN"))
	if controlToken == "" {
		t.Fatal("UNIMAP_LIVE_TAMPER_CONTROL_TOKEN is required")
	}

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
	port := liveBridgePort(t)
	configureLiveTamperEvidence(cfg, stateDir, port)

	controlClient := urlguard.SafeHTTPClient(urlguard.CheckOptions{}, 20*time.Second)
	postLiveTamperControl(t, controlClient, resetURL, controlToken)
	t.Cleanup(func() {
		postLiveTamperControl(t, controlClient, resetURL, controlToken)
	})

	app, srv, serverErr := startLiveTamperServer(t, cfg)
	serverRunning := true
	t.Cleanup(func() {
		if serverRunning {
			shutdownLiveTamperServer(t, srv, app)
		}
	})
	waitForLiveServer(t, srv, serverErr, 30*time.Second)

	baseline, err := srv.tamperApp.SetBaseline(t.Context(), service.TamperBaselineRequest{
		URLs: []string{targetURL}, Concurrency: 1,
	}, srv.screenshotMgr)
	if err != nil || baseline.Summary["saved"] != 1 {
		t.Fatalf("establish controlled page baseline: saved=%d err=%v", baseline.Summary["saved"], err)
	}

	postLiveTamperControl(t, controlClient, mutateURL, controlToken)

	beforeNotify := liveNotifyMetric(t, srv, "feishu_app", "success")
	runAt := time.Now().Add(time.Hour)
	taskID := fmt.Sprintf("live-tamper-evidence-%d", time.Now().UnixNano())
	task := &scheduler.ScheduledTask{
		ID: taskID, Name: "Live controlled tamper evidence verification",
		Type: scheduler.TaskTamperCheck, Enabled: false, ScheduleType: "once", RunAt: &runAt,
		Payload: &model.TaskPayload{
			URLs:       []string{targetURL},
			DetectMode: "strict",
		},
		TimeoutSec: 120,
		Notifications: &scheduler.NotificationConfig{
			Enabled: true, OnSuccess: true, ChannelIDs: []string{channelID},
		},
	}
	if err := srv.scheduler.AddTask(task); err != nil {
		t.Fatalf("create live tamper task: %v", err)
	}
	if err := srv.scheduler.RunTaskNow(taskID); err != nil {
		t.Fatalf("run live tamper task: %v", err)
	}

	record := waitForLiveTaskRecord(t, srv.scheduler, taskID, 130*time.Second)
	if record.Status != "success" {
		t.Fatalf("tamper evidence task failed: status=%s error=%s", record.Status, record.Error)
	}
	if !strings.Contains(record.Result, "已篡改") {
		t.Fatalf("controlled page mutation was not classified as tampered: %s", record.Result)
	}
	imagePath := liveScreenshotPath(t, record.Result)
	image, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("read tamper evidence screenshot: %v", err)
	}
	if !hasPNGSignature(image) {
		t.Fatal("tamper evidence screenshot is not a real PNG")
	}
	waitForLiveNotificationMetric(t, srv, beforeNotify, 20*time.Second)

	shutdownLiveTamperServer(t, srv, app)
	serverRunning = false

	restartedApp, restartedSrv, restartedErr := startLiveTamperServer(t, cfg)
	srv, app, serverErr = restartedSrv, restartedApp, restartedErr
	serverRunning = true
	waitForLiveServer(t, restartedSrv, restartedErr, 30*time.Second)
	if _, err := restartedSrv.scheduler.GetTask(taskID); err != nil {
		t.Fatalf("scheduled tamper task was not restored after restart: %v", err)
	}
	check, err := restartedSrv.tamperApp.Check(t.Context(), service.TamperCheckRequest{
		URLs: []string{targetURL}, Concurrency: 1, Mode: "strict",
	}, restartedSrv.screenshotMgr)
	if err != nil || check.Summary["tampered"] != 1 {
		t.Fatalf("persisted baseline failed after restart: tampered=%d err=%v", check.Summary["tampered"], err)
	}

	t.Logf(
		"LIVE_TAMPER_EVIDENCE success tampered=true png_bytes=%d notification_success=true task_restored=true baseline_persisted=true",
		len(image),
	)
}

func configureLiveTamperEvidence(cfg *config.Config, stateDir string, port int) {
	cfg.Web.BindAddress = "127.0.0.1"
	cfg.Web.Port = port
	cfg.Screenshot.Enabled = true
	cfg.Screenshot.Engine = "cdp"
	cfg.Screenshot.Mode = "cdp"
	cfg.Screenshot.ChromeUserDataDir = ""
	cfg.Screenshot.ChromeProfileDir = ""
	cfg.Screenshot.ChromeRemoteDebugURL = ""
	cfg.Screenshot.ChromeRemoteDebugAddress = ""
	cfg.Screenshot.BaseDir = filepath.Join(stateDir, "screenshots")
	cfg.History.DatabasePath = filepath.Join(stateDir, "history.db")
	cfg.ICP.DatabasePath = filepath.Join(stateDir, "icp_results.db")
	cfg.Tamper.EvidenceScreenshotEnabled = true
	headless := true
	fallback := false
	cfg.Screenshot.Headless = &headless
	cfg.Screenshot.Fallback = &fallback
}

func startLiveTamperServer(
	t *testing.T,
	cfg *config.Config,
) (*service.UnifiedService, *Server, <-chan error) {
	t.Helper()
	app := service.NewUnifiedServiceWithConfig(cfg)
	srv, err := NewServer(cfg.Web.Port, app, app.GetOrchestrator(), cfg, nil)
	if err != nil {
		_ = app.Shutdown()
		t.Fatalf("create live tamper server: %v", err)
	}
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	return app, srv, serverErr
}

func shutdownLiveTamperServer(t *testing.T, srv *Server, app *service.UnifiedService) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown live tamper server: %v", err)
	}
	if err := app.Shutdown(); err != nil {
		t.Fatalf("shutdown live tamper application: %v", err)
	}
}

func requireLivePublicHTTPSURL(t *testing.T, envName string) string {
	t.Helper()
	rawURL := strings.TrimSpace(os.Getenv(envName))
	if rawURL == "" {
		t.Fatalf("%s is required", envName)
	}
	parsed, err := urlguard.Check(rawURL, urlguard.CheckOptions{AllowedSchemes: []string{"https"}})
	if err != nil {
		t.Fatalf("%s must be a public HTTPS URL: %v", envName, err)
	}
	if urlguard.IsInternalHost(t.Context(), parsed.Hostname()) {
		t.Fatalf("%s resolves to an internal or restricted address", envName)
	}
	return parsed.String()
}

func postLiveTamperControl(t *testing.T, client *http.Client, endpoint, token string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint, nil)
	if err != nil {
		t.Fatalf("create controlled page mutation request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("call controlled page mutation endpoint: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("controlled page mutation endpoint returned status %d", resp.StatusCode)
	}
}

func hasPNGSignature(image []byte) bool {
	return len(image) >= 8 &&
		string(image[:8]) == "\x89PNG\r\n\x1a\n"
}
