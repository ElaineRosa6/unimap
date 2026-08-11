//go:build query_excel_push

// Command query-excel-push runs one UQL query through BOTH collection paths
// (API + browser DOM/Bridge) in parallel, merges the results, exports a
// full-field Excel workbook (all UnifiedAsset fields + Extra columns), and
// pushes that workbook as a WeCom file message to a configured channel.
//
// This is a network/browser diagnostic — it is kept behind an explicit build
// tag so it never enters `go build ./...` or `go test ./...` by accident:
//
//	go run -tags query_excel_push ./tools/query-excel-push \
//	  -config configs/config.yaml -engines fofa -page-size 100 -channel dijia_01
//
// With -query empty the tool loads the fofa_ynmobile_daily task query ([REDACTED]
// body 查询) from docs/CLOUD_SCHEDULER_TASKS_2026-08-06.json.
//
// -via-web routes the query through the running web service's /api/v1/query
// with browser_query=true, so browser collection uses the extension bridge
// (paired to the user's real Chrome) that the web service owns — headless
// Chrome is blocked by FOFA. Example:
//
//	go run -tags query_excel_push ./tools/query-excel-push \
//	  -config configs/config.yaml -engines fofa -page-size 100 -channel dijia_01 -via-web
//
// Secrets (API keys, webhook URLs, cookies, admin token) are read from
// config.yaml only and are never printed. Output shows channel IDs and asset
// counts, never URLs.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/exporter"
	"github.com/unimap/project/internal/history"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/notify"
	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/service"
	"github.com/unimap/project/internal/utils"
)

func main() {
	defer func() { _ = logger.Sync() }()

	configPath := flag.String("config", utils.DefaultConfigPath(), "config yaml path")
	query := flag.String("query", "", "UQL query to run (empty: load from -task in -tasks-json)")
	tasksJSON := flag.String("tasks-json", "docs/CLOUD_SCHEDULER_TASKS_2026-08-06.json", "scheduler task JSON used when -query is empty")
	taskName := flag.String("task", "fofa_ynmobile_daily", "task name to load the query from when -query is empty")
	enginesFlag := flag.String("engines", "fofa", "comma separated engine list")
	pageSize := flag.Int("page-size", 100, "page size")
	channelID := flag.String("channel", "dijia_01", "notify channel id used for the file push")
	outPath := flag.String("out", "", "output xlsx path (default ./data/query_excel_push_<ts>.xlsx)")
	noBrowser := flag.Bool("no-browser", false, "skip the browser DOM/Bridge collection path")
	localChrome := flag.Bool("local-chrome", true, "launch a guarded local Chrome for browser collection instead of the configured remote debugger")
	allowRemoteChrome := flag.Bool("allow-remote-chrome", false, "authorize using the configured remote Chrome debugger (relaxes the SSRF guarded-egress refusal); for trusted local diagnostics only")
	noPush := flag.Bool("no-push", false, "export the Excel only, do not push to the channel")
	timeout := flag.Duration("timeout", 300*time.Second, "overall query+browser timeout")
	viaWeb := flag.Bool("via-web", false, "route the query through the running web service (/api/v1/query, browser_query=true) so browser collection uses the extension bridge instead of running engines in-process")
	webURL := flag.String("web-url", "http://127.0.0.1:8448", "base URL of the running web service (used with -via-web)")
	onlyNew := flag.Bool("only-new", false, "deduplicate against previously pushed assets; -task names the dedup key (same SQLite history DB the web service uses), so the tool and a scheduled task with the same name share one pushed set")
	flag.Parse()

	logger.Init(logger.Config{Level: logger.LevelInfo, Encoding: "console", File: ""})
	startedAt := time.Now()

	engines := splitEngines(*enginesFlag)
	if len(engines) == 0 {
		logger.Errorf("no engines specified")
		os.Exit(1)
	}
	if !*localChrome && !*allowRemoteChrome {
		logger.Errorf("-local-chrome=false uses the configured remote debugger, which the SSRF guard refuses unless -allow-remote-chrome is set (trusted local diagnostics only)")
		os.Exit(1)
	}
	if *localChrome && *allowRemoteChrome {
		logger.Warnf("-allow-remote-chrome is ignored because -local-chrome is set (local guarded Chrome is used)")
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		logger.Errorf("load config: %v", err)
		os.Exit(1)
	}
	if cfg == nil {
		logger.Errorf("config is empty at %s", *configPath)
		os.Exit(1)
	}

	runQuery := strings.TrimSpace(*query)
	if runQuery == "" {
		runQuery, err = loadTaskQuery(*tasksJSON, *taskName)
		if err != nil {
			logger.Errorf("load task query: %v", err)
			os.Exit(1)
		}
		logger.Infof("loaded query from task %q (%s)", *taskName, *tasksJSON)
	}

	var resp *service.QueryResponse
	if *viaWeb {
		// Route through the running web service so browser collection uses the
		// extension bridge (paired to the user's real Chrome), which headless
		// Chrome cannot reach for FOFA. The web service owns the browser router.
		resp, err = runQueryViaWeb(*webURL, cfg, runQuery, engines, *pageSize, *timeout)
		if err != nil {
			logger.Errorf("via-web query failed: %v", err)
			os.Exit(1)
		}
	} else {
		// Build the unified service + engine adapters (mirrors cmd/unimap-web).
		svc := service.NewUnifiedServiceWithConfig(cfg)
		registerEngines(svc, cfg)
		orchestrator := svc.GetOrchestrator()

		// Build the browser DOM path: screenshot manager + CDP provider. The SSRF
		// guard refuses an externally-launched remote debugger (it cannot prove
		// guarded egress), so by default we launch a guarded local Chrome.
		screenshotMgr := buildScreenshotManager(cfg, !*localChrome, *allowRemoteChrome)
		var browserRouter service.BrowserRouter
		var screenshotApp *service.ScreenshotAppService
		if !*noBrowser && screenshotMgr != nil {
			cdpProvider := screenshot.NewCDPProvider(screenshotMgr)
			browserRouter = cdpProvider
			baseDir := strings.TrimSpace(cfg.Screenshot.BaseDir)
			if baseDir == "" {
				baseDir = utils.ScreenshotsDir()
			}
			screenshotApp = service.NewScreenshotAppServiceWithProvider(baseDir, cdpProvider)
			screenshotApp.SetFallbackToCDP(cfg.Screenshot.Extension.FallbackToCDP)
		}

		queryApp := service.NewQueryAppService(svc, orchestrator)
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()

		// Run API query in parallel with browser DOM/Bridge collection, then merge.
		if browserRouter != nil {
			merged, outcome, wfErr := queryApp.ExecuteQueryWithBrowserWorkflow(ctx, runQuery, engines, *pageSize, service.BrowserQueryWorkflowOptions{
				Action:             "collect_and_capture",
				AutoCaptureEnabled: true,
				ScreenshotApp:      screenshotApp,
				ScreenshotManager:  screenshotMgr,
				BrowserRouter:      browserRouter,
				RequireComplete:    false,
				RequirePersistence: false,
			})
			resp = merged
			reportBrowserOutcome(outcome)
			if wfErr != nil && len(merged.Assets) == 0 {
				logger.Errorf("browser workflow failed and API produced no assets: %v", wfErr)
				os.Exit(1)
			}
		} else {
			resp, err = queryApp.ExecuteQuery(ctx, runQuery, engines, *pageSize)
			if err != nil {
				logger.Errorf("API query failed: %v", err)
				os.Exit(1)
			}
		}
	}
	if resp == nil {
		logger.Errorf("query returned no response")
		os.Exit(1)
	}

	// Incremental push: keep only assets not already pushed under -task, then
	// record the new fingerprints after the workbook is pushed. State lives in
	// the same SQLite history DB the web service uses (cfg.History.DatabasePath),
	// keyed by the task name, so the tool and a scheduled task with the same
	// name share one dedup set.
	var pushRepo *history.Repository
	var onlyNewKeys []string
	if *onlyNew {
		if strings.TrimSpace(*taskName) == "" {
			logger.Errorf("-only-new requires -task (the dedup key)")
			os.Exit(1)
		}
		historyPath := strings.TrimSpace(cfg.History.DatabasePath)
		if historyPath == "" {
			historyPath = filepath.Join(utils.AppDataDir(), "history.db")
		}
		db, err := history.NewDatabase(historyPath)
		if err != nil {
			logger.Errorf("open history db for only-new: %v", err)
			os.Exit(1)
		}
		defer func() { _ = db.Close() }()
		if err := db.InitSchema(); err != nil {
			logger.Errorf("init history schema: %v", err)
			os.Exit(1)
		}
		pushRepo = history.NewRepository(db.DB())

		pushed, err := pushRepo.LoadPushedAssetKeys(*taskName)
		if err != nil {
			logger.Errorf("load pushed keys: %v", err)
			os.Exit(1)
		}
		fresh := make([]model.UnifiedAsset, 0, len(resp.Assets))
		onlyNewKeys = make([]string, 0, len(resp.Assets))
		for _, a := range resp.Assets {
			key := a.Key()
			if key == "" {
				fresh = append(fresh, a)
				continue
			}
			if _, seen := pushed[key]; seen {
				continue
			}
			fresh = append(fresh, a)
			onlyNewKeys = append(onlyNewKeys, key)
		}
		skipped := len(resp.Assets) - len(fresh)
		resp.Assets = fresh
		resp.TotalCount = len(fresh)
		logger.Infof("only-new: %d total, %d already pushed, %d new for task %q", len(fresh)+skipped, skipped, len(fresh), *taskName)
		if len(fresh) == 0 {
			printSummary(runQuery, engines, resp, "", countBrowserAssets(resp.Assets), countExtraColumns(resp.Assets), "skipped (no new assets)", startedAt)
			return
		}
	}

	if len(resp.Assets) == 0 {
		logger.Errorf("query returned 0 assets")
		os.Exit(1)
	}

	// Export the full-field Excel workbook.
	path := *outPath
	if strings.TrimSpace(path) == "" {
		path = filepath.Join("data", fmt.Sprintf("query_excel_push_%d.xlsx", time.Now().Unix()))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		logger.Errorf("create output dir: %v", err)
		os.Exit(1)
	}
	if err := exporter.NewExcelExporter().ExportFull(resp.Assets, path); err != nil {
		logger.Errorf("export full-field excel: %v", err)
		os.Exit(1)
	}

	browserCount := countBrowserAssets(resp.Assets)
	extraCount := countExtraColumns(resp.Assets)

	// Push the workbook as a WeCom file message.
	if *noPush {
		printSummary(runQuery, engines, resp, path, browserCount, extraCount, "skipped (no-push)", startedAt)
		return
	}
	ch, err := buildWeComChannel(cfg, *channelID)
	if err != nil {
		logger.Errorf("build channel %s: %v", *channelID, err)
		os.Exit(1)
	}
	pushCtx, pushCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer pushCancel()
	n := notify.TaskNotification{
		TaskID:   fmt.Sprintf("query_excel_push_%d", time.Now().Unix()),
		TaskName: "query-excel-push",
		TaskType: "query",
		Status:   "success",
		Result: fmt.Sprintf("全量字段 Excel：%d 条资产（API %d / 浏览器 %d），extra 列 %d",
			len(resp.Assets), len(resp.Assets)-browserCount, browserCount, extraCount),
		Duration:   float64(time.Since(startedAt).Milliseconds()),
		Timestamp:  time.Now(),
		ImagePaths: []string{path},
	}
	if err := ch.Send(pushCtx, n); err != nil {
		logger.Errorf("push wecom file to %s failed: %v", *channelID, err)
		os.Exit(1)
	}
	logger.Infof("wecom file pushed to channel %s", *channelID)
	if *onlyNew && len(onlyNewKeys) > 0 {
		if err := pushRepo.RecordPushedAssets(*taskName, onlyNewKeys); err != nil {
			logger.Errorf("record pushed keys for %q: %v", *taskName, err)
			os.Exit(1)
		}
		logger.Infof("recorded %d newly pushed keys for task %q", len(onlyNewKeys), *taskName)
	}
	printSummary(runQuery, engines, resp, path, browserCount, extraCount, fmt.Sprintf("pushed to %s", *channelID), startedAt)
}

// viaWebResponse mirrors the fields of web.QueryAPIPayload the tool consumes.
type viaWebResponse struct {
	Status             string               `json:"status"`
	Assets             []model.UnifiedAsset `json:"assets"`
	TotalCount         int                  `json:"totalCount"`
	EngineStats        map[string]int       `json:"engineStats"`
	Errors             []string             `json:"errors"`
	BrowserQueryErrors []string             `json:"browserQueryErrors"`
	Error              string               `json:"error"`
}

// runQueryViaWeb posts the query to the running web service's /api/v1/query with
// browser_query=true, so the web service runs the API query and browser
// collection through its own router (the extension bridge when it is the active
// mode). Auth uses X-Admin-Token read from config; the token is never logged.
func runQueryViaWeb(webURL string, cfg *config.Config, query string, engines []string, pageSize int, timeout time.Duration) (*service.QueryResponse, error) {
	form := url.Values{}
	form.Set("query", query)
	form.Set("engines", strings.Join(engines, ","))
	form.Set("page_size", strconv.Itoa(pageSize))
	form.Set("browser_query", "true")
	form.Set("browser_action", "collect_and_capture")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(webURL, "/")+"/api/v1/query", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build via-web request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// The API requires a same-origin/allowed Origin for state-changing requests
	// (CSRF protection). A local CLI client sets it to the web service's own
	// origin, exactly as the same-origin UI does.
	if origin := strings.TrimRight(webURL, "/"); origin != "" {
		req.Header.Set("Origin", origin)
	}
	if token := strings.TrimSpace(cfg.Web.Auth.AdminToken); token != "" {
		req.Header.Set("X-Admin-Token", token)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("via-web request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("read via-web response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("via-web query returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload viaWebResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse via-web response: %w", err)
	}
	if payload.Error != "" {
		return nil, fmt.Errorf("via-web query error: %s", payload.Error)
	}
	return &service.QueryResponse{
		Assets:      payload.Assets,
		TotalCount:  payload.TotalCount,
		EngineStats: payload.EngineStats,
		Errors:      append(append([]string{}, payload.Errors...), payload.BrowserQueryErrors...),
	}, nil
}

// loadConfig loads and returns the config snapshot at path.
func loadConfig(path string) (*config.Config, error) {
	mgr := config.NewManager(path)
	if err := mgr.Load(); err != nil {
		return nil, err
	}
	return mgr.GetConfig(), nil
}

// splitEngines parses a comma-separated engine list into normalized tokens.
func splitEngines(s string) []string {
	var out []string
	for _, e := range strings.Split(s, ",") {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

// registerEngines registers enabled engine adapters (API mode when credentials
// exist, Web-only otherwise), mirroring cmd/unimap-web/main.go.
func registerEngines(svc *service.UnifiedService, cfg *config.Config) {
	type engineEntry struct {
		enabled     bool
		hasCreds    bool
		supportsWeb bool
		regAPI      func()
		regWeb      func()
		name        string
	}
	engines := []engineEntry{
		{cfg.Engines.Fofa.Enabled, cfg.Engines.Fofa.APIKey != "", true,
			func() {
				svc.RegisterAdapter(adapter.NewFofaAdapter(cfg.Engines.Fofa.APIBaseURL, cfg.Engines.Fofa.APIKey, cfg.Engines.Fofa.Email, cfg.Engines.Fofa.BackupAPIKey, cfg.Engines.Fofa.BackupEmail, cfg.Engines.Fofa.QPS, time.Duration(cfg.Engines.Fofa.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewFofaAdapterWebOnly()) }, "FOFA"},
		{cfg.Engines.Hunter.Enabled, cfg.Engines.Hunter.APIKey != "", true,
			func() {
				svc.RegisterAdapter(adapter.NewHunterAdapter(cfg.Engines.Hunter.BaseURL, cfg.Engines.Hunter.APIKey, cfg.Engines.Hunter.QPS, time.Duration(cfg.Engines.Hunter.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewHunterAdapterWebOnly()) }, "Hunter"},
		{cfg.Engines.Zoomeye.Enabled, cfg.Engines.Zoomeye.APIKey != "", true,
			func() {
				svc.RegisterAdapter(adapter.NewZoomEyeAdapter(cfg.Engines.Zoomeye.BaseURL, cfg.Engines.Zoomeye.APIKey, cfg.Engines.Zoomeye.BackupAPIKey, cfg.Engines.Zoomeye.QPS, time.Duration(cfg.Engines.Zoomeye.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewZoomEyeAdapterWebOnly()) }, "ZoomEye"},
		{cfg.Engines.Quake.Enabled, cfg.Engines.Quake.APIKey != "", true,
			func() {
				svc.RegisterAdapter(adapter.NewQuakeAdapter(cfg.Engines.Quake.BaseURL, cfg.Engines.Quake.APIKey, cfg.Engines.Quake.BackupAPIKey, cfg.Engines.Quake.QPS, time.Duration(cfg.Engines.Quake.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewQuakeAdapterWebOnly()) }, "Quake"},
		{cfg.Engines.Shodan.Enabled, cfg.Engines.Shodan.APIKey != "", true,
			func() {
				svc.RegisterAdapter(adapter.NewShodanAdapter(cfg.Engines.Shodan.BaseURL, cfg.Engines.Shodan.APIKey, cfg.Engines.Shodan.BackupAPIKey, cfg.Engines.Shodan.QPS, time.Duration(cfg.Engines.Shodan.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewShodanAdapterWebOnly()) }, "Shodan"},
		{cfg.Engines.Censys.Enabled, cfg.Engines.Censys.APIID != "" && cfg.Engines.Censys.APISecret != "", true,
			func() {
				svc.RegisterAdapter(adapter.NewCensysAdapter(cfg.Engines.Censys.BaseURL, cfg.Engines.Censys.APIID, cfg.Engines.Censys.APISecret, cfg.Engines.Censys.BackupAPIID, cfg.Engines.Censys.BackupAPISecret, cfg.Engines.Censys.QPS, time.Duration(cfg.Engines.Censys.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewCensysAdapterWebOnly()) }, "Censys"},
		{cfg.Engines.Daydaymap.Enabled, cfg.Engines.Daydaymap.APIKey != "", true,
			func() {
				svc.RegisterAdapter(adapter.NewDayDayMapAdapter(cfg.Engines.Daydaymap.BaseURL, cfg.Engines.Daydaymap.APIKey, cfg.Engines.Daydaymap.BackupAPIKey, cfg.Engines.Daydaymap.QPS, time.Duration(cfg.Engines.Daydaymap.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewDayDayMapAdapterWebOnly()) }, "DayDayMap"},
	}
	for _, e := range engines {
		if !e.enabled {
			continue
		}
		if e.hasCreds {
			e.regAPI()
			logger.Infof("%s engine registered (API mode)", e.name)
		} else if e.supportsWeb {
			e.regWeb()
			logger.Infof("%s engine registered (Web-only mode)", e.name)
		} else {
			logger.Warnf("%s engine is enabled but requires complete API credentials; registration skipped", e.name)
		}
	}
}

// buildScreenshotManager mirrors web.Server.initScreenshotManager and loads
// per-engine cookies so the browser DOM path stays logged in. When useRemoteDebug
// is false (the tool default), the configured remote debugger is ignored so the
// guarded browser session launches its own Chrome — remote Chrome is refused by
// the SSRF guard because it cannot prove guarded egress.
func buildScreenshotManager(cfg *config.Config, useRemoteDebug, allowRemoteDebug bool) *screenshot.Manager {
	if cfg == nil || !cfg.Screenshot.Enabled {
		logger.Warnf("screenshot disabled in config; browser DOM path skipped")
		return nil
	}
	headless := true
	if cfg.Screenshot.Headless != nil {
		headless = *cfg.Screenshot.Headless
	}
	remoteDebugURL := ""
	if useRemoteDebug {
		remoteDebugURL = strings.TrimSpace(cfg.Screenshot.ChromeRemoteDebugURL)
		if remoteDebugURL != "" && !remoteDebuggerAvailable(remoteDebugURL) {
			logger.Warnf("remote debugger not available at %s; browser DOM path skipped", remoteDebugURL)
			return nil
		}
	}
	mgr := screenshot.NewManager(screenshot.Config{
		BaseDir:        cfg.Screenshot.BaseDir,
		ChromePath:     cfg.Screenshot.ChromePath,
		ProxyServer:    cfg.Screenshot.ProxyServer,
		UserDataDir:    cfg.Screenshot.ChromeUserDataDir,
		ProfileDir:     cfg.Screenshot.ChromeProfileDir,
		RemoteDebugURL: remoteDebugURL,
		Headless:       headless,
		NoSandbox:      cfg.Screenshot.NoSandbox,
		Timeout:        time.Duration(cfg.Screenshot.Timeout) * time.Second,
		WindowWidth:    cfg.Screenshot.WindowWidth,
		WindowHeight:   cfg.Screenshot.WindowHeight,
		WaitTime:       time.Duration(cfg.Screenshot.WaitTime) * time.Millisecond,
		MaxSessions:    cfg.Screenshot.MaxSessions,
	})
	if useRemoteDebug && allowRemoteDebug {
		// Explicit opt-in for this local diagnostic: attach to the user's own
		// logged-in remote Chrome. The CDP-level SSRF interceptor still blocks
		// private/loopback URLs per request.
		mgr.SetAllowRemoteDebug(true)
	}
	for _, engine := range browserEngines() {
		if cookies := engineCookies(cfg, engine); len(cookies) > 0 {
			mgr.SetCookies(engine, convertConfigCookies(cookies))
		}
	}
	return mgr
}

// loadTaskQuery reads the query of a named scheduler task from the tasks JSON
// file. The fofa_ynmobile_daily task holds the [REDACTED] body query in raw FOFA
// syntax, which is what the user chose for this run.
func loadTaskQuery(tasksJSON, taskName string) (string, error) {
	data, err := os.ReadFile(tasksJSON)
	if err != nil {
		return "", fmt.Errorf("read tasks json: %w", err)
	}
	var tasks []struct {
		Name    string `json:"name"`
		Payload struct {
			Query string `json:"query"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(data, &tasks); err != nil {
		return "", fmt.Errorf("parse tasks json: %w", err)
	}
	for _, t := range tasks {
		if strings.TrimSpace(t.Name) == strings.TrimSpace(taskName) {
			q := strings.TrimSpace(t.Payload.Query)
			if q == "" {
				return "", fmt.Errorf("task %q has an empty query", taskName)
			}
			return q, nil
		}
	}
	return "", fmt.Errorf("task %q not found in %s", taskName, tasksJSON)
}

// browserEngines mirrors web.cookie_handlers.browserEngines.
func browserEngines() []string {
	return []string{"fofa", "hunter", "zoomeye", "quake", "shodan", "censys", "daydaymap"}
}

// engineCookies mirrors web.cookie_handlers.engineCookies.
func engineCookies(cfg *config.Config, engine string) []config.Cookie {
	switch engine {
	case "fofa":
		return cfg.Engines.Fofa.Cookies
	case "hunter":
		return cfg.Engines.Hunter.Cookies
	case "quake":
		return cfg.Engines.Quake.Cookies
	case "zoomeye":
		return cfg.Engines.Zoomeye.Cookies
	case "shodan":
		return cfg.Engines.Shodan.Cookies
	case "censys":
		return cfg.Engines.Censys.Cookies
	case "daydaymap":
		return cfg.Engines.Daydaymap.Cookies
	}
	return nil
}

// convertConfigCookies mirrors web.Server.convertConfigCookies.
func convertConfigCookies(cfgCookies []config.Cookie) []screenshot.Cookie {
	cookies := make([]screenshot.Cookie, len(cfgCookies))
	for i, c := range cfgCookies {
		cookies[i] = screenshot.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		}
	}
	return cookies
}

// remoteDebuggerAvailable mirrors web.cdp_handlers.isRemoteDebuggerAvailable.
func remoteDebuggerAvailable(rawURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(strings.TrimRight(rawURL, "/") + "/json/version")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// buildWeComChannel finds a wecom channel by id and forces a file message type
// so the workbook is delivered as a file (≤20MB) rather than markdown.
func buildWeComChannel(cfg *config.Config, id string) (notify.NotifyChannel, error) {
	for _, cc := range cfg.Notifications.Channels {
		if strings.EqualFold(cc.ID, id) && strings.EqualFold(cc.Type, "wecom") {
			return notify.NewWeComChannelWithOptions(cc.ID, cc.WebhookURL, cc.Enabled, cc.AllowPrivateIP, notify.WeComOptions{
				MsgType: notify.WeComMsgTypeFile,
			})
		}
	}
	return nil, fmt.Errorf("wecom channel %q not found in config notifications", id)
}

// reportBrowserOutcome prints browser collection results and errors. It never
// prints webhook URLs or credentials.
func reportBrowserOutcome(o service.BrowserQueryOutcome) {
	for _, r := range o.CollectedResults {
		logger.Infof("browser collect: engine=%s assets=%d total=%d", r.Engine, len(r.Assets), r.Total)
	}
	for _, e := range o.Errors {
		logger.Warnf("browser error: %s", e)
	}
	for engine, p := range o.AutoCapturedPaths {
		logger.Infof("browser capture: engine=%s path=%s", engine, p)
	}
	for _, e := range o.AutoCaptureErrors {
		logger.Warnf("browser capture error: %s", e)
	}
}

// countBrowserAssets counts assets tagged by the browser workflow
// (Extra["collection_method"] == "browser").
func countBrowserAssets(assets []model.UnifiedAsset) int {
	n := 0
	for _, a := range assets {
		if v, ok := a.Extra["collection_method"].(string); ok && v == "browser" {
			n++
		}
	}
	return n
}

// countExtraColumns returns the number of distinct Extra keys across assets.
func countExtraColumns(assets []model.UnifiedAsset) int {
	set := make(map[string]struct{})
	for _, a := range assets {
		for k := range a.Extra {
			set[k] = struct{}{}
		}
	}
	return len(set)
}

// printSummary prints a compact, secret-free summary of the run.
func printSummary(query string, engines []string, resp *service.QueryResponse, path string, browserCount, extraCount int, pushStatus string, startedAt time.Time) {
	fmt.Println("==============================================")
	fmt.Printf("query       : %s\n", query)
	fmt.Printf("engines     : %v\n", engines)
	fmt.Printf("total assets: %d (api %d / browser %d)\n", len(resp.Assets), len(resp.Assets)-browserCount, browserCount)
	fmt.Printf("engine stats: %v\n", resp.EngineStats)
	fmt.Printf("extra cols  : %d\n", extraCount)
	if len(resp.Errors) > 0 {
		fmt.Printf("errors      : %d\n", len(resp.Errors))
		for _, e := range resp.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}
	fmt.Printf("excel file  : %s\n", path)
	fmt.Printf("push        : %s\n", pushStatus)
	fmt.Printf("duration    : %s\n", time.Since(startedAt).Round(time.Second))
	fmt.Println("==============================================")
}
