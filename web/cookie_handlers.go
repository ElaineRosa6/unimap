package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/screenshot"
)

// handleImportCookieJSON 导入浏览器导出的Cookie JSON
func (s *Server) handleImportCookieJSON(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}
	if s.config == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "config_not_loaded", "config not loaded", nil)
		return
	}
	if s.currentScreenshotEngine() == "extension" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":      true,
			"cookieHeader": "",
			"engine":       "extension",
			"message":      "extension mode uses browser session; cookie import is optional",
		})
		return
	}

	engine := strings.TrimSpace(r.FormValue("engine"))
	jsonStr := r.FormValue("cookie_json")
	if engine == "" || strings.TrimSpace(jsonStr) == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "engine and cookie_json are required", nil)
		return
	}

	cookies, err := config.ParseCookieJSON(jsonStr, config.DefaultCookieDomain(engine))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_cookie_json", "invalid cookie json", err.Error())
		return
	}
	if len(cookies) == 0 {
		writeAPIError(w, http.StatusBadRequest, "empty_cookie_set", "no cookies parsed", nil)
		return
	}

	s.configMutex.Lock()
	switch strings.ToLower(engine) {
	case "fofa":
		s.config.Engines.Fofa.Cookies = cookies
	case "hunter":
		s.config.Engines.Hunter.Cookies = cookies
	case "quake":
		s.config.Engines.Quake.Cookies = cookies
	case "zoomeye":
		s.config.Engines.Zoomeye.Cookies = cookies
	default:
		s.configMutex.Unlock()
		writeAPIError(w, http.StatusBadRequest, "unsupported_engine", "unsupported engine", map[string]string{"engine": engine})
		return
	}
	if s.screenshotMgr != nil {
		s.screenshotMgr.SetCookies(engine, convertConfigCookies(cookies))
	}
	if s.configManager != nil {
		if err := s.configManager.Save(); err != nil {
			logger.Warnf("Failed to persist cookies: %v", err)
		}
	}
	s.configMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"cookieHeader": cookiesToHeader(cookies),
	})
}

// handleVerifyCookies 验证Cookie是否可访问搜索结果页
func (s *Server) handleVerifyCookies(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	query := strings.TrimSpace(r.FormValue("query"))
	if err := validateQueryInput(query); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}

	s.applyCookiesFromRequest(r)

	engines := parseEnginesParam(r)
	if len(engines) == 0 {
		engines = s.orchestrator.ListAdapters()
	}
	if len(engines) == 0 {
		writeAPIError(w, http.StatusServiceUnavailable, "no_engines_available", "no engines configured or registered", nil)
		return
	}

	ctx := r.Context()
	results := make(map[string]interface{})
	engineMode := s.currentScreenshotEngine()
	for _, engine := range engines {
		ok, title, hint, err := s.verifyEngineSession(ctx, engineMode, engine, query)
		payload := map[string]interface{}{
			"ok":    ok,
			"title": title,
			"hint":  hint,
		}
		if err != nil {
			payload["error"] = err.Error()
		}
		results[engine] = payload
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":   query,
		"results": results,
	})
}

// handleSaveCookies 处理保存Cookie请求
func (s *Server) handleSaveCookies(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	s.applyCookiesFromRequest(r)
	engineMode := s.currentScreenshotEngine()
	if engineMode == "extension" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"engine":  "extension",
			"message": "extension mode uses browser session; cookie injection is skipped",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"engine":  engineMode,
	})
}

func (s *Server) applyCookiesFromRequest(r *http.Request) {
	if s.config == nil {
		return
	}
	_ = r.ParseForm()

	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	engineMode := s.currentScreenshotEngine()

	if engineMode == "extension" {
		changed := false
		if _, present := r.Form["proxy_server"]; present {
			proxy := strings.TrimSpace(r.FormValue("proxy_server"))
			if s.config.Screenshot.ProxyServer != proxy {
				s.config.Screenshot.ProxyServer = proxy
				changed = true
				if s.screenshotMgr != nil {
					s.screenshotMgr.SetProxyServer(proxy)
				}
			}
		}

		if changed && s.configManager != nil {
			if err := s.configManager.Save(); err != nil {
				logger.Warnf("Failed to persist extension proxy config: %v", err)
			}
		}
		logger.Infof("Cookie apply mode=extension_session: skipped cookie injection, proxy update only")
		return
	}

	changed := false
	clear := strings.EqualFold(strings.TrimSpace(r.FormValue("clear_cookies")), "true")
	if clear {
		s.config.Engines.Fofa.Cookies = nil
		s.config.Engines.Hunter.Cookies = nil
		s.config.Engines.Quake.Cookies = nil
		s.config.Engines.Zoomeye.Cookies = nil
		changed = true
		if s.screenshotMgr != nil {
			s.screenshotMgr.SetCookies("fofa", nil)
			s.screenshotMgr.SetCookies("hunter", nil)
			s.screenshotMgr.SetCookies("quake", nil)
			s.screenshotMgr.SetCookies("zoomeye", nil)
		}
	}

	apply := func(engine, value string) {
		if clear {
			return
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		cookies := config.ParseCookieHeader(value, config.DefaultCookieDomain(engine))
		if len(cookies) == 0 {
			return
		}

		switch strings.ToLower(engine) {
		case "fofa":
			s.config.Engines.Fofa.Cookies = cookies
		case "hunter":
			s.config.Engines.Hunter.Cookies = cookies
		case "quake":
			s.config.Engines.Quake.Cookies = cookies
		case "zoomeye":
			s.config.Engines.Zoomeye.Cookies = cookies
		default:
			return
		}
		changed = true

		if s.screenshotMgr != nil {
			s.screenshotMgr.SetCookies(engine, convertConfigCookies(cookies))
		}
	}

	apply("fofa", r.FormValue("cookie_fofa"))
	apply("hunter", r.FormValue("cookie_hunter"))
	apply("zoomeye", r.FormValue("cookie_zoomeye"))
	apply("quake", r.FormValue("cookie_quake"))

	if _, present := r.Form["proxy_server"]; present {
		proxy := strings.TrimSpace(r.FormValue("proxy_server"))
		if s.config.Screenshot.ProxyServer != proxy {
			s.config.Screenshot.ProxyServer = proxy
			changed = true
			if s.screenshotMgr != nil {
				s.screenshotMgr.SetProxyServer(proxy)
			}
		}
	}

	if changed && s.configManager != nil {
		if err := s.configManager.Save(); err != nil {
			logger.Warnf("Failed to persist cookies: %v", err)
		}
	}
	logger.Infof("Cookie apply mode=cdp_cookie_injection: cookie/proxy updates applied")
}

func (s *Server) currentScreenshotEngine() string {
	if s == nil || s.config == nil {
		return "cdp"
	}
	engine := strings.ToLower(strings.TrimSpace(s.config.Screenshot.Engine))
	if engine == "extension" {
		return "extension"
	}
	return "cdp"
}

func (s *Server) verifyEngineSession(ctx context.Context, engineMode, engine, query string) (bool, string, string, error) {
	if engineMode == "extension" {
		if s.bridge.Service == nil {
			return false, "", "extension_not_paired", fmt.Errorf("bridge_unavailable")
		}
		if s.screenshotMgr == nil {
			return false, "", "extension_session_required", fmt.Errorf("screenshot manager not initialized")
		}

		searchURL := strings.TrimSpace(s.screenshotMgr.BuildSearchEngineURL(engine, query))
		if searchURL == "" {
			return false, "", "unsupported engine", fmt.Errorf("unsupported engine: %s", engine)
		}

		result, err := s.bridge.Service.Submit(ctx, screenshot.BridgeTask{
			RequestID:    fmt.Sprintf("verify_%s_%d", strings.ToLower(strings.TrimSpace(engine)), time.Now().UnixNano()),
			URL:          searchURL,
			BatchID:      "cookie_verify",
			WaitStrategy: "load",
			Action:       "collect",
			Timeout:      20 * time.Second,
		})
		if err != nil {
			return false, "", "extension_session_required", err
		}
		if !result.Success {
			if strings.TrimSpace(result.Error) != "" {
				return false, "", "extension_session_required", fmt.Errorf("%s", result.Error)
			}
			if strings.TrimSpace(result.ErrorCode) != "" {
				return false, "", "extension_session_required", fmt.Errorf("%s", result.ErrorCode)
			}
			return false, "", "extension_session_required", fmt.Errorf("extension verification failed")
		}

		if loginRequiredFromBridgeResult(result) {
			return false, titleFromBridgeResult(result), "login_required", nil
		}
		if hasCollectedAssets(result) {
			return true, titleFromBridgeResult(result), "ok", nil
		}
		return false, titleFromBridgeResult(result), "no_results_or_login_required", nil
	}

	if s.screenshotMgr == nil {
		return false, "", "cdp_cookie_missing", fmt.Errorf("screenshot manager not initialized")
	}
	cookies := s.screenshotMgr.GetCookies(engine)
	return s.screenshotMgr.ValidateSearchEngineResult(ctx, engine, query, cookies)
}

func titleFromBridgeResult(result screenshot.BridgeResult) string {
	if title, ok := result.StructuredCollectedData["title"].(string); ok {
		return strings.TrimSpace(title)
	}
	return strings.TrimSpace(result.CollectedData)
}

func hasCollectedAssets(result screenshot.BridgeResult) bool {
	items, ok := result.StructuredCollectedData["items"].([]interface{})
	if ok && len(items) > 0 {
		return true
	}
	if total, ok := result.StructuredCollectedData["total"].(float64); ok && total > 0 {
		return true
	}
	return false
}

func loginRequiredFromBridgeResult(result screenshot.BridgeResult) bool {
	data := result.StructuredCollectedData
	if required, ok := data["login_required"].(bool); ok && required {
		return true
	}
	textParts := []string{titleFromBridgeResult(result), strings.TrimSpace(result.Error), strings.TrimSpace(result.ErrorCode)}
	if v, ok := data["extraction_error"].(string); ok {
		textParts = append(textParts, v)
	}
	joined := strings.ToLower(strings.Join(textParts, " "))
	markers := []string{"login", "sign in", "signin", "登录", "登陆", "请先登录", "unauthorized", "未登录"}
	for _, marker := range markers {
		if strings.Contains(joined, marker) {
			return true
		}
	}
	return false
}

// engineDomain maps an engine name to its login cookie domain.
// Returns empty string when the engine doesn't support cookie-based detection.
func engineDomain(engine string) string {
	switch engine {
	case "hunter":
		return "hunter.qianxin.com"
	case "fofa":
		return "fofa.info"
	case "quake":
		return "quake.360.cn"
	default:
		return ""
	}
}

// judgeLoginByCookieNames inspects a flat name→value cookie map and returns
// true when the engine-specific login markers are present.
func judgeLoginByCookieNames(engine string, byName map[string]string) bool {
	switch engine {
	case "hunter":
		return strings.TrimSpace(byName["next"]) != ""
	case "fofa":
		// FOFA cookie name happens to be case-sensitive on the server side,
		// but we've historically accepted any case for robustness.
		for k, v := range byName {
			if strings.EqualFold(k, "user") && strings.TrimSpace(v) != "" {
				return true
			}
		}
		return false
	case "quake":
		return strings.TrimSpace(byName["Q"]) != "" && strings.TrimSpace(byName["T"]) != ""
	default:
		return false
	}
}

// detectLoginViaCDP reads cookies via CDP protocol and judges login state.
// No page opening needed — direct cookie store query.
// Returns (loggedIn, reason). When CDP is connected but no login marker is
// found, reason is "cdp_session_unverified" (so the UI can differentiate
// "unverified browser session" from "no browser session at all").
func (s *Server) detectLoginViaCDP(ctx context.Context, engine string, cookieSet bool) (bool, string) {
	domain := engineDomain(engine)
	if domain == "" {
		if cookieSet {
			return false, "cookie_configured"
		}
		return false, "cdp_session_unverified"
	}

	cookies, err := s.getCDPCookies(ctx, domain)
	if err != nil {
		if cookieSet {
			return false, "cookie_configured"
		}
		return false, "cdp_session_unverified"
	}

	byName := make(map[string]string, len(cookies))
	for _, c := range cookies {
		byName[c.Name] = c.Value
	}
	if judgeLoginByCookieNames(engine, byName) {
		return true, "browser_session"
	}
	if cookieSet {
		return false, "cookie_configured"
	}
	return false, "cdp_session_unverified"
}

// detectLoginViaExtension reads cookies via extension Bridge (chrome.cookies API)
// and judges login state. No page opening needed.
// Returns (loggedIn, reason). When Extension is paired but no login marker is
// found, reason is "extension_paired_session_unverified".
func (s *Server) detectLoginViaExtension(ctx context.Context, engine string, cookieSet bool) (bool, string) {
	if s.bridge == nil || s.bridge.Service == nil {
		if cookieSet {
			return false, "cookie_configured"
		}
		return false, "no_session"
	}

	domain := engineDomain(engine)
	if domain == "" {
		if cookieSet {
			return false, "cookie_configured"
		}
		return false, "extension_paired_session_unverified"
	}

	requestID := fmt.Sprintf("cookies_%s_%d", engine, time.Now().UnixNano())
	result, err := s.bridge.Service.Submit(ctx, screenshot.BridgeTask{
		RequestID: requestID,
		URL:       domain,
		BatchID:   "cookie_read",
		Action:    "get_cookies",
		Timeout:   8 * time.Second,
	})
	if err != nil {
		logger.Warnf("extension cookie read failed for %s: %v", engine, err)
		if cookieSet {
			return false, "cookie_configured"
		}
		return false, "extension_paired_session_unverified"
	}
	if !result.Success {
		if cookieSet {
			return false, "cookie_configured"
		}
		return false, "extension_paired_session_unverified"
	}

	byName := make(map[string]string)
	if data, ok := result.StructuredCollectedData["cookies"].([]interface{}); ok {
		for _, item := range data {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			value, _ := m["value"].(string)
			if name == "" {
				continue
			}
			byName[name] = value
		}
	}

	if judgeLoginByCookieNames(engine, byName) {
		return true, "browser_session"
	}
	if cookieSet {
		return false, "cookie_configured"
	}
	return false, "extension_paired_session_unverified"
}

func cookiesToHeader(cookies []config.Cookie) string {
	if len(cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if strings.TrimSpace(c.Name) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	return strings.Join(parts, "; ")
}

func hasCookies(cookies []config.Cookie) bool {
	for _, c := range cookies {
		if strings.TrimSpace(c.Name) != "" {
			return true
		}
	}
	return false
}

// handleCookieLoginStatus returns per-engine login status for the UI.
// GET /api/cookies/login-status?query=...
// Detects: CDP connected, Extension paired, per-engine login wall detection.
func (s *Server) handleCookieLoginStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = `protocol="http"`
	}

	// Detect CDP connection status — actual HTTP probe to /json/version
	cdpConnected := false
	if s.screenshotMgr != nil && s.screenshotMgr.RemoteDebugURL() != "" {
		baseURL := s.resolveCDPURL()
		if online, _, _ := s.checkCDPStatus(r.Context(), baseURL); online {
			cdpConnected = true
		}
	}

	// Detect Extension pairing status — check for live clients (seen in last 15s)
	extPaired := false
	if s.bridge != nil && s.bridge.Service != nil {
		extPaired = s.activeBridgeLiveTokens() > 0
	}

	// Check per-engine login status
	engines := []string{"fofa", "hunter", "zoomeye", "quake"}
	results := make([]map[string]interface{}, 0, len(engines))

	if cdpConnected || extPaired {
		// Browser session available (CDP or Extension) → read cookies to detect login.
		// CDP reads directly via protocol; Extension reads via chrome.cookies API.
		// Same judgment logic applies to both channels.
		for _, engine := range engines {
			loginURL := ""
			if s.screenshotMgr != nil {
				loginURL = s.screenshotMgr.EngineLoginURL(engine)
			}

			// Config-level fallback
			var cookieSet bool
			if s.config != nil {
				s.configMutex.Lock()
				switch engine {
				case "fofa":
					cookieSet = hasCookies(s.config.Engines.Fofa.Cookies)
				case "hunter":
					cookieSet = hasCookies(s.config.Engines.Hunter.Cookies)
				case "quake":
					cookieSet = hasCookies(s.config.Engines.Quake.Cookies)
				case "zoomeye":
					cookieSet = hasCookies(s.config.Engines.Zoomeye.Cookies)
				}
				s.configMutex.Unlock()
			}

			// Read cookies via available channel and judge login state.
			reason := "no_session"
			loggedIn := false

			if cdpConnected && s.screenshotMgr != nil {
				// Channel 1: CDP protocol — direct cookie read, no page opening.
				cdpCtx, cdpCancel := context.WithTimeout(r.Context(), 8*time.Second)
				loggedIn, reason = s.detectLoginViaCDP(cdpCtx, engine, cookieSet)
				cdpCancel()
			} else if extPaired {
				// Channel 2: Extension — read via chrome.cookies API, no page opening.
				extCtx, extCancel := context.WithTimeout(r.Context(), 8*time.Second)
				loggedIn, reason = s.detectLoginViaExtension(extCtx, engine, cookieSet)
				extCancel()
			}

			results = append(results, map[string]interface{}{
				"engine":        engine,
				"logged_in":     loggedIn,
				"reason":        reason,
				"title":         "",
				"login_url":     loginURL,
				"cdp_connected": cdpConnected,
				"ext_paired":    extPaired,
			})
		}
	} else {
		// No browser session → just check if cookies are configured
		if s.config != nil {
			s.configMutex.Lock()
			for _, engine := range engines {
				var cookieSet bool
				loginURL := ""
				if s.screenshotMgr != nil {
					loginURL = s.screenshotMgr.EngineLoginURL(engine)
				}
				switch engine {
				case "fofa":
					cookieSet = hasCookies(s.config.Engines.Fofa.Cookies)
				case "hunter":
					cookieSet = hasCookies(s.config.Engines.Hunter.Cookies)
				case "quake":
					cookieSet = hasCookies(s.config.Engines.Quake.Cookies)
				case "zoomeye":
					cookieSet = hasCookies(s.config.Engines.Zoomeye.Cookies)
				}
				reason := "no_session"
				if cookieSet {
					reason = "cookie_configured"
				}
				results = append(results, map[string]interface{}{
					"engine":        engine,
					"logged_in":     false,
					"reason":        reason,
					"login_url":     loginURL,
					"cdp_connected": cdpConnected,
					"ext_paired":    extPaired,
				})
			}
			s.configMutex.Unlock()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"cdp_connected": cdpConnected,
		"ext_paired":    extPaired,
		"engines":       results,
	})
}

