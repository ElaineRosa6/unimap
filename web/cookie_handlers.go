package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/unimap-icp-hunter/project/internal/config"
	"github.com/unimap-icp-hunter/project/internal/logger"
	"github.com/unimap-icp-hunter/project/internal/screenshot"
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

	if cdpConnected {
		// CDP connected → check per-engine login status via CDP cookie reading.
		// No page opening needed — we directly read cookies from the browser session.
		cdpCtx, cdpCancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cdpCancel()

		for _, engine := range engines {
			loginURL := ""
			if s.screenshotMgr != nil {
				loginURL = s.screenshotMgr.EngineLoginURL(engine)
			}

			// Check if cookies are configured for this engine (config-level fallback)
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

			// Try to detect actual login status via CDP cookie reading.
			// Logic: compare key cookies — logged-in sessions have more keys
			// and specific marker cookies (e.g., "user" for FOFA, "next" for Hunter).
			reason := "no_session"
			loggedIn := false

			if s.screenshotMgr != nil {
				switch engine {
				case "hunter":
					// Hunter: "next" cookie indicates login state.
					// When logged in, "next" cookie exists with value containing
					// "https://hunter.qianxin.com/api/uLogin". Missing → not logged in.
					const hunterDomain = "hunter.qianxin.com"
					cookies, err := s.getCDPCookies(cdpCtx, hunterDomain)
					if err != nil {
						// CDP read failed — fall back to config check
						if cookieSet {
							reason = "cookie_configured"
						}
						break
					}
					hasNext := false
					for _, c := range cookies {
						if c.Name == "next" {
							hasNext = true
							// Has "next" key → logged in (value is the redirect target after login)
							loggedIn = true
							reason = "browser_session"
							break
						}
					}
					if !hasNext && len(cookies) == 0 {
						// No cookies at all for this domain
						if cookieSet {
							reason = "cookie_configured"
						}
					}

				case "fofa":
					// FOFA: "user" cookie indicates logged-in state.
					// If "user" key exists → logged in. Missing → not logged in.
					const fofaDomain = "fofa.info"
					cookies, err := s.getCDPCookies(cdpCtx, fofaDomain)
					if err != nil {
						if cookieSet {
							reason = "cookie_configured"
						}
						break
					}
					for _, c := range cookies {
						if strings.ToLower(c.Name) == "user" {
							loggedIn = true
							reason = "browser_session"
							break
						}
					}
					if !loggedIn && len(cookies) == 0 {
						if cookieSet {
							reason = "cookie_configured"
						}
					}

				case "quake":
					// Quake: "Q" and "T" cookies with non-empty values indicate logged-in state.
					// Both must have non-empty values → logged in. Either empty or missing → not logged in.
					const quakeDomain = "quake.360.cn"
					cookies, err := s.getCDPCookies(cdpCtx, quakeDomain)
					if err != nil {
						if cookieSet {
							reason = "cookie_configured"
						}
						break
					}
					var qVal, tVal string
					for _, c := range cookies {
						if c.Name == "Q" {
							qVal = c.Value
						}
						if c.Name == "T" {
							tVal = c.Value
						}
					}
					if qVal != "" && tVal != "" {
						loggedIn = true
						reason = "browser_session"
					} else if len(cookies) == 0 {
						if cookieSet {
							reason = "cookie_configured"
						}
					}

				default:
					// Other engines: config-level check only
					if cookieSet {
						reason = "cookie_configured"
					}
				}
			} else {
				// screenshotMgr not available, config-level check
				if cookieSet {
					reason = "cookie_configured"
				}
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
	} else if extPaired && s.screenshotRouter != nil {
		// Extension paired → use bridge to collect from each engine page.
		// The extension's capture.js detects login walls via keywords
		// ("请登录", "请先登录", "login required", etc.) and returns
		// is_login_wall / login_required flags in structured data.
		collectCtx, collectCancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer collectCancel()

		for _, engine := range engines {
			loginURL := ""
			if s.screenshotMgr != nil {
				loginURL = s.screenshotMgr.EngineLoginURL(engine)
			}

			collectResults, err := s.screenshotRouter.CollectSearchEngineResult(collectCtx, engine, query, fmt.Sprintf("logincheck_%s_%d", engine, time.Now().UnixNano()))
			if err != nil {
				logger.Warnf("login status collect failed for %s: %v", engine, err)
				collectResults = []screenshot.CollectResult{{Engine: engine, IsLoginWall: false}}
			}

			var loggedIn, isLoginWall bool
			if len(collectResults) > 0 {
				isLoginWall = collectResults[0].IsLoginWall || collectResults[0].LoginRequired
				loggedIn = len(collectResults[0].Assets) > 0 || collectResults[0].Total > 0
			}

			reason := "no_session"
			if isLoginWall {
				reason = "login_required"
				loggedIn = false
			} else if loggedIn {
				reason = "browser_session"
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
	} else if extPaired {
		// Extension paired but no router → fall back to unverified
		for _, engine := range engines {
			loginURL := ""
			if s.screenshotMgr != nil {
				loginURL = s.screenshotMgr.EngineLoginURL(engine)
			}
			results = append(results, map[string]interface{}{
				"engine":        engine,
				"logged_in":     false,
				"reason":        "extension_paired_session_unverified",
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
