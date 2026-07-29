package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/alerting"
	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/distributed"
	"github.com/unimap/project/internal/history"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/service"
	"github.com/unimap/project/internal/tamper"
)

type failingSchedulerAdapter struct{ name string }

func (a *failingSchedulerAdapter) Name() string { return a.name }
func (a *failingSchedulerAdapter) Translate(*model.UQLAST) (string, error) {
	return "translated", nil
}
func (a *failingSchedulerAdapter) Search(context.Context, string, int, int) (*model.EngineResult, error) {
	return nil, fmt.Errorf("authentication failed")
}
func (a *failingSchedulerAdapter) Normalize(*model.EngineResult) ([]model.UnifiedAsset, error) {
	return nil, nil
}
func (a *failingSchedulerAdapter) GetQuota() (*model.QuotaInfo, error) { return nil, nil }
func (a *failingSchedulerAdapter) IsWebOnly() bool                     { return false }

type successfulSchedulerAdapter struct {
	name   string
	assets []model.UnifiedAsset
}

func (a *successfulSchedulerAdapter) Name() string { return a.name }
func (a *successfulSchedulerAdapter) Translate(*model.UQLAST) (string, error) {
	return "translated", nil
}
func (a *successfulSchedulerAdapter) Search(context.Context, string, int, int) (*model.EngineResult, error) {
	assets := a.assets
	if len(assets) == 0 {
		assets = []model.UnifiedAsset{{
			IP: "192.0.2.10", Port: 443, URL: "https://api.example.test",
		}}
	}
	return &model.EngineResult{
		EngineName:     a.name,
		NormalizedData: assets,
	}, nil
}
func (a *successfulSchedulerAdapter) Normalize(result *model.EngineResult) ([]model.UnifiedAsset, error) {
	return result.NormalizedData, nil
}
func (a *successfulSchedulerAdapter) GetQuota() (*model.QuotaInfo, error) { return nil, nil }
func (a *successfulSchedulerAdapter) IsWebOnly() bool                     { return false }

type combinedSchedulerBrowserRouter struct {
	path string
}

func (r *combinedSchedulerBrowserRouter) OpenSearchEngineResult(context.Context, string, string) (string, error) {
	return "https://search.example.test", nil
}

func (r *combinedSchedulerBrowserRouter) CollectSearchEngineResult(context.Context, string, string, string) ([]collection.CollectResult, error) {
	return nil, errors.New("separate collect should not be used")
}

func (r *combinedSchedulerBrowserRouter) CollectAndCaptureSearchEngineResult(_ context.Context, engine, query, _ string) ([]collection.CollectResult, string, error) {
	return []collection.CollectResult{{
		Engine: engine,
		Query:  query,
		Total:  1,
		Assets: []model.UnifiedAsset{{
			IP: "198.51.100.20", Port: 8443, URL: "https://bridge.example.test",
		}},
	}}, r.path, nil
}

// ===== Bridge client mock for scheduler tests =====

type mockBridgeSchedulerClient struct {
	awaitResult screenshot.BridgeResult
	awaitErr    error
}

func (m *mockBridgeSchedulerClient) SubmitTask(ctx context.Context, task screenshot.BridgeTask) error {
	return nil
}
func (m *mockBridgeSchedulerClient) AwaitResult(ctx context.Context, requestID string) (screenshot.BridgeResult, error) {
	if m.awaitErr != nil {
		return screenshot.BridgeResult{}, m.awaitErr
	}
	return m.awaitResult, nil
}

// ===== QueryRunner Execute tests =====

func TestQueryRunner_Execute_NilService(t *testing.T) {
	r := NewQueryRunner(nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{Query: "test"})
	if err == nil {
		t.Fatal("expected error for nil service")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("error should mention 'not available': %v", err)
	}
}

func TestQueryRunner_Execute_MissingQuery(t *testing.T) {
	r := NewQueryRunner(service.NewQueryAppService(nil, nil))
	_, err := r.Execute(context.Background(), &model.TaskPayload{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error should mention 'missing': %v", err)
	}
}

func TestQueryRunner_Execute_AllEnginesFailed(t *testing.T) {
	svc := service.NewUnifiedService()
	svc.RegisterAdapter(&failingSchedulerAdapter{name: "failed-engine"})
	r := NewQueryRunner(service.NewQueryAppService(svc, svc.GetOrchestrator()))

	_, err := r.Execute(context.Background(), &model.TaskPayload{
		Query:   `ip="132.232.231.41"`,
		Engines: []string{"failed-engine"},
	})

	if err == nil {
		t.Fatal("expected all-engine failure to fail the scheduled query")
	}
	if !strings.Contains(err.Error(), "all query engines failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueryRunner_Execute_APIQueryIncludesAssetDetailsForNotification(t *testing.T) {
	svc := service.NewUnifiedService()
	svc.RegisterAdapter(&successfulSchedulerAdapter{name: "fofa"})
	runner := NewQueryRunner(service.NewQueryAppService(svc, svc.GetOrchestrator()))

	result, err := runner.Execute(context.Background(), &model.TaskPayload{
		Query: `port="443"`, Engines: []string{"fofa"}, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("execute API query: %v", err)
	}
	for _, want := range []string{"📋 查询结果明细", "https://api.example.test", "192.0.2.10:443"} {
		if !strings.Contains(result, want) {
			t.Fatalf("query notification result lacks %q:\n%s", want, result)
		}
	}
}

func TestQueryRunner_Execute_QueryNotificationDetailLimit(t *testing.T) {
	svc := service.NewUnifiedService()
	svc.RegisterAdapter(&successfulSchedulerAdapter{name: "fofa", assets: []model.UnifiedAsset{
		{IP: "192.0.2.10", Port: 443, URL: "https://first.example.test"},
		{IP: "192.0.2.11", Port: 8443, URL: "https://second.example.test"},
	}})
	runner := NewQueryRunner(service.NewQueryAppService(svc, svc.GetOrchestrator()))

	result, err := runner.Execute(context.Background(), &model.TaskPayload{
		Query:                   `port="443"`,
		Engines:                 []string{"fofa"},
		PageSize:                10,
		NotificationDetailLimit: 1,
	})
	if err != nil {
		t.Fatalf("execute API query: %v", err)
	}
	firstShown := strings.Contains(result, "https://first.example.test")
	secondShown := strings.Contains(result, "https://second.example.test")
	if firstShown == secondShown {
		t.Fatalf("query notification detail limit not applied:\n%s", result)
	}
	if !strings.Contains(result, "另有 1 条结果已持久化") {
		t.Fatalf("query notification omits persisted remainder notice:\n%s", result)
	}
}

func TestQueryRunner_Execute_QueryNotificationOmitsSensitiveAssetFields(t *testing.T) {
	svc := service.NewUnifiedService()
	svc.RegisterAdapter(&successfulSchedulerAdapter{name: "fofa", assets: []model.UnifiedAsset{{
		IP:          "192.0.2.10",
		Port:        443,
		URL:         "https://safe.example.test",
		Title:       "line one\n<at>all</at>",
		BodySnippet: "body-secret-must-not-be-sent",
		Headers:     map[string]string{"Authorization": "header-secret-must-not-be-sent"},
		Extra:       map[string]any{"cookie": "extra-secret-must-not-be-sent"},
	}}})
	runner := NewQueryRunner(service.NewQueryAppService(svc, svc.GetOrchestrator()))

	result, err := runner.Execute(context.Background(), &model.TaskPayload{
		Query: `port="443"`, Engines: []string{"fofa"}, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("execute API query: %v", err)
	}
	for _, forbidden := range []string{"body-secret", "header-secret", "extra-secret", "<at>"} {
		if strings.Contains(result, forbidden) {
			t.Fatalf("query notification leaked or preserved unsafe field %q:\n%s", forbidden, result)
		}
	}
	if !strings.Contains(result, "line one ‹at›all‹/at›") {
		t.Fatalf("query notification did not safely normalize the title:\n%s", result)
	}
}

func TestQueryRunner_Execute_QueryNotificationCapsDetailBodySize(t *testing.T) {
	assets := make([]model.UnifiedAsset, 0, 100)
	for i := 0; i < 100; i++ {
		assets = append(assets, model.UnifiedAsset{
			IP: fmt.Sprintf("192.0.2.%d", i+1), Port: 443,
			URL:    fmt.Sprintf("https://asset-%d.example.test/%s", i, strings.Repeat("x", 120)),
			Title:  strings.Repeat("title", 32),
			Server: strings.Repeat("server", 27),
			Org:    strings.Repeat("organization", 14),
		})
	}
	svc := service.NewUnifiedService()
	svc.RegisterAdapter(&successfulSchedulerAdapter{name: "fofa", assets: assets})
	runner := NewQueryRunner(service.NewQueryAppService(svc, svc.GetOrchestrator()))

	result, err := runner.Execute(context.Background(), &model.TaskPayload{
		Query: `port="443"`, Engines: []string{"fofa"}, PageSize: 100,
		NotificationDetailLimit: 100,
	})
	if err != nil {
		t.Fatalf("execute API query: %v", err)
	}
	if len(result) > maxQueryNotificationDetailBytes+2048 {
		t.Fatalf("query notification size = %d, expected bounded detail body", len(result))
	}
	if !strings.Contains(result, "条结果已持久化，通知中未展开") {
		t.Fatalf("query notification omits byte-cap remainder notice:\n%s", result)
	}
}

func TestQueryRunner_Execute_BridgeCollectCapturePersistsCombinedResults(t *testing.T) {
	db, err := history.NewDatabase(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("open history database: %v", err)
	}
	defer db.Close()
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init history schema: %v", err)
	}
	repo := history.NewRepository(db.DB())

	unified := service.NewUnifiedService()
	unified.RegisterAdapter(&successfulSchedulerAdapter{name: "fofa"})
	queryApp := service.NewQueryAppService(unified, unified.GetOrchestrator())
	queryApp.SetHistoryRepository(repo)

	screenshotPath := filepath.Join(t.TempDir(), "fofa.png")
	if err := os.WriteFile(screenshotPath, []byte("png"), 0o600); err != nil {
		t.Fatalf("write screenshot fixture: %v", err)
	}
	router := &combinedSchedulerBrowserRouter{path: screenshotPath}
	screenshotApp := service.NewScreenshotAppService(t.TempDir())
	runner := NewQueryRunnerWithBrowser(queryApp, screenshotApp, &screenshot.Manager{}, router)

	result, err := runner.Execute(context.Background(), &model.TaskPayload{
		Query:         `port="443"`,
		Engines:       []string{"fofa"},
		PageSize:      10,
		BrowserQuery:  true,
		BrowserAction: "collect_and_capture",
		QueryID:       "scheduled-query-1",
	})
	if err != nil {
		t.Fatalf("execute closed-loop query: %v", err)
	}
	if !strings.Contains(result, screenshotPath) {
		t.Fatalf("task result does not expose screenshot path for notification: %q", result)
	}
	if got := extractImagePaths(result); len(got) != 1 || got[0] != screenshotPath {
		t.Fatalf("notification image paths = %#v, want %q", got, screenshotPath)
	}
	for _, want := range []string{"https://api.example.test", "https://bridge.example.test"} {
		if !strings.Contains(result, want) {
			t.Fatalf("Bridge query notification result lacks %q:\n%s", want, result)
		}
	}
	notification := (&Scheduler{}).buildNotificationMessage(
		&ScheduledTask{ID: "scheduled-query-1", Name: "closed loop", Type: TaskQuery},
		ExecutionRecord{Status: "success", Result: result},
	)
	if !strings.Contains(notification.Result, "https://bridge.example.test") {
		t.Fatalf("notification payload lost Bridge asset details: %#v", notification)
	}
	if len(notification.ImagePaths) != 1 || notification.ImagePaths[0] != screenshotPath {
		t.Fatalf("notification payload image paths = %#v, want %q", notification.ImagePaths, screenshotPath)
	}

	histories, total, err := repo.ListHistory(string(history.OpTypeQuery), 10, 0)
	if err != nil {
		t.Fatalf("list query history: %v", err)
	}
	if total != 1 || len(histories) != 1 {
		t.Fatalf("query history count = %d/%d, want one combined record", total, len(histories))
	}
	var summary struct {
		QueryID     string            `json:"browser_query_id"`
		Screenshots map[string]string `json:"browser_screenshots"`
	}
	if err := json.Unmarshal([]byte(histories[0].Summary), &summary); err != nil {
		t.Fatalf("decode query history summary: %v", err)
	}
	if summary.QueryID != "scheduled-query-1" || summary.Screenshots["fofa"] != screenshotPath {
		t.Fatalf("query history summary lacks Bridge correlation metadata: %#v", summary)
	}
	rows, err := repo.GetResults(histories[0].ID)
	if err != nil {
		t.Fatalf("get persisted query results: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("persisted result rows = %d, want API + Bridge assets", len(rows))
	}
}

func TestQueryRunner_Execute_BridgeWorkflowFailsWhenScreenshotIsMissing(t *testing.T) {
	db, err := history.NewDatabase(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("open history database: %v", err)
	}
	defer db.Close()
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init history schema: %v", err)
	}
	repo := history.NewRepository(db.DB())

	unified := service.NewUnifiedService()
	unified.RegisterAdapter(&successfulSchedulerAdapter{name: "fofa"})
	queryApp := service.NewQueryAppService(unified, unified.GetOrchestrator())
	queryApp.SetHistoryRepository(repo)

	router := &combinedSchedulerBrowserRouter{path: filepath.Join(t.TempDir(), "missing.png")}
	runner := NewQueryRunnerWithBrowser(queryApp, service.NewScreenshotAppService(t.TempDir()), &screenshot.Manager{}, router)
	_, err = runner.Execute(context.Background(), &model.TaskPayload{
		Query:         `port="443"`,
		Engines:       []string{"fofa"},
		BrowserQuery:  true,
		BrowserAction: "collect_and_capture",
	})
	if err == nil || !strings.Contains(err.Error(), "screenshot unavailable") {
		t.Fatalf("missing screenshot error = %v", err)
	}

	histories, total, err := repo.ListHistory(string(history.OpTypeQuery), 10, 0)
	if err != nil {
		t.Fatalf("list query history: %v", err)
	}
	if total != 1 || len(histories) != 1 || histories[0].Status != "error" {
		t.Fatalf("failed workflow history = %#v (total %d), want one error record", histories, total)
	}
}

// ===== SearchScreenshotRunner Execute tests =====

func TestSearchScreenshotRunner_Execute_NilService(t *testing.T) {
	r := NewSearchScreenshotRunner(nil, nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{Query: "test", Extra: map[string]any{"engine": "fofa"}})
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

func TestSearchScreenshotRunner_Execute_MissingParams(t *testing.T) {
	svc := service.NewScreenshotAppService("./screenshots")
	r := NewSearchScreenshotRunner(svc, nil)

	_, err := r.Execute(context.Background(), &model.TaskPayload{Query: "test"})
	if err == nil {
		t.Fatal("expected error for missing engine")
	}

	_, err = r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{"engine": "fofa"}})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

// ===== BatchScreenshotRunner Execute tests =====

func TestBatchScreenshotRunner_Execute_NilService(t *testing.T) {
	r := NewBatchScreenshotRunner(nil, nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{URLs: []string{"http://example.com"}})
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

func TestBatchScreenshotRunner_Execute_MissingURLs(t *testing.T) {
	svc := service.NewScreenshotAppService("./screenshots")
	r := NewBatchScreenshotRunner(svc, nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{})
	if err == nil {
		t.Fatal("expected error for missing urls")
	}
}

// ===== TamperCheckRunner Execute tests =====

func TestTamperCheckRunner_Execute_NilService(t *testing.T) {
	r := NewTamperCheckRunner(nil, nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{URLs: []string{"http://example.com"}})
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

func TestTamperCheckRunner_Execute_MissingURLs(t *testing.T) {
	svc := service.NewTamperAppService("", nil)
	r := NewTamperCheckRunner(svc, nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{})
	if err == nil {
		t.Fatal("expected error for missing urls")
	}
}

// ===== URLReachabilityRunner Execute tests =====

func TestURLReachabilityRunner_Execute_NilService(t *testing.T) {
	r := NewURLReachabilityRunner(nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{URLs: []string{"http://example.com"}})
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

func TestURLReachabilityRunner_Execute_MissingURLs(t *testing.T) {
	r := NewURLReachabilityRunner(service.NewMonitorAppService(nil))
	_, err := r.Execute(context.Background(), &model.TaskPayload{})
	if err == nil {
		t.Fatal("expected error for missing urls")
	}
}

// ===== CookieVerifyRunner Execute tests =====

func TestCookieVerifyRunner_Execute_NilMgr(t *testing.T) {
	r := NewCookieVerifyRunner(nil, nil)
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil manager")
	}
}

func TestCookieVerifyRunner_Execute_DefaultEngines(t *testing.T) {
	mgr := &screenshot.Manager{}
	r := NewCookieVerifyRunner(nil, mgr)
	result, err := r.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "fofa") {
		t.Errorf("result should mention fofa: %s", result)
	}
	if !strings.Contains(result, "未配置 Cookie") {
		t.Errorf("result should mention 未配置 Cookie: %s", result)
	}
}

func TestCookieVerifyRunner_Execute_SpecificEngines(t *testing.T) {
	mgr := &screenshot.Manager{}
	r := NewCookieVerifyRunner(nil, mgr)
	result, err := r.Execute(context.Background(), &model.TaskPayload{Engines: []string{"fofa", "custom"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "fofa") {
		t.Errorf("result should mention fofa: %s", result)
	}
	if !strings.Contains(result, "custom") {
		t.Errorf("result should mention custom: %s", result)
	}
}

// ===== LoginStatusCheckRunner Execute tests =====

func TestLoginStatusCheckRunner_Execute_NilMgr(t *testing.T) {
	r := NewLoginStatusCheckRunner(nil)
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil manager")
	}
}

// ===== DistributedSubmitRunner Execute tests =====

func TestDistributedSubmitRunner_Execute_NilQueue(t *testing.T) {
	r := NewDistributedSubmitRunner(nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{"task_type": "scan"}})
	if err == nil {
		t.Fatal("expected error for nil queue")
	}
}

func TestDistributedSubmitRunner_Execute_MissingTaskType(t *testing.T) {
	q := distributed.NewTaskQueue()
	r := NewDistributedSubmitRunner(q)
	_, err := r.Execute(context.Background(), &model.TaskPayload{})
	if err == nil {
		t.Fatal("expected error for missing task_type")
	}
}

func TestDistributedSubmitRunner_Execute_Success(t *testing.T) {
	q := distributed.NewTaskQueue()
	r := NewDistributedSubmitRunner(q)
	result, err := r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{
		"task_type":       "port_scan",
		"task_payload":    map[string]any{"target": "example.com"},
		"priority":        5,
		"timeout_seconds": 60,
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "分布式任务已提交") {
		t.Errorf("result should mention '分布式任务已提交': %s", result)
	}
	if !strings.Contains(result, "port_scan") {
		t.Errorf("result should mention task type: %s", result)
	}
}

// ===== ExportRunner Execute tests =====

func TestExportRunner_Execute_NilDeps(t *testing.T) {
	r := NewExportRunner(nil, nil, "/tmp")
	_, err := r.Execute(context.Background(), &model.TaskPayload{Query: "test"})
	if err == nil {
		t.Fatal("expected error for nil deps")
	}
}

func TestExportRunner_Execute_MissingQuery(t *testing.T) {
	r := NewExportRunner(service.NewQueryAppService(nil, nil), adapter.NewEngineOrchestrator(), "/tmp")
	_, err := r.Execute(context.Background(), &model.TaskPayload{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

// ===== PortScanRunner Execute tests =====

func TestPortScanRunner_Execute_NilService(t *testing.T) {
	r := NewPortScanRunner(nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{URLs: []string{"http://example.com"}})
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

func TestPortScanRunner_Execute_MissingURLs(t *testing.T) {
	r := NewPortScanRunner(service.NewMonitorAppService(nil))
	_, err := r.Execute(context.Background(), &model.TaskPayload{})
	if err == nil {
		t.Fatal("expected error for missing urls")
	}
}

func TestPortScanRunner_Execute_UsesConfiguredStringPorts(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	r := NewPortScanRunner(service.NewMonitorAppService(nil))

	result, err := r.Execute(context.Background(), &model.TaskPayload{
		URLs: []string{"http://127.0.0.1"},
		Extra: map[string]any{
			"ports":       []any{strconv.Itoa(port)},
			"concurrency": 1,
		},
	})

	if err != nil {
		t.Fatalf("execute port scan: %v", err)
	}
	if !strings.Contains(result, fmt.Sprintf("端口 [%d]", port)) {
		t.Fatalf("configured port was discarded: %s", result)
	}
}

func TestPortScanRunner_RejectsInvalidPortRange(t *testing.T) {
	r := NewPortScanRunner(service.NewMonitorAppService(nil))
	_, err := r.Execute(context.Background(), &model.TaskPayload{
		URLs:  []string{"https://example.com"},
		Extra: map[string]any{"port_spec": "443,9000-8000"},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid port specification") {
		t.Fatalf("expected invalid port specification error, got %v", err)
	}
}

func TestPortScanRunnerRejectsInvalidAuthorizedTarget(t *testing.T) {
	r := NewPortScanRunner(service.NewMonitorAppService(nil))
	_, err := r.Execute(context.Background(), &model.TaskPayload{
		URLs: []string{"https://example.com"},
		Extra: map[string]any{
			"port_spec":          "443",
			"authorized_targets": []any{"not-an-ip"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "authorized target") {
		t.Fatalf("expected invalid authorized target error, got %v", err)
	}
}

// ===== ScreenshotCleanupRunner Execute tests =====

func TestScreenshotCleanupRunner_Execute_NilService(t *testing.T) {
	r := NewScreenshotCleanupRunner(nil, 30)
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

// ===== TamperCleanupRunner Execute tests =====

func TestTamperCleanupRunner_Execute_NilService(t *testing.T) {
	r := NewTamperCleanupRunner(nil, 90)
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

// ===== QuotaMonitorRunner Execute tests =====

func TestQuotaMonitorRunner_Execute_NilOrchestrator(t *testing.T) {
	r := NewQuotaMonitorRunner(nil, 10)
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil orchestrator")
	}
}

func TestQuotaMonitorRunner_Execute_NoAdapters(t *testing.T) {
	orch := adapter.NewEngineOrchestrator()
	r := NewQuotaMonitorRunner(orch, 10)
	result, err := r.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "no engine adapters") {
		t.Errorf("result should mention no adapters: %s", result)
	}
}

// ===== AlertSummaryRunner Execute tests =====

func TestAlertSummaryRunner_Execute_NilManager(t *testing.T) {
	r := NewAlertSummaryRunner(nil)
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil manager")
	}
}

func TestAlertSummaryRunner_Execute_EmptyRecords(t *testing.T) {
	m := alerting.NewManager()
	r := NewAlertSummaryRunner(m)
	result, err := r.Execute(context.Background(), &model.TaskPayload{MaxAgeDays: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "总计: 0") {
		t.Errorf("result should show 总计: 0: %s", result)
	}
}

func TestAlertSummaryRunner_Execute_WithRecords(t *testing.T) {
	m := alerting.NewManager()
	m.SendInfo(alerting.AlertTypeTamper, "t1", "tamper detected", nil, "s", "u1")
	m.SendWarning(alerting.AlertTypeSystem, "t2", "system alert", nil, "s", "u2")

	r := NewAlertSummaryRunner(m)
	result, err := r.Execute(context.Background(), &model.TaskPayload{MaxAgeDays: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "总计: 2") {
		t.Errorf("result should show 总计: 2: %s", result)
	}
}

// ===== BaselineRefreshRunner Execute tests =====

func TestBaselineRefreshRunner_Execute_NilService(t *testing.T) {
	r := NewBaselineRefreshRunner(nil)
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

func TestBaselineRefreshRunner_Execute_UsesPageLoader(t *testing.T) {
	sentinel := errors.New("guarded loader unavailable")
	var calls atomic.Int32
	loader := tamper.BrowserPageLoaderFunc(func(context.Context, string) (string, string, error) {
		calls.Add(1)
		return "", "", sentinel
	})
	r := NewBaselineRefreshRunner(service.NewTamperAppService(t.TempDir(), nil), loader)

	result, err := r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{
		"urls": []string{"https://example.test"},
	}})
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("page loader calls = %d, want 1", calls.Load())
	}
	if !strings.Contains(result, "失败: 1") {
		t.Fatalf("result should report failed refresh, got %q", result)
	}
}

// ===== URLImportRunner Execute tests =====

func TestURLImportRunner_Execute_Unconfigured(t *testing.T) {
	r := NewURLImportRunner("")
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for unconfigured import dir")
	}
}

func TestURLImportRunner_Execute_NoFiles(t *testing.T) {
	dir := t.TempDir()
	r := NewURLImportRunner(dir)
	result, err := r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{
		"file_pattern": "*.txt",
		"max_lines":    1000,
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "未找到匹配文件") {
		t.Errorf("result should mention 未找到匹配文件: %s", result)
	}
}

func TestURLImportRunner_Execute_WithFile(t *testing.T) {
	dir := t.TempDir()
	content := "http://example.com\nhttp://example.org\n# this is a comment\n\nhttp://example.net"
	err := os.WriteFile(filepath.Join(dir, "urls.txt"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	r := NewURLImportRunner(dir)
	result, err := r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{
		"file_pattern": "*.txt",
		"max_lines":    10,
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "3 条 URL") {
		t.Errorf("result should mention 3 条 URL: %s", result)
	}
}

func TestURLImportRunner_Execute_MaxLines(t *testing.T) {
	dir := t.TempDir()
	lines := ""
	for i := 0; i < 100; i++ {
		lines += "http://example.com/page\n"
	}
	err := os.WriteFile(filepath.Join(dir, "urls.txt"), []byte(lines), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	r := NewURLImportRunner(dir)
	result, err := r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{
		"file_pattern": "*.txt",
		"max_lines":    5,
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "5 条 URL") {
		t.Errorf("result should mention 5 条 URL (limited by max_lines): %s", result)
	}
}

func TestReadURLsFromFile_EdgeCases(t *testing.T) {
	dir := t.TempDir()

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(dir, "empty.txt")
		os.WriteFile(path, []byte(""), 0644)
		urls, err := readURLsFromFile(path, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(urls) != 0 {
			t.Errorf("expected 0 URLs, got %d", len(urls))
		}
	})

	t.Run("comments and blank lines", func(t *testing.T) {
		path := filepath.Join(dir, "comments.txt")
		os.WriteFile(path, []byte("# comment\n\nhttp://a.com\n\n# another comment\nhttp://b.com\n"), 0644)
		urls, err := readURLsFromFile(path, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(urls) != 2 {
			t.Errorf("expected 2 URLs, got %d", len(urls))
		}
	})

	t.Run("max lines", func(t *testing.T) {
		path := filepath.Join(dir, "many.txt")
		content := "http://a.com\nhttp://b.com\nhttp://c.com\nhttp://d.com\n"
		os.WriteFile(path, []byte(content), 0644)
		urls, err := readURLsFromFile(path, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(urls) != 2 {
			t.Errorf("expected 2 URLs, got %d", len(urls))
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := readURLsFromFile(filepath.Join(dir, "does_not_exist.txt"), 100)
		if err == nil {
			t.Fatal("expected error for non-existent file")
		}
	})
}

// ===== PluginHealthRunner Execute tests =====

func TestPluginHealthRunner_Execute_NilService(t *testing.T) {
	r := NewPluginHealthRunner(nil)
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

// ===== BridgeTokenRotateRunner Execute tests =====

func TestBridgeTokenRotateRunner_Execute_NilService(t *testing.T) {
	r := NewBridgeTokenRotateRunner(nil)
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

func TestBridgeTokenRotateRunner_Execute_NotStarted(t *testing.T) {
	client := &mockBridgeSchedulerClient{}
	svc := screenshot.NewBridgeService(client, 5, 5*time.Second)
	r := NewBridgeTokenRotateRunner(svc)
	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for not-started bridge")
	}
	if !strings.Contains(err.Error(), "not started") {
		t.Errorf("error should mention 'not started': %v", err)
	}
}

func TestBridgeTokenRotateRunner_Execute_Started(t *testing.T) {
	client := &mockBridgeSchedulerClient{}
	svc := screenshot.NewBridgeService(client, 5, 5*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	r := NewBridgeTokenRotateRunner(svc)
	result, err := r.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "运行中") {
		t.Errorf("result should mention 运行中: %s", result)
	}
}

// ===== AlertSilenceRunner Execute tests =====

func TestAlertSilenceRunner_Execute_NilManager(t *testing.T) {
	r := NewAlertSilenceRunner(nil)
	_, err := r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{"alert_type": "tamper"}})
	if err == nil {
		t.Fatal("expected error for nil manager")
	}
}

func TestAlertSilenceRunner_Execute_WithAlertType(t *testing.T) {
	m := alerting.NewManager()
	m.SendInfo(alerting.AlertTypeTamper, "t1", "msg", nil, "s", "u1")

	r := NewAlertSilenceRunner(m)
	result, err := r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{
		"alert_type":       "tamper",
		"duration_minutes": 30,
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "告警静默设置完成") {
		t.Errorf("result should mention '告警静默设置完成': %s", result)
	}
	if !strings.Contains(result, "30") {
		t.Errorf("result should mention duration: %s", result)
	}
}

func TestAlertSilenceRunner_Execute_CleanupOldRecords(t *testing.T) {
	m := alerting.NewManager()
	r := NewAlertSilenceRunner(m)
	result, err := r.Execute(context.Background(), &model.TaskPayload{MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "告警记录清理完成") {
		t.Errorf("result should mention '告警记录清理完成': %s", result)
	}
}

// ===== CacheWarmupRunner Execute tests =====

func TestCacheWarmupRunner_Execute_NoURLs(t *testing.T) {
	r := NewCacheWarmupRunner()
	result, err := r.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "warmup_urls") {
		t.Errorf("result should mention warmup_urls: %s", result)
	}
}

func TestCacheWarmupRunner_Execute_WithInvalidURL(t *testing.T) {
	r := NewCacheWarmupRunner()
	result, err := r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{
		"warmup_urls": []string{"://invalid"},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "失败: 1") {
		t.Errorf("result should show 失败: 1: %s", result)
	}
}

func TestCacheWarmupRunner_Execute_WithUnreachableURL(t *testing.T) {
	r := NewCacheWarmupRunner()
	result, err := r.Execute(context.Background(), &model.TaskPayload{Extra: map[string]any{
		"warmup_urls": []string{"http://localhost:65535/unreachable"},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "失败: 1") {
		t.Errorf("result should show 失败: 1: %s", result)
	}
}

// ===== extractStrings edge cases =====

func TestExtractStrings_InterfaceSliceWithNoStrings(t *testing.T) {
	payload := &model.TaskPayload{Extra: map[string]any{"items": []any{1, 2, 3}}}
	got := extractStrings(payload, "items", []string{"default"})
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

// ===== sanitizeUTF8 tests =====

func TestSanitizePayload_NilMap(t *testing.T) {
	got := sanitizePayload(nil)
	if got != nil {
		t.Errorf("expected nil for nil payload, got %v", got)
	}
}

func TestSanitizePayload_StringValues(t *testing.T) {
	// GBK bytes for "测试" = 0xB2 0xE2 0xCA 0xD4
	gbkBytes := []byte{0xB2, 0xE2, 0xCA, 0xD4}
	payload := &model.TaskPayload{
		URLs: []string{"https://example.com"},
		Extra: map[string]any{
			"name":  string(gbkBytes), // GBK encoded
			"count": 42,
		},
	}
	got := sanitizePayload(payload)

	// "name" should be converted
	if got.Extra["name"] != "测试" {
		t.Errorf("expected name=测试, got %q", got.Extra["name"])
	}
	// non-string values pass through
	if got.Extra["count"] != 42 {
		t.Errorf("expected count=42, got %v", got.Extra["count"])
	}
	// slice passes through
	if got.URLs == nil {
		t.Error("expected urls to be preserved")
	}
}

func TestSanitizeUTF8_ValidUTF8(t *testing.T) {
	input := "ICP 备案查询完成 ✅ baidu.com [web]: 5 条"
	got := sanitizeUTF8(input)
	if got != input {
		t.Errorf("valid UTF-8 should pass through unchanged, got %q", got)
	}
}

func TestSanitizeUTF8_GBKEncoded(t *testing.T) {
	// "ICP备案查询" in GBK encoding:
	//   I=0x49 C=0x43 P=0x50 备=0xB1B8 案=0xB0B8 查=0xB2E9 询=0xD1AF
	gbkBytes := []byte{0x49, 0x43, 0x50, 0xB1, 0xB8, 0xB0, 0xB8, 0xB2, 0xE9, 0xD1, 0xAF}
	input := string(gbkBytes)
	got := sanitizeUTF8(input)
	expected := "ICP备案查询"
	if got != expected {
		t.Errorf("GBK input should be converted to UTF-8\ngot:      %q (% x)\nexpected: %q", got, []byte(got), expected)
	}
}

func TestSanitizeUTF8_InvalidBytes(t *testing.T) {
	// Completely invalid bytes that are neither valid UTF-8 nor valid GBK.
	input := string([]byte{0xFF, 0xFE, 0x80, 0x81})
	got := sanitizeUTF8(input)
	// Should contain replacement characters, not panic.
	if got == "" {
		t.Error("expected non-empty output for invalid bytes")
	}
}

func TestSanitizeUTF8_MixedASCII(t *testing.T) {
	// Pure ASCII should pass through unchanged.
	input := "hello world 123"
	got := sanitizeUTF8(input)
	if got != input {
		t.Errorf("pure ASCII should pass through, got %q", got)
	}
}

func TestSanitizeUTF8_EmptyString(t *testing.T) {
	got := sanitizeUTF8("")
	if got != "" {
		t.Errorf("empty string should return empty, got %q", got)
	}
}

// ===== extractImagePaths tests =====

func TestExtractImagePaths_EmptyResult(t *testing.T) {
	paths := extractImagePaths("")
	if paths != nil {
		t.Errorf("expected nil for empty result, got %v", paths)
	}
}

func TestExtractImagePaths_NoImages(t *testing.T) {
	result := "UQL 查询完成\n\n📋 查询: test\n📊 共返回 10 条资产"
	paths := extractImagePaths(result)
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d: %v", len(paths), paths)
	}
}

func TestExtractImagePaths_ArrowFormat(t *testing.T) {
	result := "批量截图完成：2/2 成功\n\n✅ http://a.com → screenshots/a.png\n✅ http://b.com → screenshots/b.jpg\n\n📁 截图目录: screenshots"
	paths := extractImagePaths(result)
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
	}
	if paths[0] != "screenshots/a.png" {
		t.Errorf("expected screenshots/a.png, got %s", paths[0])
	}
	if paths[1] != "screenshots/b.jpg" {
		t.Errorf("expected screenshots/b.jpg, got %s", paths[1])
	}
}

func TestExtractImagePaths_MixedSuccessAndFailure(t *testing.T) {
	result := "批量截图完成：1/2 成功\n\n✅ http://a.com → screenshots/a.png\n❌ http://b.com — timeout\n\n📁 截图目录: screenshots"
	paths := extractImagePaths(result)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if paths[0] != "screenshots/a.png" {
		t.Errorf("expected screenshots/a.png, got %s", paths[0])
	}
}

func TestExtractImagePaths_ArrowFormatWithSpaces(t *testing.T) {
	result := "✅ http://example.com →   screenshots/dir/shot.png  "
	paths := extractImagePaths(result)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if paths[0] != "screenshots/dir/shot.png" {
		t.Errorf("expected trimmed path, got %s", paths[0])
	}
}

func TestExtractImagePaths_SkipsDirectoryPaths(t *testing.T) {
	result := "搜索引擎截图完成\n\n✅ 引擎: fofa\n✅ 保存: screenshots/batch1/shot.png\n✅ 截图目录: screenshots/batch1"
	paths := extractImagePaths(result)
	// "截图目录:" lines should be skipped
	for _, p := range paths {
		if strings.Contains(p, "截图目录") {
			t.Errorf("should skip directory path, got %s", p)
		}
	}
}

func TestExtractImagePaths_VariousImageExtensions(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int
	}{
		{"png", "✅ http://a.com → /tmp/shot.png", 1},
		{"jpg", "✅ http://a.com → /tmp/shot.jpg", 1},
		{"jpeg", "✅ http://a.com → /tmp/shot.jpeg", 1},
		{"gif", "✅ http://a.com → /tmp/shot.gif", 1},
		{"webp", "✅ http://a.com → /tmp/shot.webp", 1},
		{"PNG uppercase", "✅ http://a.com → /tmp/shot.PNG", 1},
		{"txt not image", "✅ http://a.com → /tmp/result.txt", 0},
		{"html not image", "✅ http://a.com → /tmp/page.html", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := extractImagePaths(tt.input)
			if len(paths) != tt.expect {
				t.Errorf("expected %d paths, got %d: %v", tt.expect, len(paths), paths)
			}
		})
	}
}

func TestExtractImagePaths_MultipleArrows(t *testing.T) {
	// Only split on first "→" in case URL contains arrow-like chars
	result := "✅ http://a.com/path → screenshots/out.png"
	paths := extractImagePaths(result)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if paths[0] != "screenshots/out.png" {
		t.Errorf("expected screenshots/out.png, got %s", paths[0])
	}
}

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		path   string
		expect bool
	}{
		{"shot.png", true},
		{"shot.PNG", true},
		{"shot.jpg", true},
		{"shot.jpeg", true},
		{"shot.gif", true},
		{"shot.webp", true},
		{"/abs/path/shot.png", true},
		{"relative/path/shot.jpg", true},
		{"file.txt", false},
		{"file.html", false},
		{"file.pdf", false},
		{"", false},
		{"png", false},          // no dot prefix
		{".png", true},          // just extension
		{"file.PNG.bak", false}, // wrong suffix
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isImageFile(tt.path)
			if got != tt.expect {
				t.Errorf("isImageFile(%q) = %v, want %v", tt.path, got, tt.expect)
			}
		})
	}
}

// ===== distributedIDCounter monotonicity =====

func TestDistributedTaskIDMonotonic(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateDistributedTaskID()
		if ids[id] {
			t.Errorf("duplicate ID: %s", id)
		}
		ids[id] = true
	}
}
