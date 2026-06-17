package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/service"
)

// stableEngines 前端展示的稳定引擎列表。新引擎（censys/daydaymap/binaryedge/onyphe/greynoise）代码保留，API Key 验证通过后补充到此列表即可启用。
var stableEngines = map[string]bool{
	"fofa": true, "hunter": true, "zoomeye": true, "quake": true, "shodan": true,
}

func filterStableEngines(engines []string) []string {
	out := make([]string, 0, len(engines))
	for _, e := range engines {
		if stableEngines[strings.ToLower(e)] {
			out = append(out, e)
		}
	}
	return out
}

func (s *Server) runBrowserQueryAsync(ctx context.Context, query string, engines []string, enabled bool, action string, queryID string, progress func(done, total int, engine string, err error)) <-chan browserQueryOutcome {
	autoCaptureEnabled := false
	if s.config != nil {
		autoCaptureEnabled = s.config.Screenshot.AutoCapture.Enabled && s.config.Screenshot.AutoCapture.CaptureSearchResults
	}

	return s.queryApp.RunBrowserQueryAsync(
		ctx,
		query,
		engines,
		enabled,
		action,
		queryID,
		autoCaptureEnabled,
		s.screenshotApp,
		s.screenshotMgr,
		s.screenshotPathToPreviewURL,
		s.browserQueryProvider(),
		progress,
	)
}

func (s *Server) browserQueryProvider() screenshot.Provider {
	if s == nil {
		return nil
	}
	if s.screenshotRouter != nil {
		return s.screenshotRouter
	}
	if s.bridge != nil && s.bridge.Service != nil {
		return screenshot.NewExtensionProvider(s.bridge.Service, s.screenshotMgr)
	}
	if s.screenshotMgr != nil {
		return screenshot.NewCDPProvider(s.screenshotMgr)
	}
	return nil
}

func buildQueryAPIPayload(query string, engines []string, resp *service.QueryResponse, browserOutcome browserQueryOutcome, browserAction string, explicitErrors ...string) map[string]interface{} {
	for i := range browserOutcome.CollectedResults {
		collection.NormalizeAssets(browserOutcome.CollectedResults[i].Engine, browserOutcome.CollectedResults[i].Assets)
	}

	// Build set of engines that browser query successfully handled
	browserOK := make(map[string]bool)
	for _, e := range browserOutcome.OpenedEngines {
		browserOK[strings.ToLower(e)] = true
	}
	for _, cr := range browserOutcome.CollectedResults {
		browserOK[strings.ToLower(cr.Engine)] = true
	}

	// Filter API errors: suppress errors for engines where browser query succeeded
	combinedErrors := []string{}
	if resp != nil {
		for _, e := range resp.Errors {
			lower := strings.ToLower(e)
			suppressed := false
			for eng := range browserOK {
				if strings.Contains(lower, "engine "+eng) && browserAction != "" {
					suppressed = true
					break
				}
			}
			if !suppressed {
				combinedErrors = append(combinedErrors, e)
			}
		}
	}
	combinedErrors = appendUniqueStrings(combinedErrors, browserOutcome.Errors)
	combinedErrors = appendUniqueStrings(combinedErrors, browserOutcome.AutoCaptureErrors)
	combinedErrors = appendUniqueStrings(combinedErrors, explicitErrors)

	assets := []model.UnifiedAsset{}
	totalCount := 0
	engineStats := map[string]int{}
	if resp != nil {
		assets = resp.Assets
		totalCount = resp.TotalCount
		if resp.EngineStats != nil {
			engineStats = resp.EngineStats
		}
	}
	for _, collected := range browserOutcome.CollectedResults {
		assets = append(assets, collected.Assets...)
		if collected.Total > 0 {
			totalCount += collected.Total
		} else {
			totalCount += len(collected.Assets)
		}
		if len(collected.Assets) > 0 {
			engineStats[collected.Engine] += len(collected.Assets)
		}
	}

	return map[string]interface{}{
		"query":                query,
		"engines":              engines,
		"assets":               assets,
		"totalCount":           totalCount,
		"engineStats":          engineStats,
		"errors":               combinedErrors,
		"browserQuery":         browserOutcome.Enabled,
		"browserAction":        browserAction,
		"browserOpenedEngines": browserOutcome.OpenedEngines,
		"browserCollectedData": browserOutcome.CollectedResults,
		"browserQueryErrors":   browserOutcome.Errors,
		"autoCapture":          browserOutcome.AutoCaptureEnabled,
		"autoCaptureQueryID":   browserOutcome.AutoCaptureQueryID,
		"autoCapturedPaths":    browserOutcome.AutoCapturedPaths,
		"autoCaptureErrors":    browserOutcome.AutoCaptureErrors,
	}
}

// handleAPIQuery 处理API查询请求（用于异步查询）
func (s *Server) handleAPIQuery(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	query := strings.TrimSpace(r.FormValue("query"))
	if err := validateQueryInput(query); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}

	s.applyCookiesFromRequest(r)

	pageSizeStr := r.FormValue("page_size")

	// 解析页码和页大小
	pageSize := 50
	if pageSizeStr != "" {
		if size, err := strconv.Atoi(pageSizeStr); err == nil && size > 0 {
			pageSize = size
		}
	}

	// 解析引擎列表（支持 engines=a&engines=b 和 engines=a,b 两种形式）
	engines := s.queryApp.ResolveEngines(parseEnginesParam(r))
	if len(engines) == 0 {
		writeAPIError(w, http.StatusServiceUnavailable, "no_engines_available", "no engines configured or registered", nil)
		return
	}

	browserQueryID := fmt.Sprintf("query_%d", time.Now().UnixNano())
	browserAction := strings.TrimSpace(r.FormValue("browser_action"))
	browserQueryCh := s.runBrowserQueryAsync(r.Context(), query, engines, parseBoolValue(r.FormValue("browser_query")), browserAction, browserQueryID, nil)

	resp, err := s.queryApp.ExecuteQuery(r.Context(), query, engines, pageSize)
	var browserOutcome browserQueryOutcome
	if browserQueryCh != nil {
		browserOutcome = <-browserQueryCh
	}
	if err != nil {
		writeAPIError(
			w,
			http.StatusBadGateway,
			"query_execution_failed",
			fmt.Sprintf("query failed: %v", err),
			buildQueryAPIPayload(query, engines, nil, browserOutcome, browserAction, fmt.Sprintf("Query failed: %v", err)),
		)
		return
	}

	// 返回JSON结果
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buildQueryAPIPayload(query, engines, resp, browserOutcome, browserAction))
}

// handleIndex 处理首页请求
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	engines := filterStableEngines(s.orchestrator.ListAdapters())
	var fofaCookies, hunterCookies, quakeCookies, zoomeyeCookies []config.Cookie
	proxyServer := ""
	if s.config != nil {
		fofaCookies = s.config.Engines.Fofa.Cookies
		hunterCookies = s.config.Engines.Hunter.Cookies
		quakeCookies = s.config.Engines.Quake.Cookies
		zoomeyeCookies = s.config.Engines.Zoomeye.Cookies
		proxyServer = strings.TrimSpace(s.config.Screenshot.ProxyServer)
	}
	if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "index.html", map[string]interface{}{
		"engines":          engines,
		"staticVersion":    s.staticVersion,
		"proxyServer":      proxyServer,
		"cookieFofa":       cookiesToHeader(fofaCookies),
		"cookieHunter":     cookiesToHeader(hunterCookies),
		"cookieQuake":      cookiesToHeader(quakeCookies),
		"cookieZoomeye":    cookiesToHeader(zoomeyeCookies),
		"cookieHasFofa":    hasCookies(fofaCookies),
		"cookieHasHunter":  hasCookies(hunterCookies),
		"cookieHasQuake":   hasCookies(quakeCookies),
		"cookieHasZoomeye": hasCookies(zoomeyeCookies),
	}) {
		return
	}
}

// handleQuery 处理查询请求
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	query := strings.TrimSpace(r.FormValue("query"))
	if err := validateQueryInput(query); err != nil {
		if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "error.html", map[string]interface{}{
			"error": err.Error(),
		}) {
			return
		}
		return
	}

	s.applyCookiesFromRequest(r)

	pageSize := 50

	// 解析引擎列表（支持 engines=a&engines=b 和 engines=a,b 两种形式）
	engines := parseEnginesParam(r)
	if len(engines) == 0 {
		// 如果没有选择引擎，使用默认引擎
		defaultEngines := filterStableEngines(s.orchestrator.ListAdapters())
		if len(defaultEngines) > 0 {
			engines = []string{defaultEngines[0]}
		}
	}
	if len(engines) == 0 {
		if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "error.html", map[string]interface{}{
			"error": "no engines configured/registered. Please set API keys in configs/config.yaml and enable at least one engine.",
		}) {
			return
		}
		return
	}

	// 执行查询
	req := service.QueryRequest{
		Query:       query,
		Engines:     engines,
		PageSize:    pageSize,
		ProcessData: true,
	}

	resp, err := s.service.Query(r.Context(), req)
	if err != nil {
		if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "error.html", map[string]interface{}{
			"error":   fmt.Sprintf("Query failed: %v", err),
			"query":   query,
			"engines": engines,
		}) {
			return
		}
		return
	}

	// 渲染结果页面
	if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "results.html", map[string]interface{}{
		"query":         query,
		"engines":       engines,
		"assets":        resp.Assets,
		"totalCount":    resp.TotalCount,
		"engineStats":   resp.EngineStats,
		"errors":        resp.Errors,
		"staticVersion": s.staticVersion,
	}) {
		return
	}
}

// handleResults 处理结果页面请求
func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	engines := []string{}
	if engine := strings.TrimSpace(r.URL.Query().Get("engine")); engine != "" {
		engines = []string{engine}
	}

	// 渲染结果页面
	if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "results.html", map[string]interface{}{
		"query":         query,
		"engines":       engines,
		"assets":        []model.UnifiedAsset{},
		"staticVersion": s.staticVersion,
	}) {
		return
	}
}

// handleQuota 处理配额页面请求
func (s *Server) handleQuota(w http.ResponseWriter, r *http.Request) {
	engines := filterStableEngines(s.orchestrator.ListAdapters())
	quotaInfo, errorInfo := s.fetchEngineQuotas(engines)
	if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "quota.html", map[string]interface{}{
		"engines": engines, "quotaInfo": quotaInfo, "errorInfo": errorInfo, "staticVersion": s.staticVersion,
	}) {
		return
	}
}

// fetchEngineQuotas 并发获取所有引擎配额
func (s *Server) fetchEngineQuotas(engines []string) (map[string]*model.QuotaInfo, map[string]string) {
	type quotaResult struct {
		engine string
		quota  *model.QuotaInfo
		err    error
	}
	quotaInfo := make(map[string]*model.QuotaInfo)
	errorInfo := make(map[string]string)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := make(chan quotaResult, len(engines))
	for _, engine := range engines {
		go func(e string) {
			adapter, exists := s.orchestrator.GetAdapter(e)
			if !exists {
				ch <- quotaResult{engine: e, err: fmt.Errorf("adapter not found")}
				return
			}
			quota, err := adapter.GetQuota()
			select {
			case ch <- quotaResult{engine: e, quota: quota, err: err}:
			case <-ctx.Done():
			}
		}(engine)
	}

	results := make(map[string]quotaResult)
	for i := 0; i < len(engines); i++ {
		select {
		case res := <-ch:
			results[res.engine] = res
		case <-ctx.Done():
			break
		}
	}

	for _, engine := range engines {
		res, ok := results[engine]
		if !ok {
			errorInfo[engine] = "timeout: failed to fetch quota"
		} else if res.err != nil {
			errorInfo[engine] = truncateQuotaError(res.err.Error())
		} else if res.quota == nil {
			errorInfo[engine] = "quota not available"
		} else {
			quotaInfo[engine] = res.quota
		}
	}
	return quotaInfo, errorInfo
}

func truncateQuotaError(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "failed to fetch quota"
	}
	if len(msg) > 120 {
		lines := strings.SplitN(msg, "\n", 2)
		short := strings.TrimSpace(lines[0])
		if len(short) > 120 {
			short = short[:120] + "..."
		}
		return short
	}
	return msg
}

// handleQueryStatus 处理查询状态请求
func (s *Server) handleQueryStatus(w http.ResponseWriter, r *http.Request) {
	queryID := r.URL.Query().Get("query_id")
	if queryID == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_query_id", "query_id is required", nil)
		return
	}

	// 获取查询状态
	s.queryMutex.RLock()
	status, exists := s.queryStatus[queryID]
	var statusCopy QueryStatus
	if exists && status != nil {
		statusCopy = *status
	}
	s.queryMutex.RUnlock()

	if !exists {
		writeAPIError(w, http.StatusNotFound, "query_not_found", "query not found", map[string]string{"query_id": queryID})
		return
	}

	// 返回JSON结果
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statusCopy)
}

// handleAccountPage renders the account management page (GET /account).
func (s *Server) handleAccountPage(w http.ResponseWriter, r *http.Request) {
	username := ""
	tokenPrefix := ""
	isMultiUser := s.userRepo != nil
	role := ""

	// Try to get current user from session
	currentUser := s.getCurrentUser(r)
	if currentUser != nil && currentUser.ID > 0 {
		// Real user from user DB
		username = currentUser.Username
		role = currentUser.Role
	} else if currentUser != nil {
		// Synthetic admin (token auth, userID=-1)
		role = currentUser.Role
		if s.config != nil {
			token := s.adminToken()
			if len(token) >= 8 {
				tokenPrefix = token[:8]
			}
		}
	} else if s.config != nil {
		// Legacy config-based user
		username = s.config.Web.Auth.Username
		token := s.adminToken()
		if len(token) >= 8 {
			tokenPrefix = token[:8]
		}
	}

	if !s.renderTemplateWithNonce(r, w, http.StatusOK, "account-page", map[string]interface{}{
		"username":      username,
		"tokenPrefix":   tokenPrefix,
		"staticVersion": s.staticVersion,
		"isMultiUser":   isMultiUser,
		"userID":        currentUserID(r),
		"role":          role,
	}) {
		return
	}
}

// handleGetAdminToken returns the admin token for authenticated users (GET /api/account/admin-token).
// Used by the account page to allow copying the token into the browser extension.
func (s *Server) handleGetAdminToken(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	token := s.adminToken()
	// When auth is disabled there is no admin token and no auth model in play;
	// preserve the legacy behavior of returning an empty token (no escalation risk).
	if token == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"token":   "",
		})
		return
	}
	// P0 fix (FINDING-001): a real admin token grants synthetic-admin (userID=-1)
	// that bypasses all role checks. Returning it in plaintext to any logged-in
	// user allowed vertical privilege escalation (normal user → super admin).
	//
	// Authorized identities:
	//   - adminSyntheticUserID (-1): request authenticated via X-Admin-Token
	//   - userID == 0: legacy single-user mode (config admin account, no user DB)
	// Only multi-user DB users must be checked for the admin role.
	uid := currentUserID(r)
	if uid > 0 {
		if ok, reason := s.requireAdmin(r); !ok {
			writeAPIError(w, http.StatusForbidden, "forbidden", reason, nil)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"token":   token,
	})
}

// currentUserID returns the user ID from context, or 0.
func currentUserID(r *http.Request) int64 {
	if uid, ok := r.Context().Value(contextKeyUserID).(int64); ok {
		return uid
	}
	return 0
}

// handleChangePassword handles POST /api/v1/account/change-password.
// In multi-user mode, redirects user-DB users to /api/v1/users/{id}/password.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// Multi-user mode: if the user has a real DB account, redirect to user endpoint
	if s.userRepo != nil {
		uid := currentUserID(r)
		if uid > 0 {
			writeJSON(w, http.StatusConflict, map[string]interface{}{
				"error":       "use /api/v1/users/" + fmt.Sprintf("%d", uid) + "/password instead",
				"redirect_to": fmt.Sprintf("/api/v1/users/%d/password", uid),
			})
			return
		}
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if s.config == nil || s.configManager == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server configuration error"})
		return
	}

	currentHash := s.config.Web.Auth.PasswordHash
	if currentHash == "" || !config.CheckPassword(req.CurrentPassword, currentHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}

	if len(req.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new password must be at least 8 characters"})
		return
	}

	newHash, err := config.HashPassword(req.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to process password"})
		return
	}

	s.configMutex.Lock()
	s.config.Web.Auth.PasswordHash = newHash
	if err := s.configManager.Save(); err != nil {
		s.config.Web.Auth.PasswordHash = currentHash
		s.configMutex.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist config"})
		return
	}
	s.configMutex.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"success": "password updated"})
}
