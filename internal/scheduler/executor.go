package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/distributed"
	"github.com/unimap/project/internal/exporter"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/service"
)

// --- QueryRunner (ST-01) ---

// QueryRunner executes scheduled UQL queries via QueryAppService.
type QueryRunner struct {
	querySvc *service.QueryAppService
}

// NewQueryRunner creates a QueryRunner.
func NewQueryRunner(b *service.QueryAppService) *QueryRunner {
	return &QueryRunner{querySvc: b}
}

func (r *QueryRunner) Type() TaskType { return TaskQuery }

func (r *QueryRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.querySvc == nil {
		return "", fmt.Errorf("query service not available")
	}

	if payload == nil || payload.Query == "" {
		return "", fmt.Errorf("missing 'query' in payload")
	}

	engines := payload.Engines
	if len(engines) == 0 {
		engines = extractStrings(payload, "engine", []string{})
	}
	pageSize := payload.PageSize
	if pageSize == 0 {
		pageSize = 100
	}

	resp, err := r.querySvc.ExecuteQuery(ctx, payload.Query, engines, pageSize)
	if err != nil {
		return "", fmt.Errorf("query execution failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "UQL 查询完成\n\n")
	fmt.Fprintf(&b, "📋 查询: %s\n", payload.Query)
	fmt.Fprintf(&b, "📋 引擎: %s\n", strings.Join(engines, ","))
	fmt.Fprintf(&b, "📋 页大小: %d\n", pageSize)
	fmt.Fprintf(&b, "📊 共返回 %d 条资产\n\n", resp.TotalCount)
	for eng, count := range resp.EngineStats {
		fmt.Fprintf(&b, "✅ %s: %d 条\n", eng, count)
	}
	for _, e := range resp.Errors {
		fmt.Fprintf(&b, "❌ %s\n", e)
	}
	return sanitizeUTF8(b.String()), nil
}

// --- SearchScreenshotRunner (ST-02) ---

// SearchScreenshotRunner executes scheduled search engine screenshots.
type SearchScreenshotRunner struct {
	screenshotSvc *service.ScreenshotAppService
	mgr           *screenshot.Manager
}

// NewSearchScreenshotRunner creates a SearchScreenshotRunner.
func NewSearchScreenshotRunner(svc *service.ScreenshotAppService, mgr *screenshot.Manager) *SearchScreenshotRunner {
	return &SearchScreenshotRunner{screenshotSvc: svc, mgr: mgr}
}

func (r *SearchScreenshotRunner) Type() TaskType { return TaskSearchScreenshot }

func (r *SearchScreenshotRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.screenshotSvc == nil {
		return "", fmt.Errorf("screenshot service not available")
	}

	engine := extractString(payload, "engine", "")
	query := payload.Query
	queryID := extractString(payload, "query_id", "")

	if engine == "" || query == "" {
		return "", fmt.Errorf("missing 'engine' or 'query' in payload")
	}

	path, eng, q, id, err := r.screenshotSvc.CaptureSearchEngineResult(ctx, r.mgr, engine, query, queryID)
	if err != nil {
		return "", fmt.Errorf("screenshot capture failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "搜索引擎截图完成\n\n")
	fmt.Fprintf(&b, "✅ 引擎: %s\n", eng)
	fmt.Fprintf(&b, "✅ 查询: %s\n", q)
	fmt.Fprintf(&b, "✅ 保存: %s\n", path)
	if id != "" {
		fmt.Fprintf(&b, "✅ 查询ID: %s\n", id)
	}
	return sanitizeUTF8(b.String()), nil
}

// --- BatchScreenshotRunner (ST-03) ---

// BatchScreenshotRunner executes scheduled batch URL screenshots.
type BatchScreenshotRunner struct {
	screenshotSvc *service.ScreenshotAppService
	mgr           *screenshot.Manager
}

// NewBatchScreenshotRunner creates a BatchScreenshotRunner.
func NewBatchScreenshotRunner(svc *service.ScreenshotAppService, mgr *screenshot.Manager) *BatchScreenshotRunner {
	return &BatchScreenshotRunner{screenshotSvc: svc, mgr: mgr}
}

func (r *BatchScreenshotRunner) Type() TaskType { return TaskBatchScreenshot }

func (r *BatchScreenshotRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.screenshotSvc == nil {
		return "", fmt.Errorf("screenshot service not available")
	}

	urls := extractStrings(payload, "urls", []string{})
	if len(urls) == 0 {
		return "", fmt.Errorf("missing 'urls' in payload")
	}

	batchID := extractString(payload, "batch_id", "")
	concurrency := extractInt(payload, "concurrency", 5)

	req := service.BatchURLsRequest{
		URLs:        urls,
		BatchID:     batchID,
		Concurrency: concurrency,
	}

	resp, err := r.screenshotSvc.CaptureBatchURLs(ctx, r.mgr, req)
	if err != nil {
		return "", fmt.Errorf("batch screenshot failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "批量截图完成：%d/%d 成功\n\n", resp.Success, resp.Total)
	for _, r := range resp.Results {
		if r.Success {
			fmt.Fprintf(&b, "✅ %s → %s\n", r.URL, r.FilePath)
		} else {
			fmt.Fprintf(&b, "❌ %s — %s\n", r.URL, r.Error)
		}
	}
	fmt.Fprintf(&b, "\n📁 截图目录: %s", resp.ScreenshotDir)
	return sanitizeUTF8(b.String()), nil
}

// --- TamperCheckRunner (ST-04) ---

// TamperCheckRunner executes scheduled tamper checks.
type TamperCheckRunner struct {
	tamperSvc        *service.TamperAppService
	allocatorFactory service.TamperAllocatorFactory
}

// NewTamperCheckRunner creates a TamperCheckRunner.
func NewTamperCheckRunner(svc *service.TamperAppService, af service.TamperAllocatorFactory) *TamperCheckRunner {
	return &TamperCheckRunner{tamperSvc: svc, allocatorFactory: af}
}

func (r *TamperCheckRunner) Type() TaskType { return TaskTamperCheck }

func (r *TamperCheckRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.tamperSvc == nil {
		return "", fmt.Errorf("tamper service not available")
	}

	urls := extractStrings(payload, "urls", []string{})
	if len(urls) == 0 {
		return "", fmt.Errorf("missing 'urls' in payload")
	}

	concurrency := extractInt(payload, "concurrency", 5)
	mode := extractString(payload, "detection_mode", "relaxed")

	req := service.TamperCheckRequest{
		URLs:        urls,
		Concurrency: concurrency,
		Mode:        mode,
	}

	resp, err := r.tamperSvc.Check(ctx, req, r.allocatorFactory)
	if err != nil {
		return "", fmt.Errorf("tamper check failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "篡改检测完成（模式: %s）：共 %d 个 URL\n\n", mode, len(resp.Results))
	for _, r := range resp.Results {
		switch r.Status {
		case "tampered":
			fmt.Fprintf(&b, "⚠️ 已篡改 %s", r.URL)
			if len(r.TamperedSegments) > 0 {
				fmt.Fprintf(&b, " — 变更区域: %s", strings.Join(r.TamperedSegments, ", "))
			}
			b.WriteString("\n")
		case "no_baseline":
			fmt.Fprintf(&b, "🆕 首次检测 %s — 已建立基线\n", r.URL)
		case "unreachable":
			fmt.Fprintf(&b, "❌ 不可达 %s", r.URL)
			if r.ErrorMessage != "" {
				fmt.Fprintf(&b, " — %s", r.ErrorMessage)
			}
			b.WriteString("\n")
		case "normal":
			fmt.Fprintf(&b, "✅ 正常 %s\n", r.URL)
		default:
			fmt.Fprintf(&b, "❓ %s %s\n", r.Status, r.URL)
		}
	}
	return sanitizeUTF8(b.String()), nil
}

// --- URLReachabilityRunner (ST-05) ---

// URLReachabilityRunner executes scheduled URL reachability checks.
type URLReachabilityRunner struct {
	monitorSvc *service.MonitorAppService
}

// NewURLReachabilityRunner creates a URLReachabilityRunner.
func NewURLReachabilityRunner(svc *service.MonitorAppService) *URLReachabilityRunner {
	return &URLReachabilityRunner{monitorSvc: svc}
}

func (r *URLReachabilityRunner) Type() TaskType { return TaskURLReachability }

func (r *URLReachabilityRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.monitorSvc == nil {
		return "", fmt.Errorf("monitor service not available")
	}

	urls := extractStrings(payload, "urls", []string{})
	if len(urls) == 0 {
		return "", fmt.Errorf("missing 'urls' in payload")
	}

	concurrency := extractInt(payload, "concurrency", 5)

	resp, err := r.monitorSvc.CheckURLReachability(ctx, urls, concurrency)
	if err != nil {
		return "", fmt.Errorf("reachability check failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "URL 可达性检测完成：%d 个 URL\n\n", resp.Summary.Total)
	for _, r := range resp.Results {
		if r.Reachable {
			detail := r.Input
			if r.HTTPStatus > 0 {
				detail = fmt.Sprintf("%s (HTTP %d)", r.Input, r.HTTPStatus)
			}
			fmt.Fprintf(&b, "✅ 可达 %s\n", detail)
		} else {
			detail := r.Input
			if r.Reason != "" {
				detail += " — " + r.Reason
			}
			fmt.Fprintf(&b, "❌ 不可达 %s\n", detail)
		}
	}
	fmt.Fprintf(&b, "\n📊 可达: %d，不可达: %d", resp.Summary.Reachable, resp.Summary.Unreachable)
	return sanitizeUTF8(b.String()), nil
}

// --- CookieVerifyRunner (ST-06) ---

// CookieVerifyRunner executes scheduled cookie verification.
type CookieVerifyRunner struct {
	screenshotSvc *service.ScreenshotAppService
	mgr           *screenshot.Manager
}

// NewCookieVerifyRunner creates a CookieVerifyRunner.
func NewCookieVerifyRunner(svc *service.ScreenshotAppService, mgr *screenshot.Manager) *CookieVerifyRunner {
	return &CookieVerifyRunner{screenshotSvc: svc, mgr: mgr}
}

func (r *CookieVerifyRunner) Type() TaskType { return TaskCookieVerify }

func (r *CookieVerifyRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.mgr == nil {
		return "", fmt.Errorf("screenshot manager not available")
	}

	engines := extractStrings(payload, "engines", []string{})
	if len(engines) == 0 {
		// Default: check all supported engines
		engines = []string{"fofa", "hunter", "quake", "zoomeye"}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Cookie 验证完成：%d 个引擎\n\n", len(engines))
	for _, engine := range engines {
		cookies := r.mgr.GetCookies(engine)
		if len(cookies) > 0 {
			fmt.Fprintf(&b, "✅ %s: %d 个 Cookie 已配置\n", engine, len(cookies))
		} else {
			fmt.Fprintf(&b, "⚠️ %s: 未配置 Cookie\n", engine)
		}
	}
	return sanitizeUTF8(b.String()), nil
}

// --- LoginStatusCheckRunner (ST-07) ---

// LoginStatusCheckRunner executes scheduled login status checks.
type LoginStatusCheckRunner struct {
	mgr *screenshot.Manager
}

// NewLoginStatusCheckRunner creates a LoginStatusCheckRunner.
func NewLoginStatusCheckRunner(mgr *screenshot.Manager) *LoginStatusCheckRunner {
	return &LoginStatusCheckRunner{mgr: mgr}
}

func (r *LoginStatusCheckRunner) Type() TaskType { return TaskLoginStatusCheck }

func (r *LoginStatusCheckRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.mgr == nil {
		return "", fmt.Errorf("screenshot manager not available")
	}

	engines := extractStrings(payload, "engines", []string{})
	if len(engines) == 0 {
		engines = []string{"fofa", "hunter", "quake", "zoomeye"}
	}
	testQuery := extractString(payload, "test_query", "test")

	var b strings.Builder
	fmt.Fprintf(&b, "登录状态检查完成：%d 个引擎\n\n", len(engines))
	failedCount := 0
	for _, engine := range engines {
		status, err := r.mgr.CheckEngineLoginStatus(ctx, engine, testQuery)
		if err != nil {
			fmt.Fprintf(&b, "❌ %s: 检查失败 — %v\n", engine, err)
			failedCount++
			continue
		}
		if status.LoggedIn {
			fmt.Fprintf(&b, "✅ %s: 已登录", engine)
		} else {
			fmt.Fprintf(&b, "❌ %s: 未登录", engine)
			failedCount++
		}
		if status.Reason != "" {
			fmt.Fprintf(&b, " (%s)", status.Reason)
		}
		b.WriteString("\n")
	}
	result := sanitizeUTF8(b.String())
	if failedCount > 0 {
		return result, fmt.Errorf("%d engine(s) not logged in or errored", failedCount)
	}
	return result, nil
}

// --- DistributedSubmitRunner (ST-08) ---

// DistributedSubmitRunner executes scheduled distributed task submissions.
type DistributedSubmitRunner struct {
	taskQueue *distributed.TaskQueue
}

// NewDistributedSubmitRunner creates a DistributedSubmitRunner.
func NewDistributedSubmitRunner(q *distributed.TaskQueue) *DistributedSubmitRunner {
	return &DistributedSubmitRunner{taskQueue: q}
}

func (r *DistributedSubmitRunner) Type() TaskType { return TaskDistributedSubmit }

func (r *DistributedSubmitRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.taskQueue == nil {
		return "", fmt.Errorf("task queue not available")
	}

	taskType := extractString(payload, "task_type", "")
	if taskType == "" {
		return "", fmt.Errorf("missing 'task_type' in payload")
	}

	taskPayload := make(map[string]any)
	if payload.Extra != nil {
		if p, ok := payload.Extra["task_payload"]; ok {
			if pm, ok := p.(map[string]any); ok {
				taskPayload = pm
			}
		}
	}

	priority := extractInt(payload, "priority", 0)
	timeoutSec := extractInt(payload, "timeout_seconds", 300)
	maxReassign := extractInt(payload, "max_reassign", 3)

	// Build the envelope
	envelope := distributed.TaskEnvelope{
		TaskID:         generateDistributedTaskID(),
		TaskType:       taskType,
		Payload:        taskPayload,
		Priority:       priority,
		TimeoutSeconds: timeoutSec,
		MaxReassign:    maxReassign,
	}

	if _, err := r.taskQueue.Enqueue(envelope); err != nil {
		return "", fmt.Errorf("enqueue failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "分布式任务已提交\n\n")
	fmt.Fprintf(&b, "✅ 任务ID: %s\n", envelope.TaskID)
	fmt.Fprintf(&b, "✅ 任务类型: %s\n", taskType)
	fmt.Fprintf(&b, "✅ 优先级: %d\n", priority)
	fmt.Fprintf(&b, "✅ 超时: %ds\n", timeoutSec)
	fmt.Fprintf(&b, "✅ 最大重分配: %d\n", maxReassign)
	return sanitizeUTF8(b.String()), nil
}

// distributedIDCounter is a monotonic counter for unique distributed task IDs.
var distributedIDCounter atomic.Int64

// generateDistributedTaskID creates a unique ID for distributed task envelopes.
func generateDistributedTaskID() string {
	return fmt.Sprintf("dist_%d", distributedIDCounter.Add(1))
}

// --- ExportRunner (ST-09) ---

// ExportRunner executes scheduled data exports.
type ExportRunner struct {
	queryApp     *service.QueryAppService
	orchestrator *adapter.EngineOrchestrator
	outputDir    string
}

// NewExportRunner creates an ExportRunner.
func NewExportRunner(queryApp *service.QueryAppService, orchestrator *adapter.EngineOrchestrator, outputDir string) *ExportRunner {
	return &ExportRunner{queryApp: queryApp, orchestrator: orchestrator, outputDir: outputDir}
}

func (r *ExportRunner) Type() TaskType { return TaskExport }

func (r *ExportRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.queryApp == nil || r.orchestrator == nil {
		return "", fmt.Errorf("query service or orchestrator not available")
	}

	query := extractString(payload, "query", "")
	if query == "" {
		return "", fmt.Errorf("missing 'query' in payload")
	}

	engines := extractStrings(payload, "engines", []string{})
	pageSize := extractInt(payload, "page_size", 100)
	format := extractString(payload, "format", "json")
	outputFile := extractString(payload, "output_file", "")

	// Execute the query
	resp, err := r.queryApp.ExecuteQuery(ctx, query, engines, pageSize)
	if err != nil {
		return "", fmt.Errorf("query execution failed: %w", err)
	}

	if resp.TotalCount == 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "数据导出完成\n\n")
		fmt.Fprintf(&b, "⚠️ 查询: %s\n", query)
		fmt.Fprintf(&b, "⚠️ 引擎: %s\n", strings.Join(engines, ","))
		fmt.Fprintf(&b, "⚠️ 结果: 无数据可导出\n")
		return sanitizeUTF8(b.String()), nil
	}

	// Determine output path
	if outputFile == "" {
		outputFile = fmt.Sprintf("export_%s_%s.%s", strings.ReplaceAll(query[:min(len(query), 20)], " ", "_"), time.Now().Format("20060102_150405"), format)
	}
	outPath := filepath.Join(r.outputDir, outputFile)

	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	// Export
	var exp exporter.Exporter
	switch format {
	case "excel", "xlsx":
		exp = exporter.NewExcelExporter()
	default:
		exp = exporter.NewJSONExporter()
	}
	if err := exp.Export(resp.Assets, outPath); err != nil {
		return "", fmt.Errorf("export failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "数据导出完成\n\n")
	fmt.Fprintf(&b, "✅ 查询: %s\n", query)
	fmt.Fprintf(&b, "✅ 引擎: %s\n", strings.Join(engines, ","))
	fmt.Fprintf(&b, "✅ 格式: %s\n", format)
	fmt.Fprintf(&b, "✅ 资产数: %d\n", len(resp.Assets))
	fmt.Fprintf(&b, "✅ 保存: %s\n", outPath)
	return sanitizeUTF8(b.String()), nil
}

// --- PortScanRunner (ST-10) ---

// PortScanRunner executes scheduled port scans.
type PortScanRunner struct {
	monitorSvc *service.MonitorAppService
}

// NewPortScanRunner creates a PortScanRunner.
func NewPortScanRunner(svc *service.MonitorAppService) *PortScanRunner {
	return &PortScanRunner{monitorSvc: svc}
}

func (r *PortScanRunner) Type() TaskType { return TaskPortScan }

func (r *PortScanRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.monitorSvc == nil {
		return "", fmt.Errorf("monitor service not available")
	}

	urls := extractStrings(payload, "urls", []string{})
	if len(urls) == 0 {
		return "", fmt.Errorf("missing 'urls' in payload")
	}

	ports := extractStrings(payload, "ports", []string{})
	concurrency := extractInt(payload, "concurrency", 5)

	portNums := make([]int, 0, len(ports))
	for _, p := range ports {
		if n := extractIntFromMap(map[string]any{"v": p}, "v", 0); n > 0 {
			portNums = append(portNums, n)
		}
	}
	if len(portNums) == 0 {
		portNums = []int{80, 443} // default ports
	}

	resp, err := r.monitorSvc.ScanURLPorts(ctx, urls, portNums, concurrency)
	if err != nil {
		return "", fmt.Errorf("port scan failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "端口扫描完成：%d 个 URL，端口 %v\n\n", resp.Summary.Total, portNums)
	for _, r := range resp.Results {
		switch r.Status {
		case "scanned":
			if len(r.OpenPorts) > 0 {
				var portDetails []string
				for ip, ports := range r.OpenPorts {
					portDetails = append(portDetails, fmt.Sprintf("%s: %v", ip, ports))
				}
				fmt.Fprintf(&b, "✅ %s — 开放端口 %s\n", r.Input, strings.Join(portDetails, "; "))
			} else {
				fmt.Fprintf(&b, "✅ %s — 无开放端口\n", r.Input)
			}
		case "resolve_failed":
			fmt.Fprintf(&b, "❌ %s — DNS 解析失败", r.Input)
			if r.Reason != "" {
				fmt.Fprintf(&b, " (%s)", r.Reason)
			}
			b.WriteString("\n")
		case "cdn_excluded":
			fmt.Fprintf(&b, "⚠️ %s — CDN 已排除\n", r.Input)
		default:
			fmt.Fprintf(&b, "❓ %s — %s", r.Input, r.Status)
			if r.Reason != "" {
				fmt.Fprintf(&b, " (%s)", r.Reason)
			}
			b.WriteString("\n")
		}
	}
	return sanitizeUTF8(b.String()), nil
}

// --- ScreenshotCleanupRunner (ST-11) ---

// ScreenshotCleanupRunner executes scheduled screenshot cleanup.
type ScreenshotCleanupRunner struct {
	screenshotSvc *service.ScreenshotAppService
	maxAgeDays    int
}

// NewScreenshotCleanupRunner creates a ScreenshotCleanupRunner.
func NewScreenshotCleanupRunner(svc *service.ScreenshotAppService, maxAgeDays int) *ScreenshotCleanupRunner {
	if maxAgeDays <= 0 {
		maxAgeDays = 30
	}
	return &ScreenshotCleanupRunner{screenshotSvc: svc, maxAgeDays: maxAgeDays}
}

func (r *ScreenshotCleanupRunner) Type() TaskType { return TaskScreenshotCleanup }

func (r *ScreenshotCleanupRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.screenshotSvc == nil {
		return "", fmt.Errorf("screenshot service not available")
	}

	maxAgeDays := extractInt(payload, "max_age_days", r.maxAgeDays)
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	batches, err := r.screenshotSvc.ListBatches()
	if err != nil {
		return "", fmt.Errorf("list batches failed: %w", err)
	}

	deletedCount := 0
	skippedCount := 0
	for _, batch := range batches {
		batchTime := time.Unix(batch.UpdatedAt, 0)
		if batchTime.Before(cutoff) {
			if delErr := r.screenshotSvc.DeleteBatch(batch.Name); delErr != nil {
				continue
			}
			deletedCount++
		} else {
			skippedCount++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "截图清理完成（保留 %d 天）\n\n", maxAgeDays)
	fmt.Fprintf(&b, "🗑️ 已删除: %d 个批次\n", deletedCount)
	fmt.Fprintf(&b, "📁 保留: %d 个批次\n", skippedCount)
	return sanitizeUTF8(b.String()), nil
}

// --- TamperCleanupRunner (ST-12) ---

// TamperCleanupRunner executes scheduled tamper record cleanup.
type TamperCleanupRunner struct {
	tamperSvc  *service.TamperAppService
	maxAgeDays int
}

// NewTamperCleanupRunner creates a TamperCleanupRunner.
func NewTamperCleanupRunner(svc *service.TamperAppService, maxAgeDays int) *TamperCleanupRunner {
	if maxAgeDays <= 0 {
		maxAgeDays = 90
	}
	return &TamperCleanupRunner{tamperSvc: svc, maxAgeDays: maxAgeDays}
}

func (r *TamperCleanupRunner) Type() TaskType { return TaskTamperCleanup }

func (r *TamperCleanupRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.tamperSvc == nil {
		return "", fmt.Errorf("tamper service not available")
	}

	records, err := r.tamperSvc.ListAllCheckRecords()
	if err != nil {
		return "", fmt.Errorf("list check records failed: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -r.maxAgeDays).Unix()
	deletedCount := 0
	skippedCount := 0

	for url, urlRecords := range records {
		onlyExpired := true
		hasExpired := false
		zeroTimestampFound := false

		for _, record := range urlRecords {
			if record == nil {
				continue
			}
			if record.Timestamp == 0 {
				zeroTimestampFound = true
				continue
			}
			if record.Timestamp >= cutoff {
				onlyExpired = false
				break
			}
			hasExpired = true
		}

		if !onlyExpired || !hasExpired {
			skippedCount += len(urlRecords)
			continue
		}

		if zeroTimestampFound {
			logger.Warnf("tamper check record for %q has zero timestamp(s), skipping deletion to prevent data loss", url)
			skippedCount += len(urlRecords)
			continue
		}

		if delErr := r.tamperSvc.DeleteCheckRecords(url); delErr != nil {
			logger.Warnf("failed to delete expired tamper records for %q: %v", url, delErr)
		} else {
			deletedCount += len(urlRecords)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "篡改记录清理完成（保留 %d 天）\n\n", r.maxAgeDays)
	fmt.Fprintf(&b, "🗑️ 已删除: %d 条过期记录\n", deletedCount)
	fmt.Fprintf(&b, "📁 保留: %d 条有效记录\n", skippedCount)
	return sanitizeUTF8(b.String()), nil
}

// --- QuotaMonitorRunner (ST-13) ---

// QuotaMonitorRunner executes scheduled quota monitoring.
type QuotaMonitorRunner struct {
	orchestrator *adapter.EngineOrchestrator
	lowThreshold int
}

// NewQuotaMonitorRunner creates a QuotaMonitorRunner.
func NewQuotaMonitorRunner(orchestrator *adapter.EngineOrchestrator, lowThreshold int) *QuotaMonitorRunner {
	if lowThreshold <= 0 {
		lowThreshold = 10
	}
	return &QuotaMonitorRunner{orchestrator: orchestrator, lowThreshold: lowThreshold}
}

func (r *QuotaMonitorRunner) Type() TaskType { return TaskQuotaMonitor }

func (r *QuotaMonitorRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.orchestrator == nil {
		return "", fmt.Errorf("orchestrator not available")
	}

	engines := r.orchestrator.ListAdapters()
	if len(engines) == 0 {
		return "no engine adapters registered", nil
	}

	lowThreshold := extractInt(payload, "low_threshold", r.lowThreshold)
	var b strings.Builder
	fmt.Fprintf(&b, "引擎配额监控完成：%d 个引擎\n\n", len(engines))
	lowQuotaEngines := 0

	for _, engine := range engines {
		adapter, ok := r.orchestrator.GetAdapter(engine)
		if !ok {
			continue
		}
		quota, err := adapter.GetQuota()
		if err != nil {
			fmt.Fprintf(&b, "❌ %s: 查询失败 — %v\n", engine, err)
			continue
		}
		if quota != nil && quota.Remaining < lowThreshold {
			fmt.Fprintf(&b, "⚠️ %s: 配额不足 (剩余 %d/%d)\n", engine, quota.Remaining, quota.Total)
			lowQuotaEngines++
		} else if quota != nil {
			fmt.Fprintf(&b, "✅ %s: 配额充足 (剩余 %d/%d)\n", engine, quota.Remaining, quota.Total)
		} else {
			fmt.Fprintf(&b, "✅ %s: 配额信息不可用\n", engine)
		}
	}

	result := sanitizeUTF8(b.String())
	if lowQuotaEngines > 0 {
		return result, fmt.Errorf("%d engine(s) with low quota (below %d)", lowQuotaEngines, lowThreshold)
	}
	return result, nil
}

