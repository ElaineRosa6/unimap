// Package scheduler provides cron-based task scheduling for UniMap operations.
package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/metrics"
	"github.com/unimap/project/internal/notify"
	"github.com/unimap/project/internal/utils/urlguard"
)

// TaskType identifies the type of scheduled task.
type TaskType string

const (
	TaskQuery             TaskType = "query"              // ST-01: UQL 查询
	TaskSearchScreenshot  TaskType = "search_screenshot"  // ST-02: 搜索引擎截图
	TaskBatchScreenshot   TaskType = "batch_screenshot"   // ST-03: 批量截图
	TaskTamperCheck       TaskType = "tamper_check"       // ST-04: 篡改检测
	TaskURLReachability   TaskType = "url_reachability"   // ST-05: URL 可达性检测
	TaskCookieVerify      TaskType = "cookie_verify"      // ST-06: Cookie 验证
	TaskLoginStatusCheck  TaskType = "login_status_check" // ST-07: 登录状态检测
	TaskDistributedSubmit TaskType = "distributed_submit" // ST-08: 分布式任务提交

	// ST-09 ~ ST-16: 中优先级 Runner
	TaskExport            TaskType = "export"             // ST-09: 数据导出
	TaskPortScan          TaskType = "port_scan"          // ST-10: 端口扫描
	TaskScreenshotCleanup TaskType = "screenshot_cleanup" // ST-11: 截图清理
	TaskTamperCleanup     TaskType = "tamper_cleanup"     // ST-12: 篡改记录清理
	TaskQuotaMonitor      TaskType = "quota_monitor"      // ST-13: 配额监控
	TaskAlertSummary      TaskType = "alert_summary"      // ST-14: 告警汇总
	TaskBaselineRefresh   TaskType = "baseline_refresh"   // ST-15: 基线刷新
	TaskURLImport         TaskType = "url_import"         // ST-16: URL 导入

	// ST-17 ~ ST-20: 低优先级 Runner
	TaskPluginHealth      TaskType = "plugin_health" // ST-17: 插件健康检查
	TaskBridgeTokenRotate TaskType = "bridge_token"  // ST-18: Bridge 令牌轮换
	TaskAlertSilence      TaskType = "alert_silence" // ST-19: 告警静默窗口
	TaskCacheWarmup       TaskType = "cache_warmup"  // ST-20: 缓存预热
	TaskICPQuery          TaskType = "icp_query"     // ST-21: ICP 备案查询
	TaskICPImport         TaskType = "icp_import"    // ST-22: ICP 关键词 CSV 导入
)

// AllTaskTypes returns all supported task types.
func AllTaskTypes() []TaskType {
	return []TaskType{
		TaskQuery, TaskSearchScreenshot, TaskBatchScreenshot, TaskTamperCheck,
		TaskURLReachability, TaskCookieVerify, TaskLoginStatusCheck, TaskDistributedSubmit,
		TaskExport, TaskPortScan, TaskScreenshotCleanup, TaskTamperCleanup,
		TaskQuotaMonitor, TaskAlertSummary, TaskBaselineRefresh, TaskURLImport,
		TaskPluginHealth, TaskBridgeTokenRotate, TaskAlertSilence, TaskCacheWarmup,
		TaskICPQuery, TaskICPImport,
	}
}

// TaskTypeLabel returns a human-readable label for a task type.
func TaskTypeLabel(t TaskType) string {
	labels := map[TaskType]string{
		TaskQuery:             "UQL 查询",
		TaskSearchScreenshot:  "搜索引擎截图",
		TaskBatchScreenshot:   "批量截图",
		TaskTamperCheck:       "篡改检测",
		TaskURLReachability:   "URL 可达性检测",
		TaskCookieVerify:      "Cookie 验证",
		TaskLoginStatusCheck:  "登录状态检测",
		TaskDistributedSubmit: "分布式任务提交",
		TaskExport:            "数据导出",
		TaskPortScan:          "端口扫描",
		TaskScreenshotCleanup: "截图清理",
		TaskTamperCleanup:     "篡改记录清理",
		TaskQuotaMonitor:      "配额监控",
		TaskAlertSummary:      "告警汇总",
		TaskBaselineRefresh:   "基线刷新",
		TaskURLImport:         "URL 导入",
		TaskPluginHealth:      "插件健康检查",
		TaskBridgeTokenRotate: "Bridge 健康检查",
		TaskAlertSilence:      "告警静默窗口",
		TaskCacheWarmup:       "缓存预热",
		TaskICPQuery:          "ICP 备案查询",
		TaskICPImport:         "ICP 关键词导入",
	}
	if l, ok := labels[t]; ok {
		return l
	}
	return string(t)
}

// TaskGroupInfo describes a UI grouping of task types for the scheduler form.
type TaskGroupInfo struct {
	Name  string     // 分组名称，如 "查询与采集"
	Icon  string     // 分组图标 emoji
	Types []TaskType // 该分组下的任务类型（按展示顺序）
}

// GroupedTaskTypes returns task types organized into ordered UI groups.
// The union of all groups equals AllTaskTypes (verified by tests).
// See docs/SCHEDULER_OPTIMIZATION_PLAN.md §1.2A.
func GroupedTaskTypes() []TaskGroupInfo {
	return []TaskGroupInfo{
		{Name: "查询与采集", Icon: "📊", Types: []TaskType{
			TaskQuery, TaskSearchScreenshot, TaskBatchScreenshot, TaskExport, TaskICPQuery,
		}},
		{Name: "监控与检测", Icon: "🔍", Types: []TaskType{
			TaskTamperCheck, TaskURLReachability, TaskPortScan, TaskLoginStatusCheck, TaskQuotaMonitor,
		}},
		{Name: "维护与清理", Icon: "🔧", Types: []TaskType{
			TaskScreenshotCleanup, TaskTamperCleanup, TaskBaselineRefresh, TaskAlertSilence,
		}},
		{Name: "基础设施", Icon: "📡", Types: []TaskType{
			TaskCookieVerify, TaskBridgeTokenRotate, TaskPluginHealth, TaskCacheWarmup, TaskDistributedSubmit,
		}},
		{Name: "导入与汇总", Icon: "📥", Types: []TaskType{
			TaskURLImport, TaskICPImport, TaskAlertSummary,
		}},
	}
}

// TaskTypeGroup returns the UI group name for a task type, or "其他" if ungrouped.
func TaskTypeGroup(t TaskType) string {
	for _, g := range GroupedTaskTypes() {
		for _, tt := range g.Types {
			if tt == t {
				return g.Name
			}
		}
	}
	return "其他"
}

// DefaultTemplates returns a set of pre-defined task templates.
func DefaultTemplates() []TaskTemplate {
	return []TaskTemplate{
		{
			ID:          "tmpl_daily_tamper_check",
			Name:        "每日篡改检测",
			Description: "每天凌晨 2 点对所有重要 URL 进行篡改检测",
			Type:        TaskTamperCheck,
			CronExpr:    "0 0 2 * * *",
			Payload:     map[string]interface{}{"detection_mode": "full"},
			TimeoutSec:  3600,
			MaxRetries:  2,
			Tags:        []string{"security", "daily"},
		},
		{
			ID:          "tmpl_weekly_export",
			Name:        "每周数据导出",
			Description: "每周日午夜导出本周查询数据",
			Type:        TaskExport,
			CronExpr:    "0 0 0 * * 0",
			Payload:     map[string]interface{}{"format": "json"},
			TimeoutSec:  1800,
			MaxRetries:  1,
			Tags:        []string{"export", "weekly"},
		},
		{
			ID:          "tmpl_hourly_quota_check",
			Name:        "每小时配额检查",
			Description: "每小时检查各引擎 API 配额状态",
			Type:        TaskQuotaMonitor,
			CronExpr:    "0 0 * * * *",
			Payload:     map[string]interface{}{"low_threshold": 10},
			TimeoutSec:  300,
			MaxRetries:  0,
			Tags:        []string{"monitoring", "hourly"},
		},
		{
			ID:          "tmpl_daily_screenshot_cleanup",
			Name:        "每日截图清理",
			Description: "每天凌晨 3 点清理 30 天前的截图",
			Type:        TaskScreenshotCleanup,
			CronExpr:    "0 0 3 * * *",
			Payload:     map[string]interface{}{"max_age_days": 30},
			TimeoutSec:  600,
			MaxRetries:  1,
			Tags:        []string{"cleanup", "daily"},
		},
		{
			ID:          "tmpl_weekly_baseline_refresh",
			Name:        "每周基线刷新",
			Description: "每周日凌晨刷新篡改检测基线",
			Type:        TaskBaselineRefresh,
			CronExpr:    "0 0 4 * * 0",
			Payload:     map[string]interface{}{},
			TimeoutSec:  1800,
			MaxRetries:  1,
			Tags:        []string{"security", "weekly"},
		},
		{
			ID:          "tmpl_daily_cookie_verify",
			Name:        "每日 Cookie 验证",
			Description: "每天早上 8 点验证各引擎 Cookie 有效性",
			Type:        TaskCookieVerify,
			CronExpr:    "0 0 8 * * *",
			Payload:     map[string]interface{}{},
			TimeoutSec:  600,
			MaxRetries:  2,
			Tags:        []string{"auth", "daily"},
		},
		{
			ID:          "tmpl_daily_icp_company_watch",
			Name:        "每日企业备案巡检",
			Description: "每天早上 9 点查询关注企业的 ICP 备案状态（内部模板，创建任务时编辑 queries 字段）",
			Type:        TaskICPQuery,
			CronExpr:    "0 0 9 * * *",
			Payload: map[string]interface{}{
				"queries":   []string{},
				"type":      "web",
				"page":      1,
				"page_size": 40,
			},
			TimeoutSec: 600,
			MaxRetries: 1,
			Tags:       []string{"icp", "daily", "compliance"},
		},
		{
			ID:          "tmpl_weekly_icp_domain_scan",
			Name:        "每周域名备案变更扫描",
			Description: "每周一凌晨 3 点扫描目标域名 ICP 备案变更（内部模板，创建任务时编辑 queries 字段）",
			Type:        TaskICPQuery,
			CronExpr:    "0 0 3 * * 1",
			Payload: map[string]interface{}{
				"queries":   []string{},
				"type":      "web",
				"page":      1,
				"page_size": 40,
			},
			TimeoutSec: 1800,
			MaxRetries: 1,
			Tags:       []string{"icp", "weekly", "monitoring"},
		},
	}
}

// CreateTaskFromTemplate creates a new task from a template.
func (s *Scheduler) CreateTaskFromTemplate(templateID string, name string, cronExpr string) (*ScheduledTask, error) {
	var tmpl *TaskTemplate
	for _, t := range DefaultTemplates() {
		if t.ID == templateID {
			tmpl = &t
			break
		}
	}
	if tmpl == nil {
		return nil, fmt.Errorf("template %s not found", templateID)
	}

	task := &ScheduledTask{
		Name:       name,
		Type:       tmpl.Type,
		Enabled:    true,
		CronExpr:   cronExpr,
		Payload:    tmpl.Payload,
		TimeoutSec: tmpl.TimeoutSec,
		MaxRetries: tmpl.MaxRetries,
	}

	if err := s.AddTask(task); err != nil {
		return nil, fmt.Errorf("failed to create task from template: %w", err)
	}

	return task, nil
}

// ScheduledTask represents a user-configured scheduled task.
type ScheduledTask struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       TaskType               `json:"type"`
	Enabled    bool                   `json:"enabled"`
	CronExpr   string                 `json:"cron_expr"`
	Payload    map[string]interface{} `json:"payload"`
	TimeoutSec int                    `json:"timeout_seconds"`
	MaxRetries int                    `json:"max_retries"`
	LastRunAt  *time.Time             `json:"last_run_at,omitempty"`
	NextRunAt  *time.Time             `json:"next_run_at,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`

	// 高级功能字段（阶段五新增）
	DependsOn       []string            `json:"depends_on,omitempty"`       // 依赖的任务 ID 列表
	ExecutionWindow *ExecutionWindow    `json:"execution_window,omitempty"` // 执行窗口配置
	Notifications   *NotificationConfig `json:"notifications,omitempty"`    // 通知配置
}

// ExecutionWindow defines when a task is allowed to run.
type ExecutionWindow struct {
	StartHour int    `json:"start_hour"` // 0-23
	EndHour   int    `json:"end_hour"`   // 0-23
	Weekdays  []int  `json:"weekdays"`   // 0=Sunday, 1=Monday, ..., 6=Saturday
	Timezone  string `json:"timezone"`   // IANA timezone name (e.g., "Asia/Shanghai")
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Enabled    bool     `json:"enabled"`
	OnSuccess  bool     `json:"on_success"`
	OnFailure  bool     `json:"on_failure"`
	OnTimeout  bool     `json:"on_timeout"`
	ChannelIDs []string `json:"channel_ids"`           // 新字段：引用全局 channel ID
	Channels   []string `json:"channels,omitempty"`    // 旧字段：向后兼容
	WebhookURL string   `json:"webhook_url,omitempty"` // 旧字段：任务级 inline webhook
	Recipients []string `json:"recipients,omitempty"`
}

// TaskTemplate is a pre-defined task configuration that can be used to quickly create tasks.
type TaskTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        TaskType               `json:"type"`
	CronExpr    string                 `json:"cron_expr"`
	Payload     map[string]interface{} `json:"payload"`
	TimeoutSec  int                    `json:"timeout_seconds"`
	MaxRetries  int                    `json:"max_retries"`
	Tags        []string               `json:"tags"`
}

// ExecutionRecord stores the result of a task execution.
type ExecutionRecord struct {
	TaskID     string `json:"task_id"`
	TaskName   string `json:"task_name"`
	TaskType   string `json:"task_type"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	DurationMs int64  `json:"duration_ms"`
	Status     string `json:"status"` // success, failed, timeout, skipped
	Result     string `json:"result"`
	Error      string `json:"error,omitempty"`
	RetryCount int    `json:"retry_count"`
}

// TaskHandler is the interface that wraps the Execute method for a task type.
type TaskHandler interface {
	Type() TaskType
	Execute(ctx context.Context, payload map[string]interface{}) (string, error)
}

// Scheduler manages cron-based task scheduling with persistence.
type Scheduler struct {
	tasks      map[string]*ScheduledTask
	cron       *cron.Cron
	cronIDs    map[string]cron.EntryID // taskID -> cron entry ID
	handlers   map[TaskType]TaskHandler
	history    []ExecutionRecord
	store      *Store
	stopCh     chan struct{}
	stopped    bool
	mu         sync.RWMutex
	maxHistory int

	// 生命周期控制：ctx 派生给所有执行中的任务，cancel 在 Stop 时触发
	ctx    context.Context
	cancel context.CancelFunc

	// 通知系统
	notifyRegistry    *notify.Registry
	notifyCfgProvider func() *notify.NotifyGlobalCfg
	notifyWg          sync.WaitGroup
	notifyTimeout     time.Duration
	stopping          bool
}

// NewScheduler creates a new Scheduler. If storePath is non-empty, tasks are
// persisted to that JSON file.
func NewScheduler(storePath string, historyPath string, maxHistory int) *Scheduler {
	c := cron.New(cron.WithSeconds())
	// Delay cron start until caller registers handlers and tasks.
	// Call s.Start() after setup is complete.

	if maxHistory <= 0 {
		maxHistory = 500
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		tasks:      make(map[string]*ScheduledTask),
		cron:       c,
		cronIDs:    make(map[string]cron.EntryID),
		handlers:   make(map[TaskType]TaskHandler),
		history:    make([]ExecutionRecord, 0),
		maxHistory: maxHistory,
		stopCh:     make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}

	if storePath != "" {
		s.store = NewStore(storePath, historyPath)
	}

	return s
}

// SetNotifyRegistry 设置通知渠道注册表
func (s *Scheduler) SetNotifyRegistry(reg *notify.Registry) {
	s.notifyRegistry = reg
}

// SetNotifyCfgProvider 设置全局通知配置提供者
func (s *Scheduler) SetNotifyCfgProvider(provider func() *notify.NotifyGlobalCfg) {
	s.notifyCfgProvider = provider
}

// Start begins the internal cron scheduler. Call this after registering
// handlers and loading persisted tasks.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Load persists loads tasks and history from disk.
func (s *Scheduler) Load() error {
	if s.store == nil {
		return nil
	}
	tasks, history, err := s.store.Load()
	if err != nil {
		return fmt.Errorf("load scheduler data: %w", err)
	}
	for _, t := range tasks {
		s.tasks[t.ID] = t
		if t.Enabled {
			if err := s.scheduleTask(t); err != nil {
				logger.Errorf("[scheduler] failed to schedule persisted task %s (%s): %v — task loaded but will not auto-fire", t.ID, t.Name, err)
			}
		}
	}
	s.history = history
	s.updateMetrics()
	return nil
}

// Save persists current tasks and history to disk.
func (s *Scheduler) Save() error {
	if s.store == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveLocked()
}

// saveLocked persists tasks and history. Caller must hold the mutex (write or read).
func (s *Scheduler) saveLocked() error {
	if s.store == nil {
		return nil
	}
	tasks := make([]*ScheduledTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		cp := *t
		if cp.Payload != nil {
			raw, err := json.Marshal(t.Payload)
			if err != nil {
				logger.Warnf("[scheduler] failed to deep-copy payload for task %s: %v", t.ID, err)
				cp.Payload = make(map[string]interface{})
			} else {
				_ = json.Unmarshal(raw, &cp.Payload)
			}
		}
		if cp.LastRunAt != nil {
			v := *cp.LastRunAt
			cp.LastRunAt = &v
		}
		if cp.NextRunAt != nil {
			v := *cp.NextRunAt
			cp.NextRunAt = &v
		}
		tasks = append(tasks, &cp)
	}
	history := make([]ExecutionRecord, len(s.history))
	copy(history, s.history)

	return s.store.Save(tasks, history)
}

// RegisterHandler registers a task handler. Must be called before Start().
func (s *Scheduler) RegisterHandler(h TaskHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[h.Type()] = h
	metrics.SetSchedulerTasksRegistered(string(h.Type()), 1)
}

// AddTask adds a new scheduled task and schedules it in cron.
func (s *Scheduler) AddTask(task *ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sanitize task name and payload strings at creation time to handle
	// GBK-encoded input from Windows terminals / Chinese HTTP clients.
	task.Name = sanitizeUTF8(task.Name)
	task.Payload = sanitizePayload(task.Payload)

	if task.ID == "" {
		task.ID = s.generateID()
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.TimeoutSec <= 0 {
		task.TimeoutSec = 300
	}

	// Validate cron expression (5 or 6 fields)
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(task.CronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// Validate task type
	if _, ok := s.handlers[task.Type]; !ok {
		return fmt.Errorf("unknown task type: %s", task.Type)
	}

	// Validate webhook URL if configured
	if task.Notifications != nil {
		if err := validateWebhookURL(task.Notifications.WebhookURL); err != nil {
			return err
		}
	}

	// Check for cyclic dependencies
	if s.hasCyclicDependencyLocked(task.ID, task.DependsOn) {
		return fmt.Errorf("task %s has cyclic dependencies", task.ID)
	}

	s.tasks[task.ID] = task
	if err := s.scheduleTask(task); err != nil {
		delete(s.tasks, task.ID)
		return fmt.Errorf("failed to schedule task: %w", err)
	}
	s.saveLocked()
	return nil
}

// UpdateTask updates an existing task's configuration.
func (s *Scheduler) UpdateTask(task *ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.tasks[task.ID]
	if !ok {
		return fmt.Errorf("task %s not found", task.ID)
	}

	// Sanitize on update too
	task.Name = sanitizeUTF8(task.Name)
	task.Payload = sanitizePayload(task.Payload)

	// Validate cron if changed
	if task.CronExpr != "" && task.CronExpr != existing.CronExpr {
		parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		_, err := parser.Parse(task.CronExpr)
		if err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
	}

	// Validate task type has a handler
	if task.Type != existing.Type {
		if _, hasHandler := s.handlers[task.Type]; !hasHandler {
			return fmt.Errorf("unknown task type: %s", task.Type)
		}
	}

	// Validate webhook URL if configured
	if task.Notifications != nil {
		if err := validateWebhookURL(task.Notifications.WebhookURL); err != nil {
			return err
		}
	}

	// Check for cyclic dependencies if dependencies changed
	if !s.equalStringSlices(existing.DependsOn, task.DependsOn) {
		if s.hasCyclicDependencyLocked(task.ID, task.DependsOn) {
			return fmt.Errorf("task %s has cyclic dependencies", task.ID)
		}
	}

	// Remove old cron entry
	if entryID, ok := s.cronIDs[task.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.cronIDs, task.ID)
	}

	// Update fields
	existing.Name = task.Name
	existing.Type = task.Type
	existing.Enabled = task.Enabled
	existing.CronExpr = task.CronExpr
	existing.Payload = task.Payload
	existing.TimeoutSec = task.TimeoutSec
	existing.MaxRetries = task.MaxRetries
	existing.Notifications = task.Notifications
	existing.DependsOn = task.DependsOn
	existing.ExecutionWindow = task.ExecutionWindow

	if existing.Enabled {
		if err := s.scheduleTask(existing); err != nil {
			return fmt.Errorf("failed to schedule task: %w", err)
		}
	}
	s.saveLocked()
	return nil
}

// DeleteTask removes a task from the scheduler.
func (s *Scheduler) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.cronIDs[id]; ok {
		s.cron.Remove(entryID)
		delete(s.cronIDs, id)
	}
	if _, ok := s.tasks[id]; !ok {
		return fmt.Errorf("task %s not found", id)
	}
	delete(s.tasks, id)
	return s.saveLocked()
}

// EnableTask enables a task and schedules it.
func (s *Scheduler) EnableTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}
	task.Enabled = true
	if err := s.scheduleTask(task); err != nil {
		task.Enabled = false
		return fmt.Errorf("failed to schedule task: %w", err)
	}
	return s.saveLocked()
}

// DisableTask disables a task and removes it from cron.
func (s *Scheduler) DisableTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}
	task.Enabled = false
	if entryID, ok := s.cronIDs[id]; ok {
		s.cron.Remove(entryID)
		delete(s.cronIDs, id)
	}
	return s.saveLocked()
}

// RunTaskNow executes a task immediately, regardless of its enabled state.
func (s *Scheduler) RunTaskNow(id string) error {
	s.mu.RLock()
	task, ok := s.tasks[id]
	if !ok {
		s.mu.RUnlock()
		return fmt.Errorf("task %s not found", id)
	}
	// Copy task data to avoid holding the lock during execution
	handler := s.handlers[task.Type]
	if handler == nil {
		s.mu.RUnlock()
		return fmt.Errorf("no handler registered for task type %s", task.Type)
	}
	timeoutSec := task.TimeoutSec
	retries := task.MaxRetries
	// Deep copy the task for execution
	taskCopy := *task
	if task.Payload != nil {
		taskCopy.Payload = make(map[string]interface{})
		for k, v := range task.Payload {
			taskCopy.Payload[k] = v
		}
	}
	s.mu.RUnlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("scheduler panic in RunTaskNow (%s): %v", taskCopy.ID, r)
			}
		}()
		s.executeTask(&taskCopy, handler, timeoutSec, retries)
	}()
	return nil
}

// ListTasks returns a copy of all scheduled tasks.
func (s *Scheduler) ListTasks() []*ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ScheduledTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		// Copy to avoid mutation
		cp := *t
		if t.Payload != nil {
			cp.Payload = make(map[string]interface{})
			for k, v := range t.Payload {
				cp.Payload[k] = v
			}
		}
		result = append(result, &cp)
	}
	return result
}

// GetTask returns a single task by ID.
func (s *Scheduler) GetTask(id string) (*ScheduledTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %s not found", id)
	}
	cp := *task
	return &cp, nil
}

// GetHistory returns execution history, most recent first.
func (s *Scheduler) GetHistory(limit int, taskType string, status string) []ExecutionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ExecutionRecord, 0, len(s.history))
	for i := len(s.history) - 1; i >= 0; i-- {
		r := s.history[i]
		if taskType != "" && r.TaskType != taskType {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		result = append(result, r)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// validateWebhookURL validates a webhook URL to prevent SSRF.
func validateWebhookURL(webhookURL string) error {
	return ValidateWebhookURLPublic(webhookURL)
}

// ValidateWebhookURLPublic validates a webhook URL to prevent SSRF.
// Requires https scheme and performs DNS resolution to verify the resolved IP.
func ValidateWebhookURLPublic(webhookURL string) error {
	if webhookURL == "" {
		return nil
	}
	_, err := urlguard.Check(webhookURL, urlguard.CheckOptions{
		AllowedSchemes: []string{"https"},
	})
	return err
}

// hasCyclicDependency checks for cyclic dependencies in a task's dependency chain.
func (s *Scheduler) hasCyclicDependencyLocked(taskID string, dependsOn []string) bool {
	visiting := make(map[string]bool) // nodes in current path
	visited := make(map[string]bool)  // fully explored nodes

	var dfs func(string) bool
	dfs = func(current string) bool {
		if visiting[current] {
			return true // cycle: back-edge to node in current path
		}
		if visited[current] {
			return false // already fully explored, no cycle from here
		}
		visiting[current] = true

		task, ok := s.tasks[current]
		if ok {
			for _, depID := range task.DependsOn {
				if dfs(depID) {
					return true
				}
			}
		}

		visiting[current] = false
		visited[current] = true
		return false
	}

	for _, depID := range dependsOn {
		if dfs(depID) {
			return true
		}
	}

	return false
}

// hasCyclicDependency checks for cyclic dependencies in a task's dependency chain.
func (s *Scheduler) hasCyclicDependency(taskID string, dependsOn []string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasCyclicDependencyLocked(taskID, dependsOn)
}

// equalStringSlices checks if two string slices are equal.
func (s *Scheduler) equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// normalizeCronExpr converts a 5-field cron (min hour dom month dow) to a
// 6-field expression (sec min hour dom month dow) by prepending "0".
// The scheduler is initialized with cron.WithSeconds() which requires 6 fields.
func normalizeCronExpr(expr string) string {
	fields := strings.Fields(expr)
	if len(fields) == 5 {
		return "0 " + expr
	}
	return expr
}

// scheduleTask registers a task in the cron scheduler. Returns error if the
// task cannot be added to cron (caller can then remove the task or retry).
func (s *Scheduler) scheduleTask(task *ScheduledTask) error {
	if !task.Enabled {
		return nil
	}
	handler := s.handlers[task.Type]
	if handler == nil {
		logger.Warnf("[scheduler] no handler registered for task type %s (id=%s)", task.Type, task.ID)
		return nil
	}

	schedule := func() {
		s.executeTask(task, handler, task.TimeoutSec, task.MaxRetries)
	}

	cronExpr := normalizeCronExpr(task.CronExpr)
	entryID, err := s.cron.AddFunc(cronExpr, schedule)
	if err != nil {
		logger.Errorf("[scheduler] failed to schedule task %s (cron=%q): %v", task.ID, task.CronExpr, err)
		return err
	}
	s.cronIDs[task.ID] = entryID

	// Calculate next run time
	next := s.cron.Entry(entryID).Next
	if !next.IsZero() {
		task.NextRunAt = &next
	}
	return nil
}

// executeTask runs a single task execution with optional retries.
func (s *Scheduler) executeTask(task *ScheduledTask, handler TaskHandler, timeoutSec int, maxRetries int) {
	now := time.Now()
	taskType := string(task.Type)
	var elapsed time.Duration

	// 检查依赖链
	if !s.areDependenciesMet(task) {
		logger.Infof("[scheduler] task %s (%s) skipped: dependencies not met", task.ID, task.Name)
		s.recordSkippedExecution(task, "dependencies_not_met", "dependency tasks not yet successful")
		return
	}

	// 检查执行窗口
	if task.ExecutionWindow != nil && !s.isWithinExecutionWindow(task.ExecutionWindow) {
		logger.Infof("[scheduler] task %s (%s) skipped: outside execution window", task.ID, task.Name)
		s.recordSkippedExecution(task, "outside_window", "current time outside execution window")
		return
	}

	record := ExecutionRecord{
		TaskID:     task.ID,
		TaskName:   task.Name,
		TaskType:   taskType,
		StartedAt:  now.Format(time.RFC3339),
		RetryCount: 0,
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			record.RetryCount = attempt
			metrics.IncSchedulerTaskRetry(taskType)
			time.Sleep(time.Duration(attempt*2) * time.Second) // simple backoff
		}

		ctx, cancel := context.WithTimeout(s.ctx, time.Duration(timeoutSec)*time.Second)
		var result string
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic in runner: %v", r)
				}
			}()
			result, err = handler.Execute(ctx, task.Payload)
		}()
		cancel()

		elapsed = time.Since(now)
		record.FinishedAt = time.Now().Format(time.RFC3339)
		record.DurationMs = elapsed.Milliseconds()

		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				record.Status = "timeout"
				record.Error = fmt.Sprintf("task timed out after %s", elapsed.Round(time.Millisecond))
			} else {
				record.Status = "failed"
				record.Error = err.Error()
			}
			continue
		}

		record.Status = "success"
		record.Result = result
		break
	}

	// Record metrics
	metrics.IncSchedulerTaskExecution(taskType, record.Status)
	metrics.ObserveSchedulerTaskExecutionDuration(taskType, elapsed)

	// Update task state
	s.mu.Lock()
	if t, ok := s.tasks[task.ID]; ok {
		t.LastRunAt = &now
		if next := s.getNextRunTime(task.ID); !next.IsZero() {
			t.NextRunAt = &next
		}
	}

	// Append history
	s.history = append(s.history, record)
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}
	s.mu.Unlock()

	s.updateMetrics()

	// 发送通知
	s.sendNotification(task, record)
}

func (s *Scheduler) updateMetrics() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	enabledCount := 0
	for _, t := range s.tasks {
		if t.Enabled {
			enabledCount++
		}
	}
	metrics.SetSchedulerTasksEnabled(enabledCount)
}

// areDependenciesMet checks if all dependency tasks have succeeded in their last execution.
func (s *Scheduler) areDependenciesMet(task *ScheduledTask) bool {
	if len(task.DependsOn) == 0 {
		return true
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, depID := range task.DependsOn {
		_, exists := s.tasks[depID]
		if !exists {
			logger.Warnf("[scheduler] dependency task %s not found for task %s", depID, task.ID)
			return false
		}

		// Find last execution record for this dependency
		lastRecord := s.findLastExecutionRecord(depID)
		if lastRecord == nil || lastRecord.Status != "success" {
			logger.Debugf("[scheduler] dependency task %s last status: %v (need success)", depID, lastRecord)
			return false
		}
	}
	return true
}

// findLastExecutionRecord finds the most recent execution record for a task.
func (s *Scheduler) findLastExecutionRecord(taskID string) *ExecutionRecord {
	for i := len(s.history) - 1; i >= 0; i-- {
		if s.history[i].TaskID == taskID {
			return &s.history[i]
		}
	}
	return nil
}

// isWithinExecutionWindow checks if the current time is within the allowed execution window.
func (s *Scheduler) isWithinExecutionWindow(window *ExecutionWindow) bool {
	now := time.Now()
	if window.Timezone != "" {
		loc, err := time.LoadLocation(window.Timezone)
		if err != nil {
			logger.Warnf("[scheduler] invalid timezone %q, using local time: %v", window.Timezone, err)
		} else {
			now = now.In(loc)
		}
	}

	// Check weekday constraint
	if len(window.Weekdays) > 0 {
		currentWeekday := int(now.Weekday())
		found := false
		for _, wd := range window.Weekdays {
			if wd == currentWeekday {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check hour constraint
	currentHour := now.Hour()
	if window.StartHour <= window.EndHour {
		// Normal range (e.g., 9-17 means 9am to 5pm)
		return currentHour >= window.StartHour && currentHour < window.EndHour
	}
	// Overnight range (e.g., 22-6 means 10pm to 6am next day)
	return currentHour >= window.StartHour || currentHour < window.EndHour
}

// recordSkippedExecution creates a record for a skipped task execution.
func (s *Scheduler) recordSkippedExecution(task *ScheduledTask, status string, reason string) {
	now := time.Now()
	record := ExecutionRecord{
		TaskID:     task.ID,
		TaskName:   task.Name,
		TaskType:   string(task.Type),
		StartedAt:  now.Format(time.RFC3339),
		FinishedAt: now.Format(time.RFC3339),
		DurationMs: 0,
		Status:     "skipped",
		Error:      reason,
	}

	s.mu.Lock()
	s.history = append(s.history, record)
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}
	s.mu.Unlock()

	metrics.IncSchedulerTaskExecution(string(task.Type), "skipped")
}

// sendNotification sends notifications based on task configuration and execution result.
func (s *Scheduler) sendNotification(task *ScheduledTask, record ExecutionRecord) {
	// 全局开关
	if s.notifyCfgProvider != nil {
		globalCfg := s.notifyCfgProvider()
		if globalCfg == nil || !globalCfg.Enabled {
			return
		}
	}

	if task.Notifications == nil {
		return
	}

	// 任务级总开关
	if !task.Notifications.Enabled {
		return
	}

	// 事件匹配
	shouldNotify := false
	switch record.Status {
	case "success":
		shouldNotify = task.Notifications.OnSuccess
	case "failed":
		shouldNotify = task.Notifications.OnFailure
	case "timeout":
		shouldNotify = task.Notifications.OnTimeout
	}
	if !shouldNotify {
		return
	}

	// 数据迁移：旧 Channels[] -> 新 ChannelIDs[]
	channelIDs := migrateChannelIDs(task.Notifications)
	if len(channelIDs) == 0 {
		return
	}

	msg := notify.TaskNotification{
		TaskID:    task.ID,
		TaskName:  sanitizeUTF8(task.Name),
		TaskType:  string(task.Type),
		Status:    record.Status,
		Result:    sanitizeUTF8(record.Result), // executor sanitizes, but double-check for old records
		Error:     sanitizeUTF8(record.Error),
		Duration:  float64(record.DurationMs),
		Timestamp: time.Now(),
		Payload:   sanitizePayload(task.Payload),
	}

	// 提取截图路径（用于飞书图片推送）
	msg.ImagePaths = extractImagePaths(record.Result)
	// 从通知文案中剥离系统路径，防止泄露
	msg.Result = redactImagePaths(msg.Result, msg.ImagePaths)

	timeout := s.notifyTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	for _, chID := range channelIDs {
		// 任务级 inline webhook 特殊处理
		if chID == "__task_inline_webhook__" && task.Notifications.WebhookURL != "" {
			s.mu.RLock()
			stopping := s.stopped || s.stopping
			s.mu.RUnlock()
			if stopping {
				continue
			}
			s.notifyWg.Add(1)
			go func(url string) {
				defer func() {
					s.notifyWg.Done()
					if r := recover(); r != nil {
						logger.Errorf("scheduler panic in inline webhook notification: %v", r)
					}
				}()
				ch, err := notify.NewGenericWebhookChannel("__inline__", url, nil, true, false)
				if err != nil {
					logger.Errorf("[scheduler] inline webhook URL blocked: %v", err)
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				if err := ch.Send(ctx, msg); err != nil {
					logger.Errorf("[scheduler] notify inline webhook failed: %v", err)
					metrics.IncSchedulerNotifyFail("webhook")
				} else {
					metrics.IncSchedulerNotifySuccess("webhook")
				}
			}(task.Notifications.WebhookURL)
			continue
		}

		if s.notifyRegistry == nil {
			continue
		}
		ch := s.notifyRegistry.Get(chID)
		if ch == nil {
			logger.Warnf("[scheduler] notify channel %q not registered, skipping", chID)
			continue
		}
		if !ch.IsEnabled() {
			continue
		}

		s.mu.RLock()
		stopping := s.stopped || s.stopping
		s.mu.RUnlock()
		if stopping {
			continue
		}
		s.notifyWg.Add(1)
		go func(ch notify.NotifyChannel) {
			defer func() {
				s.notifyWg.Done()
				if r := recover(); r != nil {
					logger.Errorf("scheduler panic in notify channel %s: %v", ch.ID(), r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if err := ch.Send(ctx, msg); err != nil {
				logger.Errorf("[scheduler] notify %s (%s) failed: %v", ch.ID(), ch.Type(), err)
				metrics.IncSchedulerNotifyFail(ch.Type())
			} else {
				metrics.IncSchedulerNotifySuccess(ch.Type())
			}
		}(ch)
	}
}

// extractImagePaths 从任务结果中提取截图文件路径
func extractImagePaths(result string) []string {
	if result == "" {
		return nil
	}

	var paths []string
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 匹配格式：✅ URL → filepath 或 ✅ filepath
		if strings.Contains(line, "→") {
			parts := strings.SplitN(line, "→", 2)
			if len(parts) == 2 {
				path := strings.TrimSpace(parts[1])
				if isImageFile(path) {
					paths = append(paths, path)
					continue // 已通过箭头格式提取，跳过后续独立匹配
				}
			}
		}

		// 匹配格式：✅ 截图保存: filepath
		if strings.Contains(line, "截图保存:") || strings.Contains(line, "截图目录:") {
			// 这是目录路径，不是文件路径
			continue
		}

		// 匹配直接的文件路径（以 screenshots/ 开头或包含 .png/.jpg）
		if isImageFile(line) {
			paths = append(paths, line)
		}
	}

	return paths
}

// isImageFile 检查路径是否是图片文件
func isImageFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".webp")
}

// redactImagePaths 从结果文本中剥离图片文件的完整路径，仅保留文件名。
// 防止飞书/Webhook 通知泄露服务器目录结构。
func redactImagePaths(result string, paths []string) string {
	if result == "" || len(paths) == 0 {
		return result
	}
	for _, p := range paths {
		result = strings.ReplaceAll(result, p, filepath.Base(p))
	}
	return result
}

// sanitizePayload creates a copy of payload with all string values passed through sanitizeUTF8.
func sanitizePayload(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return nil
	}
	out := make(map[string]interface{}, len(payload))
	for k, v := range payload {
		if s, ok := v.(string); ok {
			out[k] = sanitizeUTF8(s)
		} else {
			out[k] = v
		}
	}
	return out
}

// migrateChannelIDs 将旧 Channels[] 字段迁移到 ChannelIDs[]
func migrateChannelIDs(nc *NotificationConfig) []string {
	if len(nc.ChannelIDs) > 0 {
		return nc.ChannelIDs
	}

	var ids []string
	for _, name := range nc.Channels {
		switch name {
		case "log":
			ids = append(ids, "builtin-log")
		case "webhook":
			if nc.WebhookURL != "" {
				ids = append(ids, "__task_inline_webhook__")
			} else {
				logger.Warnf("[scheduler] task webhook channel without URL skipped")
			}
		case "email":
			logger.Warnf("[scheduler] email channel not supported, skipped")
		}
	}
	return ids
}

// sendWebhookNotification sends a webhook notification.
func safeWebhookClient() *http.Client {
	return urlguard.SafeHTTPClient(urlguard.CheckOptions{}, 30*time.Second)
}

func (s *Scheduler) sendWebhookNotification(webhookURL string, payload map[string]interface{}) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("scheduler panic in sendWebhookNotification: %v", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		jsonData, err := json.Marshal(payload)
		if err != nil {
			logger.Errorf("[scheduler] failed to marshal webhook payload: %v", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
		if err != nil {
			logger.Errorf("[scheduler] failed to create webhook request: %v", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "UniMap-Scheduler/1.0")

		client := safeWebhookClient()
		resp, err := client.Do(req)
		if err != nil {
			logger.Errorf("[scheduler] failed to send webhook to %s: %v", webhookURL, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			logger.Warnf("[scheduler] webhook to %s returned non-success status: %d", webhookURL, resp.StatusCode)
		}
	}()
}

func (s *Scheduler) getNextRunTime(taskID string) time.Time {
	if entryID, ok := s.cronIDs[taskID]; ok {
		return s.cron.Entry(entryID).Next
	}
	return time.Time{}
}

// saveAsync persists data to disk in a background goroutine.
func (s *Scheduler) saveAsync() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("scheduler panic in saveAsync: %v", r)
			}
		}()
		if err := s.Save(); err != nil {
			logger.Errorf("[scheduler] saveAsync failed: %v", err)
		}
	}()
}

// Stop gracefully stops the scheduler and all background goroutines.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopping = true
	s.stopped = true
	close(s.stopCh)
	s.mu.Unlock()

	// 取消所有进行中的任务（context 派生自 s.ctx 的任务将收到取消信号）
	s.cancel()

	// Stop cron and wait for it to finish
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()

	// Wait for notification goroutines to finish
	s.notifyWg.Wait()
}

// generateID creates a short unique ID using a monotonic counter.
func (s *Scheduler) generateID() string {
	return uuid.New().String()
}

// TaskExecutionStats holds statistical analysis of task execution history.
type TaskExecutionStats struct {
	TaskID        string  `json:"task_id"`
	TaskName      string  `json:"task_name"`
	TaskType      string  `json:"task_type"`
	TotalRuns     int     `json:"total_runs"`
	SuccessCount  int     `json:"success_count"`
	FailedCount   int     `json:"failed_count"`
	TimeoutCount  int     `json:"timeout_count"`
	SkippedCount  int     `json:"skipped_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	MaxDurationMs int64   `json:"max_duration_ms"`
	MinDurationMs int64   `json:"min_duration_ms"`
	P50DurationMs int64   `json:"p50_duration_ms"`
	P95DurationMs int64   `json:"p95_duration_ms"`
	TotalRetries  int     `json:"total_retries"`
	LastSuccessAt string  `json:"last_success_at,omitempty"`
	LastFailureAt string  `json:"last_failure_at,omitempty"`
}

// GetTaskExecutionStats analyzes execution history for a specific task.
func (s *Scheduler) GetTaskExecutionStats(taskID string) *TaskExecutionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var task *ScheduledTask
	if t, ok := s.tasks[taskID]; ok {
		task = t
	}

	stats := &TaskExecutionStats{
		TaskID:        taskID,
		MinDurationMs: 0,
	}
	if task != nil {
		stats.TaskName = task.Name
		stats.TaskType = string(task.Type)
	}

	var durations []int64
	for _, record := range s.history {
		if record.TaskID != taskID {
			continue
		}

		stats.TotalRuns++
		durations = append(durations, record.DurationMs)

		switch record.Status {
		case "success":
			stats.SuccessCount++
			stats.LastSuccessAt = record.FinishedAt
		case "failed":
			stats.FailedCount++
			stats.LastFailureAt = record.FinishedAt
		case "timeout":
			stats.TimeoutCount++
			stats.LastFailureAt = record.FinishedAt
		case "skipped":
			stats.SkippedCount++
		}

		stats.TotalRetries += record.RetryCount

		if record.DurationMs > stats.MaxDurationMs {
			stats.MaxDurationMs = record.DurationMs
		}
		if stats.MinDurationMs < 0 || record.DurationMs < stats.MinDurationMs {
			stats.MinDurationMs = record.DurationMs
		}
	}

	if stats.TotalRuns > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalRuns) * 100

		// Calculate average duration
		var totalDuration int64
		for _, d := range durations {
			totalDuration += d
		}
		stats.AvgDurationMs = float64(totalDuration) / float64(len(durations))

		// Sort durations for percentile calculation
		sortInt64(durations)
		if len(durations) > 0 {
			stats.MinDurationMs = durations[0]
			stats.MaxDurationMs = durations[len(durations)-1]
			stats.P50DurationMs = durations[len(durations)*50/100]
			stats.P95DurationMs = durations[len(durations)*95/100]
		}
	}

	return stats
}

// GetAllTasksStats returns execution stats for all tasks.
func (s *Scheduler) GetAllTasksStats() []*TaskExecutionStats {
	s.mu.RLock()
	taskIDs := make([]string, 0, len(s.tasks))
	for id := range s.tasks {
		taskIDs = append(taskIDs, id)
	}
	s.mu.RUnlock()

	stats := make([]*TaskExecutionStats, 0, len(taskIDs))
	for _, id := range taskIDs {
		stats = append(stats, s.GetTaskExecutionStats(id))
	}
	return stats
}

// GetRecentExecutions returns the most recent execution records.
func (s *Scheduler) GetRecentExecutions(limit int) []ExecutionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}

	start := len(s.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]ExecutionRecord, limit)
	copy(result, s.history[start:])
	return result
}

// sortInt64 sorts a slice of int64 in ascending order.
func sortInt64(s []int64) {
	sort.Slice(s, func(i, j int) bool {
		return s[i] < s[j]
	})
}
