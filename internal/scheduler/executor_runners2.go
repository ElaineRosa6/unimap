package scheduler

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/alerting"
	icpdb "github.com/unimap/project/internal/icp/database"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/service"
	"github.com/unimap/project/internal/utils/urlguard"
)

// --- AlertSummaryRunner (ST-14) ---

type AlertSummaryRunner struct {
	alertManager *alerting.Manager
}

func NewAlertSummaryRunner(alertManager *alerting.Manager) *AlertSummaryRunner {
	return &AlertSummaryRunner{alertManager: alertManager}
}

func (r *AlertSummaryRunner) Type() TaskType { return TaskAlertSummary }

func (r *AlertSummaryRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
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

type BaselineRefreshRunner struct {
	tamperSvc  *service.TamperAppService
	pageLoader service.TamperPageLoader
}

func NewBaselineRefreshRunner(svc *service.TamperAppService, loaders ...service.TamperPageLoader) *BaselineRefreshRunner {
	var loader service.TamperPageLoader
	if len(loaders) > 0 {
		loader = loaders[0]
	}
	return &BaselineRefreshRunner{tamperSvc: svc, pageLoader: loader}
}

func (r *BaselineRefreshRunner) Type() TaskType { return TaskBaselineRefresh }

func (r *BaselineRefreshRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.tamperSvc == nil {
		return "", fmt.Errorf("tamper service not available")
	}

	urls := extractStrings(payload, "urls", []string{})
	if len(urls) == 0 {
		baselines, err := r.tamperSvc.ListBaselines()
		if err != nil {
			return "", fmt.Errorf("list baselines failed: %w", err)
		}
		if len(baselines) == 0 {
			return "no baselines to refresh", nil
		}
		urls = baselines
	}

	// Concurrency: clamp to >=1. Previously baselines were refreshed one URL
	// at a time, ignoring the payload's concurrency field; large baseline sets
	// were slow. We now run SetBaseline concurrently with a semaphore.
	concurrency := extractInt(payload, "concurrency", 5)
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(urls) {
		concurrency = len(urls)
	}

	var (
		mu         sync.Mutex
		refreshed  int
		failed     int
		failedURLs []string
		wg         sync.WaitGroup
	)
	sem := make(chan struct{}, concurrency)

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			req := service.TamperBaselineRequest{URLs: []string{url}}
			response, err := r.tamperSvc.SetBaseline(ctx, req, r.pageLoader)
			mu.Lock()
			defer mu.Unlock()
			if err != nil || response == nil || response.Summary["saved"] != 1 {
				failed++
				failedURLs = append(failedURLs, url)
				return
			}
			refreshed++
		}(url)
	}
	wg.Wait()

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

type URLImportRunner struct {
	importDir string
}

func NewURLImportRunner(importDir string) *URLImportRunner {
	return &URLImportRunner{importDir: importDir}
}

func (r *URLImportRunner) Type() TaskType { return TaskURLImport }

func (r *URLImportRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.importDir == "" {
		return "", fmt.Errorf("import directory not configured")
	}

	filePattern, err := sanitizeImportPattern(extractString(payload, "file_pattern", "*.txt"))
	if err != nil {
		return "", fmt.Errorf("invalid file_pattern: %w", err)
	}
	maxLines := extractInt(payload, "max_lines", 10000)

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
	type fileDetail struct {
		name  string
		count int
		err   error
	}
	fileDetails := make([]fileDetail, 0, len(matches))

	for _, filePath := range matches {
		urls, readErr := readURLsFromFile(filePath, maxLines-len(importedURLs))
		if readErr != nil {
			fileDetails = append(fileDetails, fileDetail{filepath.Base(filePath), 0, readErr})
			continue
		}
		importedURLs = append(importedURLs, urls...)
		fileDetails = append(fileDetails, fileDetail{filepath.Base(filePath), len(urls), nil})
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

// sanitizeImportPattern 校验任务 file_pattern（FINDING-001/004 路径穿越修复）。
// file_pattern 来自任务 payload，未校验时 `..` 可借 filepath.Glob 逃逸 importDir 越界。
// 仅允许 importDir 内的相对 glob：拒绝空值、绝对路径、以 / 或 \ 开头的根相对路径，
// 以及含 `..` 段的路径（filepath.Clean 不会消除前导 `..`，逐段检查覆盖所有逃逸形态）。
func sanitizeImportPattern(pattern string) (string, error) {
	if strings.TrimSpace(pattern) == "" {
		return "", fmt.Errorf("file_pattern must not be empty")
	}
	if filepath.IsAbs(pattern) {
		return "", fmt.Errorf("file_pattern must be relative: %q", pattern)
	}
	// Windows 上 filepath.IsAbs 不识别 `/` 或 `\` 开头的根相对路径，单独拒绝。
	if strings.HasPrefix(pattern, "/") || strings.HasPrefix(pattern, "\\") {
		return "", fmt.Errorf("file_pattern must be relative: %q", pattern)
	}
	cleaned := filepath.Clean(pattern)
	for _, seg := range strings.Split(cleaned, string(filepath.Separator)) {
		if seg == ".." {
			return "", fmt.Errorf("file_pattern must not contain '..': %q", pattern)
		}
	}
	return cleaned, nil
}

// --- PluginHealthRunner (ST-17) ---

type PluginHealthRunner struct {
	unifiedSvc *service.UnifiedService
}

func NewPluginHealthRunner(svc *service.UnifiedService) *PluginHealthRunner {
	return &PluginHealthRunner{unifiedSvc: svc}
}

func (r *PluginHealthRunner) Type() TaskType { return TaskPluginHealth }

func (r *PluginHealthRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
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

type BridgeHealthCheckRunner struct {
	bridgeSvc         *screenshot.BridgeService
	bridgeSvcProvider func() *screenshot.BridgeService
}

// NewBridgeTokenRotateRunner preserves the legacy constructor and task key.
// Despite its historical name, the bridge_token task performs a health check.
func NewBridgeTokenRotateRunner(svc *screenshot.BridgeService) *BridgeHealthCheckRunner {
	return &BridgeHealthCheckRunner{bridgeSvc: svc}
}

// NewBridgeHealthCheckRunnerWithProvider resolves the bridge service only when
// the task runs. This lets the scheduler register before screenshot mode creates
// the extension bridge.
func NewBridgeHealthCheckRunnerWithProvider(provider func() *screenshot.BridgeService) *BridgeHealthCheckRunner {
	return &BridgeHealthCheckRunner{bridgeSvcProvider: provider}
}

func (r *BridgeHealthCheckRunner) Type() TaskType { return TaskBridgeHealthCheck }

func (r *BridgeHealthCheckRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	bridgeSvc := r.bridgeSvc
	if r.bridgeSvcProvider != nil {
		bridgeSvc = r.bridgeSvcProvider()
	}
	if bridgeSvc == nil {
		return "", fmt.Errorf("bridge service not available")
	}

	queueLen := bridgeSvc.QueueLen()
	workers := bridgeSvc.WorkerCount()
	inFlight := bridgeSvc.InFlight()
	started := bridgeSvc.IsStarted()

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

type AlertSilenceRunner struct {
	alertManager *alerting.Manager
}

func NewAlertSilenceRunner(alertManager *alerting.Manager) *AlertSilenceRunner {
	return &AlertSilenceRunner{alertManager: alertManager}
}

func (r *AlertSilenceRunner) Type() TaskType { return TaskAlertSilence }

func (r *AlertSilenceRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
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

	maxAgeDays := extractInt(payload, "max_age_days", 30)
	r.alertManager.CleanupOldRecords(time.Duration(maxAgeDays) * 24 * time.Hour)
	var b strings.Builder
	fmt.Fprintf(&b, "告警记录清理完成\n\n")
	fmt.Fprintf(&b, "✅ 保留天数: %d 天\n", maxAgeDays)
	fmt.Fprintf(&b, "✅ 已清理过期记录\n")
	return sanitizeUTF8(b.String()), nil
}

// --- URLHealthChecker (ST-20) ---

type URLHealthChecker struct{}

func NewURLHealthChecker() *URLHealthChecker {
	return &URLHealthChecker{}
}

// CacheWarmupRunner is a deprecated alias for URLHealthChecker.
type CacheWarmupRunner = URLHealthChecker

func NewCacheWarmupRunner() *CacheWarmupRunner {
	return NewURLHealthChecker()
}

func (r *URLHealthChecker) Type() TaskType { return TaskCacheWarmup }

func (r *URLHealthChecker) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
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

// icpRetryDefaultCount / icpRetryDefaultBackoffMs 控制单次 ICP 查询失败后的重试。
// sidecar 对 ICP 官方接口有速率限制，瞬时 429/5xx 会造成 "ICP API error"；
// 重试（短退避）可自愈，最终失败才计入通知的错误行。
const icpRetryDefaultCount = 2
const icpRetryDefaultBackoffMs = 1500

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

type ICPQueryRunner struct {
	cfgProvider func() adapter.ICPConfig
	store       ICPResultStore
	alertSender ICPAlertSender
}

func NewICPQueryRunner(p func() adapter.ICPConfig, store ICPResultStore, alertSender ICPAlertSender) *ICPQueryRunner {
	return &ICPQueryRunner{cfgProvider: p, store: store, alertSender: alertSender}
}

func (r *ICPQueryRunner) Type() TaskType { return TaskICPQuery }

type icpQueryResult struct {
	query   string
	qtype   string
	total   int
	results []adapter.ICPResult
}

func (r *ICPQueryRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	cfg := r.cfgProvider()
	if err := validateICPConfig(cfg); err != nil {
		return "", err
	}

	queries, err := extractICPQueries(payload)
	if err != nil {
		return "", err
	}

	types, err := parseICPTypes(payload, cfg.DefaultType)
	if err != nil {
		return "", err
	}

	page, pageSize := extractICPPagination(payload)
	failFast := extractBool(payload, "fail_fast", false)
	taskID := extractString(payload, "_task_id", "")

	// 拟人请求间隔配置（可通过 payload 覆盖默认值）
	companyIntervalMin := extractInt(payload, "request_interval_min", 30)
	companyIntervalMax := extractInt(payload, "request_interval_max", 90)
	typeIntervalMin := extractInt(payload, "type_interval_min", 3)
	typeIntervalMax := extractInt(payload, "type_interval_max", 8)
	// 单查询失败重试（sidecar 速率限制自愈）
	retryCount := extractInt(payload, "retry_count", icpRetryDefaultCount)
	retryBackoffMs := extractInt(payload, "retry_backoff_ms", icpRetryDefaultBackoffMs)

	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := cfg.APIKey
	startedAt := time.Now()

	execRes := r.executeICPQueries(
		ctx, queries, types, baseURL, apiKey, page, pageSize, failFast, taskID, startedAt,
		companyIntervalMin, companyIntervalMax, typeIntervalMin, typeIntervalMax,
		retryCount, retryBackoffMs,
	)
	if execRes.ctxErr != nil {
		return "", execRes.ctxErr
	}

	result := formatICPResults(types, queries, execRes.succeeded, execRes.totalRecords, execRes.queryResults, execRes.errs)

	if execRes.succeeded == 0 && len(execRes.errs) > 0 {
		return result, fmt.Errorf("all %d ICP query(ies) failed", len(execRes.errs))
	}
	return result, nil
}

func validateICPConfig(cfg adapter.ICPConfig) error {
	if !cfg.Enabled {
		return fmt.Errorf("ICP query is disabled")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return fmt.Errorf("ICP base_url not configured")
	}
	return nil
}

func extractICPQueries(payload *model.TaskPayload) ([]string, error) {
	queries := extractStrings(payload, "queries", nil)
	if len(queries) == 0 {
		if q := extractString(payload, "query", ""); q != "" {
			queries = []string{q}
		}
	}
	if len(queries) == 0 {
		return nil, fmt.Errorf("%s runner: missing 'queries' or 'query' in payload", TaskICPQuery)
	}
	if len(queries) > icpMaxQueries {
		return nil, fmt.Errorf("too many queries (%d), maximum is %d", len(queries), icpMaxQueries)
	}
	return queries, nil
}

func parseICPTypes(payload *model.TaskPayload, defaultType string) ([]string, error) {
	rawType := extractString(payload, "type", defaultType)
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
			return nil, fmt.Errorf("invalid ICP query type: %q", t)
		}
		if !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	if len(types) == 0 {
		types = []string{"web"}
	}
	return types, nil
}

func extractICPPagination(payload *model.TaskPayload) (page, pageSize int) {
	page = extractInt(payload, "page", 1)
	pageSize = extractInt(payload, "icp_page_size", 0)
	if pageSize <= 0 {
		pageSize = extractInt(payload, "page_size", 20)
	}
	if pageSize > icpMaxPageSize {
		pageSize = icpMaxPageSize
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return page, pageSize
}

type icpExecResult struct {
	totalRecords int
	succeeded    int
	errs         []string
	queryResults []icpQueryResult
	ctxErr       error
}

func (r *ICPQueryRunner) executeICPQueries(
	ctx context.Context, queries, types []string, baseURL, apiKey string,
	page, pageSize int, failFast bool, taskID string, startedAt time.Time,
	companyIntervalMin, companyIntervalMax, typeIntervalMin, typeIntervalMax,
	retryCount, retryBackoffMs int,
) icpExecResult {
	var res icpExecResult
	for i, q := range queries {
		for j, queryType := range types {
			select {
			case <-ctx.Done():
				res.ctxErr = ctx.Err()
				return res
			default:
			}

			results, total, err := icpSearchWithRetry(ctx, baseURL, apiKey, q, queryType, page, pageSize, retryCount, retryBackoffMs)
			if err != nil {
				res.errs = append(res.errs, fmt.Sprintf("%q [type=%s]: %s", q, queryType, err.Error()))
				if failFast {
					break
				}
			} else {
				res.succeeded++
				res.totalRecords += total

				res.queryResults = append(res.queryResults, icpQueryResult{query: q, qtype: queryType, total: total, results: results})

				r.persistRun(taskID, q, queryType, page, pageSize, total, results, startedAt)
			}

			// 类型间间隔（非最后一个类型）：随机 3-8 秒，模拟人切换类型
			if j < len(types)-1 {
				delay := time.Duration(randInt(typeIntervalMin, typeIntervalMax)) * time.Second
				logger.Infof("[scheduler] ICP: waiting %v between types for %q", delay, q)
				select {
				case <-ctx.Done():
					res.ctxErr = ctx.Err()
					return res
				case <-time.After(delay):
				}
			}
		}
		if failFast && len(res.errs) > 0 {
			break
		}
		// 公司间间隔（非最后一个公司）：随机 30-90 秒，模拟人查看结果
		if i < len(queries)-1 {
			delay := time.Duration(randInt(companyIntervalMin, companyIntervalMax)) * time.Second
			logger.Infof("[scheduler] ICP: waiting %v before next company %q", delay, queries[i+1])
			select {
			case <-ctx.Done():
				res.ctxErr = ctx.Err()
				return res
			case <-time.After(delay):
			}
		}
	}
	return res
}

// randInt returns a random integer in [min, max] (inclusive).
func randInt(min, max int) int {
	if min >= max {
		return min
	}
	return min + rand.Intn(max-min+1)
}

// icpSearchWithRetry performs a single ICP sidecar query, retrying transient
// failures (rate limits / 5xx) with a short random backoff. A permanent error
// or the final attempt returns the error unchanged. Context cancellation
// aborts the retry loop and surfaces ctx.Err().
func icpSearchWithRetry(ctx context.Context, baseURL, apiKey, query, queryType string, page, pageSize, retryCount, backoffMs int) ([]adapter.ICPResult, int, error) {
	var results []adapter.ICPResult
	var total int
	var lastErr error
	for attempt := 0; attempt <= retryCount; attempt++ {
		results, total, lastErr = adapter.ICPSearchWithContext(ctx, baseURL, apiKey, adapter.ICPSearchRequest{
			Query:    query,
			Type:     queryType,
			Page:     page,
			PageSize: pageSize,
		})
		if lastErr == nil {
			return results, total, nil
		}
		if attempt == retryCount {
			break
		}
		// 短退避：1000–2000ms 随机，避免再次撞上速率限制窗口
		delay := time.Duration(backoffMs/2+randInt(0, backoffMs/2)) * time.Millisecond
		logger.Warnf("[scheduler] ICP: query %q type=%s attempt %d/%d failed (%v), retrying in %v", query, queryType, attempt+1, retryCount+1, lastErr, delay)
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(delay):
		}
	}
	return results, total, lastErr
}

func formatICPResults(types, queries []string, succeeded, totalRecords int, queryResults []icpQueryResult, errs []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ICP 备案查询完成（%s）\n\n", strings.Join(types, ","))
	fmt.Fprintf(&b, "📊 成功: %d/%d，共 %d 条记录\n", succeeded, len(queries)*len(types), totalRecords)
	for _, qr := range queryResults {
		if qr.total <= 0 {
			continue // 无备案记录，不刷屏
		}
		fmt.Fprintf(&b, "\n✅ %s [%s]: %d 条\n", qr.query, qr.qtype, qr.total)
		for _, r := range qr.results {
			name := r.Domain
			if name == "" {
				name = r.ServiceName
			}
			if name == "" {
				name = r.ContentName
			}
			if name == "" {
				name = "(无名称)"
			}
			// licence fallback：Licence → MainLicWeb → MainLicence（adapter 内同名方法未导出）
			licence := r.Licence
			if licence == "" {
				licence = r.MainLicWeb
			}
			if licence == "" {
				licence = r.MainLicence
			}
			fmt.Fprintf(&b, "  • %s｜%s\n", name, licence)
		}
	}
	for _, e := range errs {
		fmt.Fprintf(&b, "\n❌ %s", e)
	}
	return sanitizeUTF8(b.String())
}

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

type ICPImportRunner struct {
	importDir string
	scheduler *Scheduler
}

func NewICPImportRunner(importDir string, scheduler *Scheduler) *ICPImportRunner {
	return &ICPImportRunner{importDir: importDir, scheduler: scheduler}
}

func (r *ICPImportRunner) Type() TaskType { return TaskICPImport }

func (r *ICPImportRunner) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	if r.importDir == "" {
		return "", fmt.Errorf("import directory not configured")
	}

	filePattern, err := sanitizeImportPattern(extractString(payload, "file_pattern", "*.csv"))
	if err != nil {
		return "", fmt.Errorf("invalid file_pattern: %w", err)
	}
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
	type fileDetail struct {
		name  string
		count int
		err   error
	}
	fileDetails := make([]fileDetail, 0, len(matches))

	for _, filePath := range matches {
		rows, readErr := readKeywordsFromCSV(filePath, maxRows-len(queries))
		if readErr != nil {
			fileDetails = append(fileDetails, fileDetail{filepath.Base(filePath), 0, readErr})
			continue
		}
		queries = append(queries, rows...)
		fileDetails = append(fileDetails, fileDetail{filepath.Base(filePath), len(rows), nil})
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
			CronExpr:   "0 0 * * * *",
			Payload:    &model.TaskPayload{Queries: queries, Type: queryType},
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
	reader.FieldsPerRecord = -1

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
			if len(record) > 0 && isCSVHeader(record[0]) {
				rowCount++
				continue
			}
		}
		if rowCount >= maxRows {
			break
		}
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
