package web

import (
	"net/http"
	"path/filepath"
	"strings"
)

// Route 定义路由
type Route struct {
	Name        string
	Method      string
	Pattern     string
	Handler     http.HandlerFunc
	RateLimited bool // 是否需要限流
}

// Router 路由管理器
type Router struct {
	routes []Route
	server *Server
}

// NewRouter 创建路由管理器
func NewRouter(s *Server) *Router {
	return &Router{
		routes: make([]Route, 0),
		server: s,
	}
}

// RegisterRoutes 注册所有路由
func (r *Router) RegisterRoutes() http.Handler {
	// 页面路由
	r.addRoute("index", "GET", "/", r.server.handleIndex, false)
	r.addRoute("results", "GET", "/results", r.server.handleResults, false)
	r.addRoute("quota", "GET", "/quota", r.server.handleQuota, false)
	r.addRoute("account", "GET", "/account", r.server.handleAccountPage, false)
	r.addRoute("batch-screenshot", "GET", "/batch-screenshot", http.RedirectHandler("/monitor", http.StatusMovedPermanently).ServeHTTP, false)
	r.addRoute("monitor", "GET", "/monitor", r.server.handleMonitorPage, false)
	r.addRoute("scheduler", "GET", "/scheduler", r.server.handleSchedulerPage, false)

	// ICP 备案查询页与设置页
	r.addRoute("icp-page", "GET", "/icp", r.server.handleICPPage, false)
	r.addRoute("settings-page", "GET", "/settings", r.server.handleSettingsPage, false)

	// 登录/登出路由
	r.addRoute("login-page", "GET", "/login", r.server.handleLoginPage, false)
	r.addAPIRoute("login-api", "POST", "/api/login", r.server.handleLoginAPI, true)
	r.addAPIRoute("logout-api", "POST", "/api/logout", r.server.handleLogoutAPI, false)

	// API 路由 - 查询相关（限流）
	r.addRoute("health", "GET", "/health", r.server.handleHealth, false)
	r.addRoute("health-ready", "GET", "/health/ready", r.server.handleHealthReady, false)
	r.addRoute("health-live", "GET", "/health/live", r.server.handleHealthLive, false)
	r.addRoute("metrics", "GET", "/metrics", r.server.handleMetrics, false)
	r.addRoute("query", "GET", "/query", r.server.handleQuery, true)
	r.addAPIRoute("api-query", "POST", "/api/query", r.server.handleAPIQuery, true)
	r.addAPIRoute("query-status", "GET", "/api/query/status", r.server.handleQueryStatus, true)

	// API 路由 - Cookie 管理
	r.addAPIRoute("cookies-save", "POST", "/api/cookies", r.server.handleSaveCookies, false)
	r.addAPIRoute("cookies-verify", "POST", "/api/cookies/verify", r.server.handleVerifyCookies, false)
	r.addAPIRoute("cookies-import", "POST", "/api/cookies/import", r.server.handleImportCookieJSON, false)
	r.addAPIRoute("cookies-login-status", "GET", "/api/cookies/login-status", r.server.handleCookieLoginStatus, false)

	// API 路由 - CDP
	r.addAPIRoute("cdp-status", "GET", "/api/cdp/status", r.server.handleCDPStatus, false)
	r.addAPIRoute("cdp-connect", "POST", "/api/cdp/connect", r.server.handleCDPConnect, false)

	// API 路由 - WebSocket
	r.addAPIRoute("websocket", "GET", "/api/ws", r.server.handleWebSocket, false)

	// API 路由 - 截图（限流）
	r.addAPIRoute("screenshot", "POST", "/api/screenshot", r.server.handleScreenshot, true)
	r.addAPIRoute("screenshot-engine", "GET", "/api/screenshot/search-engine", r.server.handleSearchEngineScreenshot, true)
	r.addAPIRoute("screenshot-target", "POST", "/api/screenshot/target", r.server.handleTargetScreenshot, true)
	r.addAPIRoute("screenshot-batch", "POST", "/api/screenshot/batch", r.server.handleBatchScreenshot, true)
	r.addAPIRoute("screenshot-batch-urls", "POST", "/api/screenshot/batch-urls", r.server.handleBatchURLsScreenshot, true)
	r.addAPIRoute("screenshot-batches", "GET", "/api/screenshot/batches", r.server.handleScreenshotBatches, false)
	r.addAPIRoute("screenshot-batch-files", "GET", "/api/screenshot/batches/files", r.server.handleScreenshotBatchFiles, false)
	r.addAPIRoute("screenshot-batch-delete", "DELETE", "/api/screenshot/batches/delete", r.server.handleScreenshotBatchDelete, false)
	r.addAPIRoute("screenshot-file-delete", "DELETE", "/api/screenshot/file/delete", r.server.handleScreenshotFileDelete, false)
	r.addRoute("screenshot-file", "GET", "/screenshots/", r.server.handleScreenshotFile, false)
	r.addAPIRoute("screenshot-bridge-health", "GET", "/api/screenshot/bridge/health", r.server.handleScreenshotBridgeHealth, false)
	r.addAPIRoute("screenshot-bridge-status", "GET", "/api/screenshot/bridge/status", r.server.handleScreenshotBridgeStatus, false)
	r.addAPIRoute("screenshot-bridge-pair", "POST", "/api/screenshot/bridge/pair", r.server.handleScreenshotBridgePair, false)
	r.addAPIRoute("screenshot-bridge-token-rotate", "POST", "/api/screenshot/bridge/token/rotate", r.server.handleScreenshotBridgeRotateToken, false)
	r.addAPIRoute("screenshot-bridge-task-next", "GET", "/api/screenshot/bridge/tasks/next", r.server.handleScreenshotBridgeTaskNext, false)
	r.addAPIRoute("screenshot-bridge-mock-result", "POST", "/api/screenshot/bridge/mock/result", r.server.handleScreenshotBridgeMockResult, false)
	r.addAPIRoute("screenshot-router-status", "GET", "/api/screenshot/router/status", r.server.handleScreenshotRouterStatus, false)
	r.addAPIRoute("screenshot-set-mode", "POST", "/api/screenshot/set-mode", r.server.handleSetScreenshotMode, false)

	// API 路由 - 导入（限流）
	r.addAPIRoute("import-urls", "POST", "/api/import/urls", r.server.handleImportURLs, true)
	r.addAPIRoute("url-reachability", "POST", "/api/url/reachability", r.server.handleURLReachability, true)
	r.addAPIRoute("url-port-scan", "POST", "/api/url/port-scan", r.server.handleURLPortScan, true)

	// API 路由 - Day15 分布式节点（首版）
	r.addAPIRoute("node-register", "POST", "/api/nodes/register", r.server.handleNodeRegister, false)
	r.addAPIRoute("node-heartbeat", "POST", "/api/nodes/heartbeat", r.server.handleNodeHeartbeat, false)
	r.addAPIRoute("node-status", "GET", "/api/nodes/status", r.server.handleNodeStatus, false)
	r.addAPIRoute("node-get", "GET", "/api/nodes/get", r.server.handleNodeGet, false)
	r.addAPIRoute("node-deregister", "DELETE", "/api/nodes/deregister", r.server.handleNodeDeregister, false)
	r.addAPIRoute("node-network-profile", "GET", "/api/nodes/network/profile", r.server.handleNodeNetworkProfile, false)
	r.addAPIRoute("node-task-enqueue", "POST", "/api/nodes/task/enqueue", r.server.handleNodeTaskEnqueue, false)
	r.addAPIRoute("node-task-claim", "POST", "/api/nodes/task/claim", r.server.handleNodeTaskClaim, false)
	r.addAPIRoute("node-task-result", "POST", "/api/nodes/task/result", r.server.handleNodeTaskResult, false)
	r.addAPIRoute("node-task-status", "GET", "/api/nodes/task/status", r.server.handleNodeTaskStatus, false)
	r.addAPIRoute("node-task-get", "GET", "/api/nodes/task/get", r.server.handleNodeTaskGet, false)
	r.addAPIRoute("node-task-delete", "DELETE", "/api/nodes/task/delete", r.server.handleNodeTaskDelete, false)

	// API 路由 - 定时任务
	r.addAPIRoute("scheduler-tasks-list", "GET", "/api/scheduler/tasks", r.server.handleListTasks, false)
	r.addAPIRoute("scheduler-task-get", "GET", "/api/scheduler/tasks/get", r.server.handleGetTask, false)
	r.addAPIRoute("scheduler-task-create", "POST", "/api/scheduler/tasks/create", r.server.handleCreateTask, false)
	r.addAPIRoute("scheduler-task-update", "POST", "/api/scheduler/tasks/update", r.server.handleUpdateTask, false)
	r.addAPIRoute("scheduler-task-delete", "POST", "/api/scheduler/tasks/delete", r.server.handleDeleteTask, false)
	r.addAPIRoute("scheduler-task-run", "POST", "/api/scheduler/tasks/run", r.server.handleRunTaskNow, false)
	r.addAPIRoute("scheduler-task-enable", "POST", "/api/scheduler/tasks/enable", r.server.handleEnableTask, false)
	r.addAPIRoute("scheduler-task-disable", "POST", "/api/scheduler/tasks/disable", r.server.handleDisableTask, false)
	r.addAPIRoute("scheduler-history", "GET", "/api/scheduler/history", r.server.handleTaskHistory, false)

	// API 路由 - 通知系统
	r.addAPIRoute("notify-channels", "GET", "/api/notifications/channels", r.server.handleNotificationChannels, false)
	r.addAPIRoute("notify-channels-save", "POST", "/api/notifications/channels", r.server.handleNotifyChannelSave, false)
	r.addAPIRoute("notify-channels-delete", "DELETE", "/api/notifications/channels", r.server.handleNotifyChannelDelete, false)
	r.addAPIRoute("notify-channels-test", "POST", "/api/notifications/channels/test", r.server.handleNotifyChannelTest, false)
	r.addAPIRoute("notify-reload", "POST", "/api/notifications/reload", r.server.handleNotifyReload, false)

	// API 路由 - 篡改检测（限流）
	r.addAPIRoute("tamper-check", "POST", "/api/tamper/check", r.server.handleTamperCheck, true)
	r.addAPIRoute("tamper-baseline", "POST", "/api/tamper/baseline", r.server.handleTamperBaseline, true)
	r.addAPIRoute("tamper-baseline-list", "GET", "/api/tamper/baseline/list", r.server.handleTamperBaselineList, false)
	r.addAPIRoute("tamper-baseline-delete", "DELETE", "/api/tamper/baseline/delete", r.server.handleTamperBaselineDelete, false)
	r.addAPIRoute("tamper-history", "GET", "/api/tamper/history", r.server.handleTamperHistory, false)
	r.addAPIRoute("tamper-history-delete", "DELETE", "/api/tamper/history/delete", r.server.handleTamperHistoryDelete, false)

	// API 路由 - 数据备份
	r.addAPIRoute("backup-create", "POST", "/api/backup/create", r.server.handleCreateBackup, false)
	r.addAPIRoute("backup-list", "GET", "/api/backup/list", r.server.handleListBackups, false)

	// API 路由 - 账号管理
	r.addAPIRoute("account-change-password", "POST", "/api/account/change-password", r.server.handleChangePassword, false)
	r.addAPIRoute("account-admin-token", "GET", "/api/account/admin-token", r.server.handleGetAdminToken, false)

	// API 路由 - 用户管理
	r.addAPIRoute("user-register", "POST", "/api/users/register", r.server.handleRegister, true)
	r.addAPIRoute("user-list", "GET", "/api/users", r.server.handleListUsers, false)
	r.addAPIRoute("user-get", "GET", "/api/users/{id}", r.server.handleGetUser, false)
	r.addAPIRoute("user-update", "PUT", "/api/users/{id}", r.server.handleUpdateUser, false)
	r.addAPIRoute("user-delete", "DELETE", "/api/users/{id}", r.server.handleDeleteUser, false)
	r.addAPIRoute("user-change-password", "POST", "/api/users/{id}/password", r.server.handleChangeUserPassword, false)

	// API 路由 - ICP 备案查询
	r.addAPIRoute("icp-health", "GET", "/api/icp/health", r.server.handleICPHealth, true)
	r.addAPIRoute("icp-query", "GET", "/api/icp/query", r.server.handleICPQuery, true)

	// API 路由 - ICP 历史与对比
	r.addAPIRoute("icp-history", "GET", "/api/icp/history", r.server.handleICPHistory, true)
	r.addAPIRoute("icp-history-results", "GET", "/api/icp/history/results", r.server.handleICPHistoryResults, true)
	r.addAPIRoute("icp-compare", "GET", "/api/icp/compare", r.server.handleICPCompare, true)

	// API 路由 - 配置读写
	r.addAPIRoute("config-get", "GET", "/api/config", r.server.handleGetConfig, false)
	r.addAPIRoute("config-save", "POST", "/api/config", r.server.handleSaveConfig, false)

	// 创建 mux
	mux := http.NewServeMux()

	// 注册路由
	for _, route := range r.routes {
		handler := http.Handler(route.Handler)

		// 如果需要限流，包装限流中间件
		if route.RateLimited {
			handler = rateLimitMiddleware(handler)
		}

		// API key auth — optional, enriches context with key info if valid key provided
		handler = r.server.apiAuth.OptionalAPIKey()(handler)

		// Go 1.22+ ServeMux requires method-prefixed patterns for same-path routes
		mux.Handle(route.Method+" "+route.Pattern, handler)
	}

	// 静态文件服务
	staticDir := filepath.Join(r.server.webRoot, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	return mux
}

// addRoute 添加路由
func (r *Router) addRoute(name, method, pattern string, handler http.HandlerFunc, rateLimited bool) {
	r.routes = append(r.routes, Route{
		Name:        name,
		Method:      method,
		Pattern:     pattern,
		Handler:     handler,
		RateLimited: rateLimited,
	})
}

// addAPIRoute registers an API route under /api/v1/... path.
// Legacy /api/... shim removed 2026-06-09 — all consumers have migrated to /api/v1/.
func (r *Router) addAPIRoute(name, method, apiPath string, handler http.HandlerFunc, rateLimited bool) {
	v1Path := "/api/v1" + strings.TrimPrefix(apiPath, "/api")
	r.addRoute(name, method, v1Path, handler, rateLimited)
}

// GetRoutes 获取所有路由（用于调试/文档）
func (r *Router) GetRoutes() []Route {
	return r.routes
}
