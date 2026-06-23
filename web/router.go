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
	r.registerPageRoutes()
	r.registerAuthRoutes()
	r.registerQueryRoutes()
	r.registerCookieRoutes()
	r.registerCDPRoutes()
	r.registerScreenshotRoutes()
	r.registerImportRoutes()
	r.registerNodeRoutes()
	r.registerSchedulerRoutes()
	r.registerNotificationRoutes()
	r.registerTamperRoutes()
	r.registerMiscRoutes()
	return r.buildMux()
}

func (r *Router) registerPageRoutes() {
	s := r.server
	r.addRoute("index", "GET", "/", s.handleIndex, false)
	r.addRoute("results", "GET", "/results", s.handleResults, false)
	r.addRoute("quota", "GET", "/quota", s.handleQuota, false)
	r.addRoute("account", "GET", "/account", s.handleAccountPage, false)
	r.addRoute("batch-screenshot", "GET", "/batch-screenshot", http.RedirectHandler("/monitor", http.StatusMovedPermanently).ServeHTTP, false)
	r.addRoute("monitor", "GET", "/monitor", s.handleMonitorPage, false)
	r.addRoute("scheduler", "GET", "/scheduler", s.handleSchedulerPage, false)
	r.addRoute("port-scan", "GET", "/port-scan", s.handlePortScanPage, false)
	r.addRoute("icp-page", "GET", "/icp", s.handleICPPage, false)
	r.addRoute("settings-page", "GET", "/settings", s.handleSettingsPage, false)
}

func (r *Router) registerAuthRoutes() {
	s := r.server
	r.addRoute("login-page", "GET", "/login", s.handleLoginPage, false)
	r.addAPIRoute("login-api", "POST", "/api/login", s.handleLoginAPI, true)
	r.addAPIRoute("logout-api", "POST", "/api/logout", s.handleLogoutAPI, true)
}

func (r *Router) registerQueryRoutes() {
	s := r.server
	r.addRoute("health", "GET", "/health", s.handleHealth, false)
	r.addRoute("health-ready", "GET", "/health/ready", s.handleHealthReady, false)
	r.addRoute("health-live", "GET", "/health/live", s.handleHealthLive, false)
	r.addRoute("metrics", "GET", "/metrics", s.handleMetrics, false)
	r.addRoute("query", "GET", "/query", s.handleQuery, true)
	r.addAPIRoute("api-query", "POST", "/api/query", s.handleAPIQuery, true)
	r.addAPIRoute("query-status", "GET", "/api/query/status", s.handleQueryStatus, true)
	r.addAPIRoute("websocket", "GET", "/api/ws", s.handleWebSocket, false)
}

func (r *Router) registerCookieRoutes() {
	s := r.server
	r.addAPIRoute("cookies-save", "POST", "/api/cookies", s.handleSaveCookies, true)
	r.addAPIRoute("cookies-verify", "POST", "/api/cookies/verify", s.handleVerifyCookies, true)
	r.addAPIRoute("cookies-import", "POST", "/api/cookies/import", s.handleImportCookieJSON, true)
	r.addAPIRoute("cookies-login-status", "GET", "/api/cookies/login-status", s.handleCookieLoginStatus, true)
}

func (r *Router) registerCDPRoutes() {
	s := r.server
	r.addAPIRoute("cdp-status", "GET", "/api/cdp/status", s.handleCDPStatus, true)
	r.addAPIRoute("cdp-connect", "POST", "/api/cdp/connect", s.handleCDPConnect, true)
}

func (r *Router) registerScreenshotRoutes() {
	s := r.server
	r.addAPIRoute("screenshot", "POST", "/api/screenshot", s.handleScreenshot, true)
	r.addAPIRoute("screenshot-engine", "GET", "/api/screenshot/search-engine", s.handleSearchEngineScreenshot, true)
	r.addAPIRoute("screenshot-target", "POST", "/api/screenshot/target", s.handleTargetScreenshot, true)
	r.addAPIRoute("screenshot-batch", "POST", "/api/screenshot/batch", s.handleBatchScreenshot, true)
	r.addAPIRoute("screenshot-batch-urls", "POST", "/api/screenshot/batch-urls", s.handleBatchURLsScreenshot, true)
	r.addAPIRoute("screenshot-batch-progress", "GET", "/api/screenshot/batch/progress", s.handleBatchScreenshotProgress, true)
	r.addAPIRoute("screenshot-batches", "GET", "/api/screenshot/batches", s.handleScreenshotBatches, true)
	r.addAPIRoute("screenshot-batch-files", "GET", "/api/screenshot/batches/files", s.handleScreenshotBatchFiles, true)
	r.addAPIRoute("screenshot-batch-delete", "DELETE", "/api/screenshot/batches/delete", s.handleScreenshotBatchDelete, true)
	r.addAPIRoute("screenshot-file-delete", "DELETE", "/api/screenshot/file/delete", s.handleScreenshotFileDelete, true)
	r.addRoute("screenshot-file", "GET", "/screenshots/", s.handleScreenshotFile, false)
	r.addAPIRoute("screenshot-bridge-health", "GET", "/api/screenshot/bridge/health", s.handleScreenshotBridgeHealth, true)
	r.addAPIRoute("screenshot-bridge-status", "GET", "/api/screenshot/bridge/status", s.handleScreenshotBridgeStatus, true)
	r.addAPIRoute("screenshot-bridge-pair", "POST", "/api/screenshot/bridge/pair", s.handleScreenshotBridgePair, true)
	r.addAPIRoute("screenshot-bridge-token-rotate", "POST", "/api/screenshot/bridge/token/rotate", s.handleScreenshotBridgeRotateToken, true)
	r.addAPIRoute("screenshot-bridge-task-next", "GET", "/api/screenshot/bridge/tasks/next", s.handleScreenshotBridgeTaskNext, true)
	r.addAPIRoute("screenshot-bridge-mock-result", "POST", "/api/screenshot/bridge/mock/result", s.handleScreenshotBridgeMockResult, true)
	r.addAPIRoute("screenshot-router-status", "GET", "/api/screenshot/router/status", s.handleScreenshotRouterStatus, true)
	r.addAPIRoute("screenshot-set-mode", "POST", "/api/screenshot/set-mode", s.handleSetScreenshotMode, true)
}

func (r *Router) registerImportRoutes() {
	s := r.server
	r.addAPIRoute("import-urls", "POST", "/api/import/urls", s.handleImportURLs, true)
	r.addAPIRoute("url-reachability", "POST", "/api/url/reachability", s.handleURLReachability, true)
	r.addAPIRoute("url-port-scan", "POST", "/api/url/port-scan", s.handleURLPortScan, true)
}

func (r *Router) registerNodeRoutes() {
	s := r.server
	r.addAPIRoute("node-register", "POST", "/api/nodes/register", s.handleNodeRegister, true)
	r.addAPIRoute("node-heartbeat", "POST", "/api/nodes/heartbeat", s.handleNodeHeartbeat, true)
	r.addAPIRoute("node-status", "GET", "/api/nodes/status", s.handleNodeStatus, true)
	r.addAPIRoute("node-get", "GET", "/api/nodes/get", s.handleNodeGet, true)
	r.addAPIRoute("node-deregister", "DELETE", "/api/nodes/deregister", s.handleNodeDeregister, true)
	r.addAPIRoute("node-network-profile", "GET", "/api/nodes/network/profile", s.handleNodeNetworkProfile, true)
	r.addAPIRoute("node-task-enqueue", "POST", "/api/nodes/task/enqueue", s.handleNodeTaskEnqueue, true)
	r.addAPIRoute("node-task-claim", "POST", "/api/nodes/task/claim", s.handleNodeTaskClaim, true)
	r.addAPIRoute("node-task-result", "POST", "/api/nodes/task/result", s.handleNodeTaskResult, true)
	r.addAPIRoute("node-task-status", "GET", "/api/nodes/task/status", s.handleNodeTaskStatus, true)
	r.addAPIRoute("node-task-get", "GET", "/api/nodes/task/get", s.handleNodeTaskGet, true)
	r.addAPIRoute("node-task-delete", "DELETE", "/api/nodes/task/delete", s.handleNodeTaskDelete, true)
}

func (r *Router) registerSchedulerRoutes() {
	s := r.server
	r.addAPIRoute("scheduler-tasks-list", "GET", "/api/scheduler/tasks", s.handleListTasks, true)
	r.addAPIRoute("scheduler-task-get", "GET", "/api/scheduler/tasks/get", s.handleGetTask, true)
	r.addAPIRoute("scheduler-task-create", "POST", "/api/scheduler/tasks/create", s.handleCreateTask, true)
	r.addAPIRoute("scheduler-task-update", "POST", "/api/scheduler/tasks/update", s.handleUpdateTask, true)
	r.addAPIRoute("scheduler-task-delete", "POST", "/api/scheduler/tasks/delete", s.handleDeleteTask, true)
	r.addAPIRoute("scheduler-task-run", "POST", "/api/scheduler/tasks/run", s.handleRunTaskNow, true)
	r.addAPIRoute("scheduler-task-enable", "POST", "/api/scheduler/tasks/enable", s.handleEnableTask, true)
	r.addAPIRoute("scheduler-task-disable", "POST", "/api/scheduler/tasks/disable", s.handleDisableTask, true)
	r.addAPIRoute("scheduler-history", "GET", "/api/scheduler/history", s.handleTaskHistory, true)
}

func (r *Router) registerNotificationRoutes() {
	s := r.server
	r.addAPIRoute("notify-channels", "GET", "/api/notifications/channels", s.handleNotificationChannels, true)
	r.addAPIRoute("notify-channels-save", "POST", "/api/notifications/channels", s.handleNotifyChannelSave, true)
	r.addAPIRoute("notify-channels-delete", "DELETE", "/api/notifications/channels", s.handleNotifyChannelDelete, true)
	r.addAPIRoute("notify-channels-test", "POST", "/api/notifications/channels/test", s.handleNotifyChannelTest, true)
	r.addAPIRoute("notify-reload", "POST", "/api/notifications/reload", s.handleNotifyReload, true)
}

func (r *Router) registerTamperRoutes() {
	s := r.server
	r.addAPIRoute("tamper-check", "POST", "/api/tamper/check", s.handleTamperCheck, true)
	r.addAPIRoute("tamper-baseline", "POST", "/api/tamper/baseline", s.handleTamperBaseline, true)
	r.addAPIRoute("tamper-baseline-list", "GET", "/api/tamper/baseline/list", s.handleTamperBaselineList, true)
	r.addAPIRoute("tamper-baseline-delete", "DELETE", "/api/tamper/baseline/delete", s.handleTamperBaselineDelete, true)
	r.addAPIRoute("tamper-history", "GET", "/api/tamper/history", s.handleTamperHistory, true)
	r.addAPIRoute("tamper-history-delete", "DELETE", "/api/tamper/history/delete", s.handleTamperHistoryDelete, true)
}

func (r *Router) registerMiscRoutes() {
	s := r.server
	r.addAPIRoute("backup-create", "POST", "/api/backup/create", s.handleCreateBackup, true)
	r.addAPIRoute("backup-list", "GET", "/api/backup/list", s.handleListBackups, true)
	r.addAPIRoute("account-change-password", "POST", "/api/account/change-password", s.handleChangePassword, true)
	r.addAPIRoute("account-admin-token", "GET", "/api/account/admin-token", s.handleGetAdminToken, true)
	r.addAPIRoute("user-register", "POST", "/api/users/register", s.handleRegister, true)
	r.addAPIRoute("user-list", "GET", "/api/users", s.handleListUsers, true)
	r.addAPIRoute("user-get", "GET", "/api/users/{id}", s.handleGetUser, true)
	r.addAPIRoute("user-update", "PUT", "/api/users/{id}", s.handleUpdateUser, true)
	r.addAPIRoute("user-delete", "DELETE", "/api/users/{id}", s.handleDeleteUser, true)
	r.addAPIRoute("user-change-password", "POST", "/api/users/{id}/password", s.handleChangeUserPassword, true)
	r.addAPIRoute("icp-health", "GET", "/api/icp/health", s.handleICPHealth, true)
	r.addAPIRoute("icp-query", "GET", "/api/icp/query", s.handleICPQuery, true)
	r.addAPIRoute("icp-history", "GET", "/api/icp/history", s.handleICPHistory, true)
	r.addAPIRoute("icp-history-results", "GET", "/api/icp/history/results", s.handleICPHistoryResults, true)
	r.addAPIRoute("icp-compare", "GET", "/api/icp/compare", s.handleICPCompare, true)
	r.addAPIRoute("config-get", "GET", "/api/config", s.handleGetConfig, true)
	r.addAPIRoute("config-save", "POST", "/api/config", s.handleSaveConfig, true)
	r.addAPIRoute("history-save", "POST", "/api/history/save", s.handleHistorySave, true)
	r.addAPIRoute("history-list", "GET", "/api/history", s.handleHistoryListOrClear, true)
	r.addAPIRoute("history-clear", "DELETE", "/api/history", s.handleHistoryListOrClear, true)
	r.addAPIRoute("history-get", "GET", "/api/history/{id}", s.handleHistoryGetOrDelete, true)
	r.addAPIRoute("history-delete", "DELETE", "/api/history/{id}", s.handleHistoryGetOrDelete, true)
}

func (r *Router) buildMux() http.Handler {
	mux := http.NewServeMux()
	for _, route := range r.routes {
		handler := http.Handler(route.Handler)
		if route.RateLimited { handler = rateLimitMiddleware(handler) }
		handler = r.server.apiAuth.OptionalAPIKey()(handler)
		mux.Handle(route.Method+" "+route.Pattern, handler)
	}
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
