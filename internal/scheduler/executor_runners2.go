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
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/alerting"
	icpdb "github.com/unimap/project/internal/icp/database"
	"github.com/unimap/project/internal/logger"
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

type BaselineRefreshRunner struct {
	tamperSvc *service.TamperAppService
}

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
		req := service.TamperBaselineRequest{URLs: []string{url}}
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

type URLImportRunner struct {
	importDir string
}

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

// --- PluginHealthRunner (ST-17) ---

type PluginHealthRunner struct {
	unifiedSvc *service.UnifiedService
}

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

type BridgeHealthCheckRunner struct {
	bridgeSvc *screenshot.BridgeService
}

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

type AlertSilenceRunner struct {
	alertManager *alerting.Manager
}

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

type ICPQueryRunner struct {
	cfgProvider func() adapter.ICPConfig
	store       ICPResultStore
	alertSender ICPAlertSender
}

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
		query   string
		qtype   string
		total   int
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
