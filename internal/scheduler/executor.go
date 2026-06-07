package scheduler

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/alerting"
	"github.com/unimap/project/internal/distributed"
	"github.com/unimap/project/internal/exporter"
	icpdb "github.com/unimap/project/internal/icp/database"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/service"
	"github.com/unimap/project/internal/utils/urlguard"
)

// sanitizeUTF8 ensures s is valid UTF-8, converting from GBK if necessary.
// This prevents garbled text (mojibake) in notification channels that assume
// valid UTF-8 (Feishu, DingTalk, WeCom).
//
// Steps:
//  1. If already valid UTF-8, return as-is.
//  2. Try GBK→UTF-8 conversion (common for Chinese API responses / Windows environments).
//  3. Strip any remaining invalid UTF-8 bytes as a final fallback.
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	// Try GBK→UTF-8 conversion — many Chinese APIs and Windows tools produce GBK output.
	decoded, _, err := transform.String(simplifiedchinese.GBK.NewDecoder(), s)
	if err == nil && utf8.ValidString(decoded) {
		return decoded
	}
	// Fallback: strip invalid UTF-8 bytes.
	return strings.ToValidUTF8(s, "�")
}

// extractStrings pulls a string slice from payload[key], falling back to def.
func extractStrings(payload map[string]interface{}, key string, def []string) []string {
	v, ok := payload[key]
	if !ok {
		return def
	}
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if val == "" {
			return def
		}
		return []string{val}
	default:
		return def
	}
}

func extractInt(payload map[string]interface{}, key string, def int) int {
	v, ok := payload[key]
	if !ok {
		return def
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	default:
		return def
	}
}

func extractString(payload map[string]interface{}, key string, def string) string {
	v, ok := payload[key]
	if !ok {
		return def
	}
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

func extractBool(payload map[string]interface{}, key string, def bool) bool {
	v, ok := payload[key]
	if !ok {
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

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

func (r *QueryRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	if r.querySvc == nil {
		return "", fmt.Errorf("query service not available")
	}

	query := extractString(payload, "query", "")
	if query == "" {
		return "", fmt.Errorf("missing 'query' in payload")
	}

	engines := extractStrings(payload, "engines", []string{})
	if len(engines) == 0 {
		engines = extractStrings(payload, "engine", []string{})
	}
	pageSize := extractInt(payload, "page_size", 100)

	resp, err := r.querySvc.ExecuteQuery(ctx, query, engines, pageSize)
	if err != nil {
		return "", fmt.Errorf("query execution failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "UQL 查询完成\n\n")
	fmt.Fprintf(&b, "📋 查询: %s\n", query)
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

func (r *SearchScreenshotRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	if r.screenshotSvc == nil {
		return "", fmt.Errorf("screenshot service not available")
	}

	engine := extractString(payload, "engine", "")
	query := extractString(payload, "query", "")
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

func (r *BatchScreenshotRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
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

func (r *TamperCheckRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
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

func (r *URLReachabilityRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
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

func (r *CookieVerifyRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
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

func (r *LoginStatusCheckRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
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

func (r *DistributedSubmitRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	if r.taskQueue == nil {
		return "", fmt.Errorf("task queue not available")
	}

	taskType := extractString(payload, "task_type", "")
	if taskType == "" {
		return "", fmt.Errorf("missing 'task_type' in payload")
	}

	taskPayload := make(map[string]interface{})
	if p, ok := payload["task_payload"]; ok {
		if pm, ok := p.(map[string]interface{}); ok {
			taskPayload = pm
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

func (r *ExportRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
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

func (r *PortScanRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
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
		if n := extractInt(map[string]interface{}{"v": p}, "v", 0); n > 0 {
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

func (r *ScreenshotCleanupRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
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

func (r *TamperCleanupRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
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

func (r *QuotaMonitorRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
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

// --- AlertSummaryRunner (ST-14) ---

// AlertSummaryRunner executes scheduled alert summary generation.
type AlertSummaryRunner struct {
	alertManager *alerting.Manager
}

// NewAlertSummaryRunner creates an AlertSummaryRunner.
func NewAlertSummaryRunner(alertManager *alerting.Manager) *AlertSummaryRunner {
	return &AlertSummaryRunner{alertManager: alertManager}
}

func (r *AlertSummaryRunner) Type() TaskType { return TaskAlertSummary }

func (r *AlertSummaryRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	if r.alertManager == nil {
		return "", fmt.Errorf("alert manager not available")
	}

	maxAgeDays := extractInt(payload, "max_age_days", 7)

	records := r.alertManager.GetAlertRecords()
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	typeCounts := make(map[string]int)
	levelCounts := make(map[string]int)
	totalCount := 0

	for _, rec := range records {
		if rec.Alert.Timestamp.Before(cutoff) {
			continue
		}
		totalCount++
		typeCounts[string(rec.Alert.Type)]++
		levelCounts[string(rec.Alert.Level)]++
	}

	var b strings.Builder
	fmt.Fprintf(&b, "告警汇总（最近 %d 天）\n\n", maxAgeDays)
	fmt.Fprintf(&b, "📊 总计: %d 条告警\n", totalCount)
	if len(typeCounts) > 0 {
		b.WriteString("\n按类型:\n")
		for t, c := range typeCounts {
			fmt.Fprintf(&b, "  • %s: %d 条\n", t, c)
		}
	}
	if len(levelCounts) > 0 {
		b.WriteString("\n按级别:\n")
		for l, c := range levelCounts {
			emoji := "ℹ️"
			switch l {
			case "warning":
				emoji = "⚠️"
			case "critical":
				emoji = "🔴"
			}
			fmt.Fprintf(&b, "  %s %s: %d 条\n", emoji, l, c)
		}
	}
	return sanitizeUTF8(b.String()), nil
}

// --- BaselineRefreshRunner (ST-15) ---

// BaselineRefreshRunner executes scheduled baseline refresh.
type BaselineRefreshRunner struct {
	tamperSvc *service.TamperAppService
}

// NewBaselineRefreshRunner creates a BaselineRefreshRunner.
func NewBaselineRefreshRunner(svc *service.TamperAppService) *BaselineRefreshRunner {
	return &BaselineRefreshRunner{tamperSvc: svc}
}

func (r *BaselineRefreshRunner) Type() TaskType { return TaskBaselineRefresh }

func (r *BaselineRefreshRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	if r.tamperSvc == nil {
		return "", fmt.Errorf("tamper service not available")
	}

	urls := extractStrings(payload, "urls", []string{})
	if len(urls) == 0 {
		// Get current baselines and refresh them
		baselines, err := r.tamperSvc.ListBaselines()
		if err != nil {
			return "", fmt.Errorf("list baselines failed: %w", err)
		}
		if len(baselines) == 0 {
			return "no baselines to refresh", nil
		}
		urls = baselines
	}

	refreshed := 0
	failed := 0
	var failedURLs []string

	for _, url := range urls {
		req := service.TamperBaselineRequest{
			URLs: []string{url},
		}
		_, err := r.tamperSvc.SetBaseline(ctx, req, nil)
		if err != nil {
			failed++
			failedURLs = append(failedURLs, url)
			continue
		}
		refreshed++
	}

	var b strings.Builder
	fmt.Fprintf(&b, "基线刷新完成：%d 个 URL\n\n", len(urls))
	fmt.Fprintf(&b, "✅ 成功: %d 个\n", refreshed)
	if failed > 0 {
		fmt.Fprintf(&b, "❌ 失败: %d 个\n", failed)
		for _, u := range failedURLs {
			fmt.Fprintf(&b, "  • %s\n", u)
		}
	}
	return sanitizeUTF8(b.String()), nil
}

// --- URLImportRunner (ST-16) ---

// URLImportRunner executes scheduled URL import from files.
type URLImportRunner struct {
	importDir string
}

// NewURLImportRunner creates a URLImportRunner.
func NewURLImportRunner(importDir string) *URLImportRunner {
	return &URLImportRunner{importDir: importDir}
}

func (r *URLImportRunner) Type() TaskType { return TaskURLImport }

func (r *URLImportRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	if r.importDir == "" {
		return "", fmt.Errorf("import directory not configured")
	}

	filePattern := extractString(payload, "file_pattern", "*.txt")
	maxLines := extractInt(payload, "max_lines", 10000)

	// Find matching files
	matches, err := filepath.Glob(filepath.Join(r.importDir, filePattern))
	if err != nil {
		return "", fmt.Errorf("glob failed: %w", err)
	}
	if len(matches) == 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "URL 导入完成\n\n")
		fmt.Fprintf(&b, "⚠️ 未找到匹配文件: %s\n", filePattern)
		fmt.Fprintf(&b, "📁 搜索目录: %s\n", r.importDir)
		return sanitizeUTF8(b.String()), nil
	}

	importedURLs := make([]string, 0)
	fileDetails := make([]struct {
		name string
		count int
		err  error
	}, 0, len(matches))

	for _, filePath := range matches {
		urls, err := readURLsFromFile(filePath, maxLines-len(importedURLs))
		if err != nil {
			fileDetails = append(fileDetails, struct {
				name string
				count int
				err  error
			}{filepath.Base(filePath), 0, err})
			continue
		}
		importedURLs = append(importedURLs, urls...)
		fileDetails = append(fileDetails, struct {
			name string
			count int
			err  error
		}{filepath.Base(filePath), len(urls), nil})
	}

	var b strings.Builder
	fmt.Fprintf(&b, "URL 导入完成：%d 个文件\n\n", len(matches))
	for _, fd := range fileDetails {
		if fd.err != nil {
			fmt.Fprintf(&b, "❌ %s: 读取失败 — %v\n", fd.name, fd.err)
		} else {
			fmt.Fprintf(&b, "✅ %s: %d 条 URL\n", fd.name, fd.count)
		}
	}
	fmt.Fprintf(&b, "\n📊 共导入: %d 条 URL\n", len(importedURLs))
	return sanitizeUTF8(b.String()), nil
}

// readURLsFromFile reads URLs from a text file, one per line.
func readURLsFromFile(filePath string, maxLines int) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	urls := make([]string, 0)
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		if count >= maxLines {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
		count++
	}
	return urls, scanner.Err()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- PluginHealthRunner (ST-17) ---

// PluginHealthRunner executes scheduled plugin health checks.
type PluginHealthRunner struct {
	unifiedSvc *service.UnifiedService
}

// NewPluginHealthRunner creates a PluginHealthRunner.
func NewPluginHealthRunner(svc *service.UnifiedService) *PluginHealthRunner {
	return &PluginHealthRunner{unifiedSvc: svc}
}

func (r *PluginHealthRunner) Type() TaskType { return TaskPluginHealth }

func (r *PluginHealthRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	if r.unifiedSvc == nil {
		return "", fmt.Errorf("unified service not available")
	}

	health := r.unifiedSvc.HealthCheck()
	if len(health) == 0 {
		return "插件健康检查完成\n\n⚠️ 无已注册插件", nil
	}

	healthyCount := 0
	var b strings.Builder
	fmt.Fprintf(&b, "插件健康检查完成：%d 个插件\n\n", len(health))
	for name, status := range health {
		if status.Healthy {
			fmt.Fprintf(&b, "✅ %s: 健康\n", name)
			healthyCount++
		} else {
			msg := status.Message
			if msg == "" {
				msg = "未知错误"
			}
			fmt.Fprintf(&b, "❌ %s: %s\n", name, msg)
		}
	}

	result := sanitizeUTF8(b.String())
	if healthyCount < len(health) {
		return result, fmt.Errorf("%d/%d plugins unhealthy", len(health)-healthyCount, len(health))
	}
	return result, nil
}

// --- BridgeHealthCheckRunner (ST-18) ---
// Note: Task type constant is "bridge_token" for backward compatibility,
// but this runner performs health checks, not token rotation.

// BridgeHealthCheckRunner executes scheduled bridge health checks.
type BridgeHealthCheckRunner struct {
	bridgeSvc *screenshot.BridgeService
}

// NewBridgeHealthCheckRunner creates a BridgeHealthCheckRunner.
// Kept as NewBridgeTokenRotateRunner alias for backward compatibility.
func NewBridgeTokenRotateRunner(svc *screenshot.BridgeService) *BridgeHealthCheckRunner {
	return &BridgeHealthCheckRunner{bridgeSvc: svc}
}

func (r *BridgeHealthCheckRunner) Type() TaskType { return TaskBridgeTokenRotate }

func (r *BridgeHealthCheckRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	if r.bridgeSvc == nil {
		return "", fmt.Errorf("bridge service not available")
	}

	queueLen := r.bridgeSvc.QueueLen()
	workers := r.bridgeSvc.WorkerCount()
	inFlight := r.bridgeSvc.InFlight()
	started := r.bridgeSvc.IsStarted()

	var b strings.Builder
	fmt.Fprintf(&b, "截图桥接服务健康检查\n\n")
	if started {
		fmt.Fprintf(&b, "✅ 状态: 运行中\n")
	} else {
		fmt.Fprintf(&b, "❌ 状态: 未启动\n")
	}
	fmt.Fprintf(&b, "📊 工作线程: %d\n", workers)
	fmt.Fprintf(&b, "📊 队列长度: %d\n", queueLen)
	fmt.Fprintf(&b, "📊 进行中: %d\n", inFlight)

	result := sanitizeUTF8(b.String())
	if !started {
		return result, fmt.Errorf("bridge service is not started")
	}
	return result, nil
}

// --- AlertSilenceRunner (ST-19) ---

// AlertSilenceRunner executes scheduled alert silence windows.
type AlertSilenceRunner struct {
	alertManager *alerting.Manager
}

// NewAlertSilenceRunner creates an AlertSilenceRunner.
func NewAlertSilenceRunner(alertManager *alerting.Manager) *AlertSilenceRunner {
	return &AlertSilenceRunner{alertManager: alertManager}
}

func (r *AlertSilenceRunner) Type() TaskType { return TaskAlertSilence }

func (r *AlertSilenceRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	if r.alertManager == nil {
		return "", fmt.Errorf("alert manager not available")
	}

	alertType := extractString(payload, "alert_type", "")
	durationMin := extractInt(payload, "duration_minutes", 60)
	duration := time.Duration(durationMin) * time.Minute

	if alertType != "" {
		r.alertManager.SilenceAlertsByType(alerting.AlertType(alertType), duration)
		var b strings.Builder
		fmt.Fprintf(&b, "告警静默设置完成\n\n")
		fmt.Fprintf(&b, "✅ 告警类型: %s\n", alertType)
		fmt.Fprintf(&b, "✅ 静默时长: %d 分钟\n", durationMin)
		return sanitizeUTF8(b.String()), nil
	}

	// No type specified: cleanup old records instead
	maxAgeDays := extractInt(payload, "max_age_days", 30)
	r.alertManager.CleanupOldRecords(time.Duration(maxAgeDays) * 24 * time.Hour)
	var b strings.Builder
	fmt.Fprintf(&b, "告警记录清理完成\n\n")
	fmt.Fprintf(&b, "✅ 保留天数: %d 天\n", maxAgeDays)
	fmt.Fprintf(&b, "✅ 已清理过期记录\n")
	return sanitizeUTF8(b.String()), nil
}

// --- URLHealthChecker (ST-20) ---

// URLHealthChecker executes scheduled URL reachability health checks.
// Despite the legacy task type name "cache_warmup", this runner performs
// HTTP GET requests against configured URLs to verify they are reachable —
// effectively acting as a URL health checker rather than warming the
// application query cache.
type URLHealthChecker struct {
	// No direct dependency — probes configured URLs via HTTP GET.
}

// NewURLHealthChecker creates a URLHealthChecker.
func NewURLHealthChecker() *URLHealthChecker {
	return &URLHealthChecker{}
}

// CacheWarmupRunner is a deprecated alias for URLHealthChecker.
// Renamed because the runner performs HTTP GET reachability checks,
// not application cache warming.
type CacheWarmupRunner = URLHealthChecker

// NewCacheWarmupRunner creates a CacheWarmupRunner (alias for NewURLHealthChecker).
func NewCacheWarmupRunner() *CacheWarmupRunner {
	return NewURLHealthChecker()
}

func (r *URLHealthChecker) Type() TaskType { return TaskCacheWarmup }

func (r *URLHealthChecker) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	urls := extractStrings(payload, "warmup_urls", []string{})
	if len(urls) == 0 {
		return "URL 健康检查完成\n\n⚠️ 未配置 warmup_urls", nil
	}

	client := urlguard.SafeHTTPClient(urlguard.CheckOptions{
		AllowedSchemes: []string{"http", "https"},
		AllowPrivate:   false,
	}, 10*time.Second)
	successCount := 0
	failedCount := 0
	var b strings.Builder
	fmt.Fprintf(&b, "URL 健康检查完成：%d 个 URL\n\n", len(urls))
	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			fmt.Fprintf(&b, "❌ %s: 请求构建失败\n", u)
			failedCount++
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(&b, "❌ %s: %v\n", u, err)
			failedCount++
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			fmt.Fprintf(&b, "✅ %s (HTTP %d)\n", u, resp.StatusCode)
			successCount++
		} else {
			fmt.Fprintf(&b, "⚠️ %s (HTTP %d)\n", u, resp.StatusCode)
			failedCount++
		}
	}
	fmt.Fprintf(&b, "\n📊 成功: %d，失败: %d\n", successCount, failedCount)
	return sanitizeUTF8(b.String()), nil
}

// --- ICPQueryRunner (ST-21) ---

const icpMaxQueries = 100
const icpMaxPageSize = 100

// ICPResultStore is the subset of the repository interface used by ICPQueryRunner.
type ICPResultStore interface {
	SaveRun(run *icpdb.ICPQueryRun) (int64, error)
	SaveResults(runID int64, results []adapter.ICPResult, fetchedAt time.Time) error
	GetLatestResults(keyword, queryType string) ([]*icpdb.ICPResultRow, error)
	GetPreviousResults(keyword, queryType string, before time.Time) ([]*icpdb.ICPResultRow, error)
}

// ICPAlertSender sends ICP change alerts.
type ICPAlertSender interface {
	SendWarning(alertType alerting.AlertType, title, message string, details interface{}, source, url string)
}

// ICPQueryRunner executes scheduled ICP备案 queries.
type ICPQueryRunner struct {
	cfgProvider func() adapter.ICPConfig
	store       ICPResultStore
	alertSender ICPAlertSender
}

// NewICPQueryRunner creates an ICPQueryRunner.
// cfgProvider must return a full ICP config snapshot; it is called on every
// execution so hot-reloaded config values (timeout, default_type, etc.) are
// always current. store may be nil, in which case results are not persisted.
// alertSender may be nil, in which case change alerts are not sent.
func NewICPQueryRunner(p func() adapter.ICPConfig, store ICPResultStore, alertSender ICPAlertSender) *ICPQueryRunner {
	return &ICPQueryRunner{cfgProvider: p, store: store, alertSender: alertSender}
}

func (r *ICPQueryRunner) Type() TaskType { return TaskICPQuery }

func (r *ICPQueryRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	cfg := r.cfgProvider()

	if !cfg.Enabled {
		return "", fmt.Errorf("ICP query is disabled")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return "", fmt.Errorf("ICP base_url not configured")
	}

	queries := extractStrings(payload, "queries", nil)
	if len(queries) == 0 {
		if q := extractString(payload, "query", ""); q != "" {
			queries = []string{q}
		}
	}
	if len(queries) == 0 {
		return "", fmt.Errorf("missing 'queries' or 'query' in payload")
	}
	if len(queries) > icpMaxQueries {
		return "", fmt.Errorf("too many queries (%d), maximum is %d", len(queries), icpMaxQueries)
	}

	// 解析 type 参数：支持逗号分隔的多类型
	rawType := extractString(payload, "type", cfg.DefaultType)
	if rawType == "" {
		rawType = "web"
	}
	var types []string
	seen := make(map[string]bool)
	for _, part := range strings.Split(rawType, ",") {
		t := strings.TrimSpace(part)
		if t == "" {
			continue
		}
		if !adapter.IsValidICPQueryType(t) {
			return "", fmt.Errorf("invalid ICP query type: %q", t)
		}
		if !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	if len(types) == 0 {
		types = []string{"web"}
	}

	page := extractInt(payload, "page", 1)
	pageSize := extractInt(payload, "page_size", 20)
	if pageSize > icpMaxPageSize {
		pageSize = icpMaxPageSize
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	failFast := extractBool(payload, "fail_fast", false)
	taskID := extractString(payload, "_task_id", "")

	totalRecords := 0
	succeeded := 0
	var errs []string
	type icpQueryResult struct {
		query string
		qtype string
		total int
		domains []string
	}
	var queryResults []icpQueryResult

	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := cfg.APIKey
	startedAt := time.Now()

	for _, q := range queries {
		for _, queryType := range types {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}

			results, total, err := adapter.ICPSearchWithContext(ctx, baseURL, apiKey, adapter.ICPSearchRequest{
				Query:    q,
				Type:     queryType,
				Page:     page,
				PageSize: pageSize,
			})
			if err != nil {
				errs = append(errs, fmt.Sprintf("%q [type=%s]: %s", q, queryType, err.Error()))
				if failFast {
					break
				}
				continue
			}
			succeeded++
			totalRecords += total

			// 收集逐条结果
			var domains []string
			for _, res := range results {
				if res.Domain != "" {
					domains = append(domains, res.Domain)
				}
			}
			queryResults = append(queryResults, icpQueryResult{query: q, qtype: queryType, total: total, domains: domains})

			r.persistRun(taskID, q, queryType, page, pageSize, total, results, startedAt)
		}
		if failFast && len(errs) > 0 {
			break
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "ICP 备案查询完成（类型: %s）\n\n", strings.Join(types, ","))
	fmt.Fprintf(&b, "📊 成功: %d/%d，共 %d 条记录\n\n", succeeded, len(queries)*len(types), totalRecords)
	for _, qr := range queryResults {
		fmt.Fprintf(&b, "✅ %s [%s]: %d 条", qr.query, qr.qtype, qr.total)
		if len(qr.domains) > 0 {
			show := qr.domains
			if len(show) > 5 {
				show = show[:5]
			}
			fmt.Fprintf(&b, " — %s", strings.Join(show, ", "))
			if len(qr.domains) > 5 {
				fmt.Fprintf(&b, " 等%d个", len(qr.domains))
			}
		}
		b.WriteString("\n")
	}
	for _, e := range errs {
		fmt.Fprintf(&b, "❌ %s\n", e)
	}
	result := sanitizeUTF8(b.String())

	if succeeded == 0 && len(errs) > 0 {
		return result, fmt.Errorf("all %d ICP query(ies) failed", len(errs))
	}
	return result, nil
}

// persistRun saves a single query's run and results to the database.
// If an alertSender is configured and previous results exist, it compares
// licence/unit_name/update_record fields and sends change alerts.
func (r *ICPQueryRunner) persistRun(taskID, keyword, queryType string, page, pageSize, total int, results []adapter.ICPResult, startedAt time.Time) {
	if r.store == nil {
		return
	}
	run := &icpdb.ICPQueryRun{
		TaskID:       taskID,
		QueryKeyword: keyword,
		QueryType:    queryType,
		Page:         page,
		PageSize:     pageSize,
		TotalRecords: total,
		ResultCount:  len(results),
		StartedAt:    startedAt,
	}
	runID, err := r.store.SaveRun(run)
	if err != nil {
		return
	}
	if err := r.store.SaveResults(runID, results, time.Now()); err != nil {
		logger.Errorf("ICP: failed to persist results for run %s: %v", runID, err)
	}

	// Check for备案 changes and alert.
	if r.alertSender == nil || len(results) == 0 {
		return
	}
	previous, _ := r.store.GetPreviousResults(keyword, queryType, startedAt)
	if len(previous) == 0 {
		return
	}

	prevMap := make(map[string]*icpdb.ICPResultRow, len(previous))
	for _, p := range previous {
		if p.Domain != "" {
			prevMap[p.Domain] = p
		}
	}

	var changes []string
	for _, res := range results {
		if res.Domain == "" {
			continue
		}
		p, ok := prevMap[res.Domain]
		if !ok {
			changes = append(changes, fmt.Sprintf("%s: new record", res.Domain))
			continue
		}
		if p.Licence != res.Licence && res.Licence != "" {
			changes = append(changes, fmt.Sprintf("%s: licence %s -> %s", res.Domain, p.Licence, res.Licence))
		}
		if p.UnitName != res.UnitName && res.UnitName != "" {
			changes = append(changes, fmt.Sprintf("%s: unit %s -> %s", res.Domain, p.UnitName, res.UnitName))
		}
		if p.UpdateRecord != res.UpdateRecord && res.UpdateRecord != "" {
			changes = append(changes, fmt.Sprintf("%s: update_record %s -> %s", res.Domain, p.UpdateRecord, res.UpdateRecord))
		}
	}

	if len(changes) == 0 {
		return
	}

	title := fmt.Sprintf("ICP备案变更: %s", keyword)
	message := fmt.Sprintf("检测到 %d 项变更 (type=%s):\n%s", len(changes), queryType, strings.Join(changes, "\n"))
	r.alertSender.SendWarning(alerting.AlertTypeICP, title, message, map[string]interface{}{
		"keyword": keyword,
		"type":    queryType,
		"changes": changes,
	}, "scheduler", "")
}

// --- ICPImportRunner (ST-22) ---

const icpImportMaxRows = 1000

// ICPImportRunner reads keyword lists from CSV files and creates ICP query tasks.
type ICPImportRunner struct {
	importDir string
	scheduler *Scheduler
}

// NewICPImportRunner creates an ICPImportRunner.
// scheduler is optional; if provided, imported keywords are auto-queued as ICP tasks.
func NewICPImportRunner(importDir string, scheduler *Scheduler) *ICPImportRunner {
	return &ICPImportRunner{importDir: importDir, scheduler: scheduler}
}

func (r *ICPImportRunner) Type() TaskType { return TaskICPImport }

func (r *ICPImportRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
	if r.importDir == "" {
		return "", fmt.Errorf("import directory not configured")
	}

	filePattern := extractString(payload, "file_pattern", "*.csv")
	queryType := extractString(payload, "type", "web")
	maxRows := extractInt(payload, "max_rows", icpImportMaxRows)
	if maxRows > icpImportMaxRows {
		maxRows = icpImportMaxRows
	}

	matches, err := filepath.Glob(filepath.Join(r.importDir, filePattern))
	if err != nil {
		return "", fmt.Errorf("glob failed: %w", err)
	}
	if len(matches) == 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "ICP 关键词导入完成\n\n")
		fmt.Fprintf(&b, "⚠️ 未找到匹配文件: %s\n", filePattern)
		fmt.Fprintf(&b, "📁 搜索目录: %s\n", r.importDir)
		return sanitizeUTF8(b.String()), nil
	}

	var queries []string
	fileDetails := make([]struct {
		name  string
		count int
		err   error
	}, 0, len(matches))

	for _, filePath := range matches {
		rows, err := readKeywordsFromCSV(filePath, maxRows-len(queries))
		if err != nil {
			fileDetails = append(fileDetails, struct {
				name  string
				count int
				err   error
			}{filepath.Base(filePath), 0, err})
			continue
		}
		queries = append(queries, rows...)
		fileDetails = append(fileDetails, struct {
			name  string
			count int
			err   error
		}{filepath.Base(filePath), len(rows), nil})
	}

	if len(queries) == 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "ICP 关键词导入完成\n\n")
		for _, fd := range fileDetails {
			if fd.err != nil {
				fmt.Fprintf(&b, "❌ %s: 读取失败 — %v\n", fd.name, fd.err)
			} else {
				fmt.Fprintf(&b, "⚠️ %s: 无关键词\n", fd.name)
			}
		}
		return sanitizeUTF8(b.String()), nil
	}

	if r.scheduler != nil {
		task := &ScheduledTask{
			Name:       fmt.Sprintf("ICP import batch %s", filePattern),
			Type:       TaskICPQuery,
			CronExpr:   "0 0 * * * *", // run once immediately
			Payload:    map[string]interface{}{"queries": queries, "type": queryType},
			TimeoutSec: 600,
			MaxRetries: 1,
			Enabled:    true,
		}
		if err := r.scheduler.AddTask(task); err != nil {
			return "", fmt.Errorf("failed to create ICP task: %w", err)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "ICP 关键词导入完成：%d 个文件\n\n", len(matches))
	for _, fd := range fileDetails {
		if fd.err != nil {
			fmt.Fprintf(&b, "❌ %s: 读取失败 — %v\n", fd.name, fd.err)
		} else {
			fmt.Fprintf(&b, "✅ %s: %d 个关键词\n", fd.name, fd.count)
		}
	}
	fmt.Fprintf(&b, "\n📊 共导入: %d 个关键词\n", len(queries))
	fmt.Fprintf(&b, "📋 查询类型: %s\n", queryType)
	fmt.Fprintf(&b, "🚀 已创建 ICP 查询任务\n")
	return sanitizeUTF8(b.String()), nil
}

func readKeywordsFromCSV(filePath string, maxRows int) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // allow variable fields

	var keywords []string
	rowCount := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if rowCount == 0 {
			// Skip header row if it looks like column names
			if len(record) > 0 && isCSVHeader(record[0]) {
				rowCount++
				continue
			}
		}
		if rowCount >= maxRows {
			break
		}
		// First column is the keyword
		if len(record) > 0 {
			kw := strings.TrimSpace(record[0])
			if kw != "" && !strings.HasPrefix(kw, "#") {
				keywords = append(keywords, kw)
			}
		}
		rowCount++
	}
	return keywords, nil
}

func isCSVHeader(s string) bool {
	lower := strings.ToLower(s)
	return lower == "keyword" || lower == "domain" || lower == "company" || lower == "query" || lower == "name"
}
