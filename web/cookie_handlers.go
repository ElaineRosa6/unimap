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
	if !requireTrustedRequest(w, r, s.allowedOrigins()) {
		return
	}
	if s.currentConfig() == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "config_not_loaded", "config not loaded", nil)
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
		writeAPIError(w, http.StatusBadRequest, "invalid_cookie_json", "invalid cookie json", nil)
		return
	}
	if len(cookies) == 0 {
		writeAPIError(w, http.StatusBadRequest, "empty_cookie_set", "no cookies parsed", nil)
		return
	}

	engine = strings.ToLower(engine)
	if !isBrowserEngine(engine) {
		writeAPIError(w, http.StatusBadRequest, "unsupported_engine", "unsupported engine", map[string]string{"engine": engine})
		return
	}
	if _, err := s.updateConfig(func(cfg *config.Config) error {
		setEngineCookies(cfg, engine, cookies)
		return nil
	}); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "save_failed", "failed to persist cookies", nil)
		return
	}
	if s.screenshotMgr != nil {
		s.screenshotMgr.SetCookies(engine, convertConfigCookies(cookies))
	}

	w.Header().Set("Content-Type", "application/json")
	payload := map[string]interface{}{
		"success":      true,
		"cookieHeader": cookiesToHeader(cookies),
	}
	if s.currentScreenshotEngine() == "extension" {
		payload["engine"] = "extension"
		payload["message"] = "cookie stored for CDP fallback; extension session remains primary"
	}
	json.NewEncoder(w).Encode(payload) //nolint:errcheck
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
		engines = filterStableEngines(s.orchestrator.ListAdapters())
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
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"query":   query,
		"results": results,
	})
}

// handleSaveCookies 处理保存Cookie请求
func (s *Server) handleSaveCookies(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireTrustedRequest(w, r, s.allowedOrigins()) {
		return
	}

	s.applyCookiesFromRequest(r)
	engineMode := s.currentScreenshotEngine()
	if engineMode == "extension" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"engine":  "extension",
			"message": "cookies stored for CDP fallback; extension session remains primary",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"engine":  engineMode,
	})
}

func (s *Server) applyCookiesFromRequest(r *http.Request) {
	if s.currentConfig() == nil {
		return
	}
	_ = r.ParseForm()

	clear := strings.EqualFold(strings.TrimSpace(r.FormValue("clear_cookies")), "true")
	proxy, proxyPresent := r.Form["proxy_server"]
	committed, err := s.updateConfig(func(cfg *config.Config) error {
		if clear {
			clearEngineCookies(cfg)
		}
		if !clear {
			for _, engine := range browserEngines() {
				value := strings.TrimSpace(r.FormValue("cookie_" + engine))
				if value == "" {
					continue
				}
				cookies := config.ParseCookieHeader(value, config.DefaultCookieDomain(engine))
				if len(cookies) > 0 {
					setEngineCookies(cfg, engine, cookies)
				}
			}
		}
		if proxyPresent {
			cfg.Screenshot.ProxyServer = strings.TrimSpace(proxy[0])
		}
		return nil
	})
	if err != nil {
		logger.Warnf("Failed to persist cookies: %v", err)
		return
	}
	if s.screenshotMgr != nil {
		for _, engine := range browserEngines() {
			s.screenshotMgr.SetCookies(engine, convertConfigCookies(engineCookies(committed, engine)))
		}
		s.screenshotMgr.SetProxyServer(committed.Screenshot.ProxyServer)
	}
}

func setEngineCookies(cfg *config.Config, engine string, cookies []config.Cookie) {
	switch engine {
	case "fofa":
		cfg.Engines.Fofa.Cookies = cookies
	case "hunter":
		cfg.Engines.Hunter.Cookies = cookies
	case "quake":
		cfg.Engines.Quake.Cookies = cookies
	case "zoomeye":
		cfg.Engines.Zoomeye.Cookies = cookies
	case "shodan":
		cfg.Engines.Shodan.Cookies = cookies
	case "censys":
		cfg.Engines.Censys.Cookies = cookies
	case "daydaymap":
		cfg.Engines.Daydaymap.Cookies = cookies
	}
}
func engineCookies(cfg *config.Config, engine string) []config.Cookie {
	switch engine {
	case "fofa":
		return cfg.Engines.Fofa.Cookies
	case "hunter":
		return cfg.Engines.Hunter.Cookies
	case "quake":
		return cfg.Engines.Quake.Cookies
	case "zoomeye":
		return cfg.Engines.Zoomeye.Cookies
	case "shodan":
		return cfg.Engines.Shodan.Cookies
	case "censys":
		return cfg.Engines.Censys.Cookies
	case "daydaymap":
		return cfg.Engines.Daydaymap.Cookies
	}
	return nil
}
func clearEngineCookies(cfg *config.Config) {
	for _, engine := range browserEngines() {
		setEngineCookies(cfg, engine, nil)
	}
}

func browserEngines() []string {
	return []string{"fofa", "hunter", "zoomeye", "quake", "shodan", "censys", "daydaymap"}
}

func isBrowserEngine(engine string) bool {
	for _, candidate := range browserEngines() {
		if engine == candidate {
			return true
		}
	}
	return false
}

func (s *Server) currentScreenshotEngine() string {
	if s == nil {
		return "cdp"
	}
	cfg := s.currentConfig()
	if cfg == nil {
		return "cdp"
	}
	engine := strings.ToLower(strings.TrimSpace(cfg.Screenshot.Engine))
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
	if result.StructuredCollectedData != nil && strings.TrimSpace(result.StructuredCollectedData.Title) != "" {
		return strings.TrimSpace(result.StructuredCollectedData.Title)
	}
	if result.StructuredCollectedData != nil && result.StructuredCollectedData.Extra != nil {
		if title, ok := result.StructuredCollectedData.Extra["title"].(string); ok {
			return strings.TrimSpace(title)
		}
	}
	return strings.TrimSpace(result.CollectedData)
}

func hasCollectedAssets(result screenshot.BridgeResult) bool {
	if result.StructuredCollectedData != nil && len(result.StructuredCollectedData.Items) > 0 {
		return true
	}
	return false
}

func loginRequiredFromBridgeResult(result screenshot.BridgeResult) bool {
	if result.StructuredCollectedData != nil && (result.StructuredCollectedData.IsLoginWall || result.StructuredCollectedData.LoginRequired) {
		return true
	}
	if result.StructuredCollectedData != nil && result.StructuredCollectedData.Extra != nil {
		if required, ok := result.StructuredCollectedData.Extra["login_required"].(bool); ok && required {
			return true
		}
	}
	textParts := []string{titleFromBridgeResult(result), strings.TrimSpace(result.Error), strings.TrimSpace(result.ErrorCode)}
	if result.StructuredCollectedData != nil {
		textParts = append(textParts, result.StructuredCollectedData.ExtractionError)
	}
	if result.StructuredCollectedData != nil && result.StructuredCollectedData.Extra != nil {
		if v, ok := result.StructuredCollectedData.Extra["extraction_error"].(string); ok {
			textParts = append(textParts, v)
		}
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
		return "quake.360.net"
	case "zoomeye":
		return "zoomeye.org"
	case "shodan":
		return "shodan.io"
	case "censys":
		return "censys.io"
	case "daydaymap":
		return "daydaymap.com"
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
	case "zoomeye":
		// ZoomEye login cookie
		return strings.TrimSpace(byName["_xsrf"]) != "" || strings.TrimSpace(byName["session"]) != ""
	case "shodan":
		return strings.TrimSpace(byName["dotcom_user"]) != ""
	case "censys":
		// Censys is API-key based; no browser session cookie marker.
		return false
	case "daydaymap":
		// DayDayMap is API-key based; no browser session cookie marker.
		return false
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
		Timeout:   20 * time.Second,
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

	cookies, cookieErr := bridgeCookiesFromResult(engine, result)
	if cookieErr != nil {
		return false, "extension_paired_session_unverified"
	}
	byName := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie.Value
	}

	if judgeLoginByCookieNames(engine, byName) {
		return true, "browser_session"
	}
	if cookieSet {
		return false, "cookie_configured"
	}
	return false, "extension_paired_session_unverified"
}

func bridgeCookiesFromResult(engine string, result screenshot.BridgeResult) ([]config.Cookie, error) {
	if result.StructuredCollectedData == nil || len(result.StructuredCollectedData.Cookies) == 0 {
		return nil, fmt.Errorf("extension returned no cookies for %s", engine)
	}
	expectedDomain := strings.ToLower(strings.TrimPrefix(engineDomain(engine), "."))
	if expectedDomain == "" {
		return nil, fmt.Errorf("unsupported browser engine: %s", engine)
	}
	cookies := make([]config.Cookie, 0, len(result.StructuredCollectedData.Cookies))
	for _, raw := range result.StructuredCollectedData.Cookies {
		name := strings.TrimSpace(raw.Name)
		if name == "" || raw.Value == "" {
			continue
		}
		domain := strings.ToLower(strings.TrimSpace(raw.Domain))
		if domain == "" {
			domain = "." + expectedDomain
		}
		normalizedDomain := strings.TrimPrefix(domain, ".")
		if normalizedDomain != expectedDomain && !strings.HasSuffix(normalizedDomain, "."+expectedDomain) {
			continue
		}
		path := strings.TrimSpace(raw.Path)
		if path == "" {
			path = "/"
		}
		cookies = append(cookies, config.Cookie{
			Name: name, Value: raw.Value, Domain: domain, Path: path,
			HTTPOnly: raw.HTTPOnly, Secure: raw.Secure,
		})
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("extension returned no usable cookies for %s", engine)
	}
	return cookies, nil
}

// syncExtensionCookiesToCDP copies credentials from the logged-in Extension
// profile into CDP. Cookies follow the existing config-secret boundary;
// origin Web Storage stays in memory only.
func (s *Server) syncExtensionCookiesToCDP(ctx context.Context, engine string) (int, error) {
	if s == nil || s.bridge == nil || s.bridge.Service == nil || s.screenshotMgr == nil {
		return 0, fmt.Errorf("browser credential handoff is unavailable")
	}
	domain := engineDomain(engine)
	if domain == "" {
		return 0, fmt.Errorf("unsupported browser engine: %s", engine)
	}
	result, err := s.bridge.Service.Submit(ctx, screenshot.BridgeTask{
		RequestID: fmt.Sprintf("cookie_sync_%s_%d", engine, time.Now().UnixNano()),
		URL:       engineCredentialURL(engine),
		BatchID:   "cookie_sync",
		Action:    "get_browser_credentials",
		Timeout:   8 * time.Second,
	})
	if err != nil {
		return 0, fmt.Errorf("read extension cookies for %s: %w", engine, err)
	}
	if !result.Success {
		return 0, fmt.Errorf("read extension credentials for %s failed: code=%s detail=%s", engine, strings.TrimSpace(result.ErrorCode), strings.TrimSpace(result.Error))
	}
	cookies, cookieErr := bridgeCookiesFromResult(engine, result)
	if cookieErr == nil {
		// Read-only credential sync must not rewrite config.yaml on every poll.
		// Persist only when the synced cookies actually differ from the current ones.
		cfg := s.currentConfig()
		if cfg == nil || !cookiesEqual(engineCookies(cfg, engine), cookies) {
			committed, updateErr := s.updateConfig(func(cfg *config.Config) error {
				setEngineCookies(cfg, engine, cookies)
				return nil
			})
			if updateErr != nil {
				return 0, fmt.Errorf("persist extension cookies for %s: %w", engine, updateErr)
			}
			s.screenshotMgr.SetCookies(engine, convertConfigCookies(engineCookies(committed, engine)))
		}
	}
	storageCount := 0
	if data := result.StructuredCollectedData; data != nil && data.Storage != nil {
		storage := screenshot.BrowserStorage{Local: data.Storage.Local, Session: data.Storage.Session}
		storageCount = len(storage.Local) + len(storage.Session)
		s.screenshotMgr.SetBrowserStorage(engine, storage)
	}
	if cookieErr != nil && storageCount == 0 {
		return 0, cookieErr
	}
	return len(cookies), nil
}

// cookiesEqual reports whether two cookie lists are identical, so read-only
// credential sync does not rewrite config.yaml when nothing changed.
func cookiesEqual(left, right []config.Cookie) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func engineCredentialURL(engine string) string {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "fofa":
		return "https://fofa.info/"
	case "hunter":
		return "https://hunter.qianxin.com/"
	case "zoomeye":
		return "https://www.zoomeye.org/"
	case "quake":
		return "https://quake.360.net/"
	case "shodan":
		return "https://www.shodan.io/"
	case "censys":
		return "https://platform.censys.io/"
	case "daydaymap":
		return "https://www.daydaymap.com/home"
	default:
		return ""
	}
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
// GET /api/v1/cookies/login-status?query=...
// Detects: CDP connected, Extension paired, per-engine login wall detection.
func (s *Server) handleCookieLoginStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	cdpConnected, extPaired := s.detectSessionChannels(r.Context())
	engines := []string{"fofa", "hunter", "zoomeye", "quake", "shodan", "censys", "daydaymap"} // 全部引擎
	results := s.checkEngineLoginStatuses(r.Context(), engines, cdpConnected, extPaired)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"cdp_connected": cdpConnected,
		"ext_paired":    extPaired,
		"engines":       results,
	})
}

// detectSessionChannels detects CDP and Extension session availability.
func (s *Server) detectSessionChannels(ctx context.Context) (cdpConnected, extPaired bool) {
	if s.screenshotMgr != nil && s.screenshotMgr.RemoteDebugURL() != "" {
		baseURL := s.resolveCDPURL()
		if online, _, _ := s.checkCDPStatus(ctx, baseURL); online {
			cdpConnected = true
		}
	}
	if s.bridge != nil && s.bridge.Service != nil {
		extPaired = s.activeBridgeLiveTokens() > 0
	}
	return
}

// EngineLoginStatus is the typed response for engine login status check.
type EngineLoginStatus struct {
	Engine       string `json:"engine"`
	LoggedIn     bool   `json:"logged_in"`
	Reason       string `json:"reason"`
	Title        string `json:"title,omitempty"`
	LoginURL     string `json:"login_url,omitempty"`
	CDPConnected bool   `json:"cdp_connected"`
	ExtPaired    bool   `json:"ext_paired"`
}

// checkEngineLoginStatuses checks login status for each engine.
func (s *Server) checkEngineLoginStatuses(ctx context.Context, engines []string, cdpConnected, extPaired bool) []EngineLoginStatus {
	results := make([]EngineLoginStatus, 0, len(engines))
	for _, engine := range engines {
		results = append(results, s.checkSingleEngineLogin(ctx, engine, cdpConnected, extPaired))
	}
	return results
}

// checkSingleEngineLogin checks login status for a single engine.
func (s *Server) checkSingleEngineLogin(ctx context.Context, engine string, cdpConnected, extPaired bool) EngineLoginStatus {
	loginURL := ""
	if s.screenshotMgr != nil {
		loginURL = s.screenshotMgr.EngineLoginURL(engine)
	}
	cookieSet := s.engineCookieConfigured(engine)

	base := EngineLoginStatus{
		Engine:       engine,
		LoginURL:     loginURL,
		CDPConnected: cdpConnected,
		ExtPaired:    extPaired,
	}

	// A configured browser cookie set is a handoff candidate, not proof that
	// the page is authenticated; the channel checks below still verify it.
	if cookieSet {
		base.Reason = "cookie_configured"
	}

	if !cdpConnected && !extPaired {
		reason := "no_session"
		if cookieSet {
			reason = "cookie_configured"
		}
		base.Reason = reason
		return base
	}

	loggedIn, reason := false, "no_session"
	if cdpConnected && s.screenshotMgr != nil {
		cdpCtx, cdpCancel := context.WithTimeout(ctx, 8*time.Second)
		loggedIn, reason = s.detectLoginViaCDP(cdpCtx, engine, cookieSet)
		cdpCancel()
	}
	if !loggedIn && extPaired {
		if cdpConnected {
			syncCtx, syncCancel := context.WithTimeout(ctx, 10*time.Second)
			_, syncErr := s.syncExtensionCookiesToCDP(syncCtx, engine)
			syncCancel()
			if syncErr == nil {
				cdpCtx, cdpCancel := context.WithTimeout(ctx, 8*time.Second)
				loggedIn, reason = s.detectLoginViaCDP(cdpCtx, engine, true)
				cdpCancel()
			}
		}
	}
	if !loggedIn && extPaired {
		extCtx, extCancel := context.WithTimeout(ctx, 8*time.Second)
		loggedIn, reason = s.detectLoginViaExtension(extCtx, engine, cookieSet)
		extCancel()
	}

	// 扩展已连接但 cookie 检测未确认登录 → 标记为 "ext_connected"（非 "no_session"）
	if !loggedIn && extPaired && reason != "browser_session" {
		reason = "ext_connected"
	}
	// CDP 已连接但 cookie 检测未确认登录 → 标记为 "cdp_connected"
	if !loggedIn && cdpConnected && reason != "browser_session" && reason != "ext_connected" {
		reason = "cdp_connected"
	}

	base.LoggedIn = loggedIn
	base.Reason = reason
	return base
}

// engineCookieConfigured checks if cookies are configured for the given engine.
func (s *Server) engineCookieConfigured(engine string) bool {
	cfg := s.currentConfig()
	if cfg == nil {
		return false
	}
	switch engine {
	case "fofa":
		return hasCookies(cfg.Engines.Fofa.Cookies)
	case "hunter":
		return hasCookies(cfg.Engines.Hunter.Cookies)
	case "quake":
		return hasCookies(cfg.Engines.Quake.Cookies)
	case "zoomeye":
		return hasCookies(cfg.Engines.Zoomeye.Cookies)
	case "shodan":
		return hasCookies(cfg.Engines.Shodan.Cookies)
	case "censys":
		return hasCookies(cfg.Engines.Censys.Cookies)
	case "daydaymap":
		return hasCookies(cfg.Engines.Daydaymap.Cookies)
	}
	return false
}
