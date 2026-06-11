package screenshot

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/metrics"
	"github.com/unimap/project/internal/model"
)

// ScreenshotType 截图类型
type ScreenshotType string

const (
	// ScreenshotTypeSearchEngine 搜索引擎结果页面截图
	ScreenshotTypeSearchEngine ScreenshotType = "search-engine-results"
	// ScreenshotTypeTargetWebsite 目标网站截图
	ScreenshotTypeTargetWebsite ScreenshotType = "target-websites"
	// ScreenshotTypeBatchUpload 批量上传URL截图
	ScreenshotTypeBatchUpload ScreenshotType = "batch-upload"
)

// EngineWebURL 搜索引擎Web界面URL模板
type EngineWebURL struct {
	Name      string
	ResultURL string // 搜索结果页面URL模板
}

// Manager 截图管理器
type Manager struct {
	baseDir        string
	chromePath     string
	proxyServer    string
	userDataDir    string
	profileDir     string
	remoteDebugURL string
	headless       bool
	cookies        map[string][]Cookie // 各引擎的Cookie
	cookiesMutex   sync.RWMutex
	timeout        time.Duration
	windowWidth    int
	windowHeight   int
	waitTime       time.Duration // 页面加载后等待时间
}

// Cookie Cookie信息
type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	HTTPOnly bool
	Secure   bool
}

// Config 截图管理器配置
type Config struct {
	BaseDir        string
	ChromePath     string
	ProxyServer    string
	UserDataDir    string
	ProfileDir     string
	RemoteDebugURL string
	Headless       bool
	Timeout        time.Duration
	WindowWidth    int
	WindowHeight   int
	WaitTime       time.Duration
}

// NewManager 创建截图管理器
func NewManager(cfg Config) *Manager {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.WindowWidth == 0 {
		cfg.WindowWidth = 1365
	}
	if cfg.WindowHeight == 0 {
		cfg.WindowHeight = 768
	}
	if cfg.WaitTime == 0 {
		cfg.WaitTime = 500 * time.Millisecond
	}

	return &Manager{
		baseDir:        cfg.BaseDir,
		chromePath:     cfg.ChromePath,
		proxyServer:    cfg.ProxyServer,
		userDataDir:    cfg.UserDataDir,
		profileDir:     cfg.ProfileDir,
		remoteDebugURL: cfg.RemoteDebugURL,
		headless:       cfg.Headless,
		cookies:        make(map[string][]Cookie),
		timeout:        cfg.Timeout,
		windowWidth:    cfg.WindowWidth,
		windowHeight:   cfg.WindowHeight,
		waitTime:       cfg.WaitTime,
	}
}

// SetCookies 设置指定引擎的Cookie
func (m *Manager) SetCookies(engine string, cookies []Cookie) {
	m.cookiesMutex.Lock()
	defer m.cookiesMutex.Unlock()
	m.cookies[strings.ToLower(engine)] = cookies
}

// GetCookies 获取指定引擎的Cookie
func (m *Manager) GetCookies(engine string) []Cookie {
	m.cookiesMutex.RLock()
	defer m.cookiesMutex.RUnlock()
	return m.cookies[strings.ToLower(engine)]
}

// CreateQueryDirectory 创建查询目录结构
// 返回: 查询目录路径, 搜索引擎截图目录, 目标网站截图目录, 错误
func (m *Manager) CreateQueryDirectory(queryID string) (string, string, string, error) {
	// 生成目录名: YYYY-MM-DD-{queryID}
	dateStr := time.Now().Format("2006-01-02")
	dirName := fmt.Sprintf("%s-%s", dateStr, queryID)

	queryDir := filepath.Join(m.baseDir, dirName)
	searchEngineDir := filepath.Join(queryDir, string(ScreenshotTypeSearchEngine))
	targetWebsiteDir := filepath.Join(queryDir, string(ScreenshotTypeTargetWebsite))

	// 创建目录
	dirs := []string{queryDir, searchEngineDir, targetWebsiteDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", "", "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return queryDir, searchEngineDir, targetWebsiteDir, nil
}

// CreateBatchUploadDirectory 创建批量上传截图目录
// 返回: 批量上传目录路径, 错误
func (m *Manager) CreateBatchUploadDirectory(batchID string) (string, error) {
	// 生成目录名: batch-YYYY-MM-DD-{batchID}
	dateStr := time.Now().Format("2006-01-02")
	dirName := fmt.Sprintf("batch-%s-%s", dateStr, batchID)

	batchDir := filepath.Join(m.baseDir, dirName)

	// 创建目录
	if err := os.MkdirAll(batchDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", batchDir, err)
	}

	return batchDir, nil
}

// CaptureScreenshot 截图指定URL
func (m *Manager) CaptureScreenshot(ctx context.Context, targetURL string, cookies []Cookie) ([]byte, error) {
	return m.CaptureScreenshotWithProxy(ctx, targetURL, cookies, "")
}

func (m *Manager) CaptureScreenshotWithProxy(ctx context.Context, targetURL string, cookies []Cookie, proxy string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	allocCtx, allocCancel, err := m.newAllocatorWithProxy(ctx, proxy)
	if err != nil {
		return nil, err
	}
	defer allocCancel()

	ctx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	var buf []byte

	// 构建ChromeDP动作列表
	actions := []chromedp.Action{}

	// 只有在非CDP模式且提供了Cookie时才设置Cookie
	// CDP模式下浏览器已保持登录状态，无需设置Cookie
	if len(cookies) > 0 && !m.isCDPMode() {
		// 需要先导航到目标域名才能设置Cookie，设置后再重新加载页面
		actions = append(actions,
			chromedp.Navigate(targetURL),
			chromedp.ActionFunc(func(ctx context.Context) error {
				for _, cookie := range cookies {
					err := network.SetCookie(cookie.Name, cookie.Value).
						WithDomain(cookie.Domain).
						WithPath(cookie.Path).
						WithHTTPOnly(cookie.HTTPOnly).
						WithSecure(cookie.Secure).
						Do(ctx)
					if err != nil {
						logger.Warnf("Failed to set cookie %s: %v", cookie.Name, err)
					}
				}
				return nil
			}),
			chromedp.Navigate(targetURL),
		)
	} else {
		if m.isCDPMode() && len(cookies) > 0 {
			logger.Infof("Using CDP mode, skipping cookie setup (browser already logged in)")
		}
		actions = append(actions, chromedp.Navigate(targetURL))
	}

	actions = append(actions,
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(m.waitTime),
	)

	// 添加截图动作
	actions = append(actions, chromedp.CaptureScreenshot(&buf))

	if err := chromedp.Run(ctx, actions...); err != nil {
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	return buf, nil
}

// OpenSearchEngineResult 在浏览器中打开搜索引擎结果页但不执行截图。
func (m *Manager) OpenSearchEngineResult(ctx context.Context, engine, query string) (string, error) {
	searchURL := m.BuildSearchEngineURL(engine, query)
	if searchURL == "" {
		return "", fmt.Errorf("unsupported engine: %s", engine)
	}

	openTimeout := m.timeout
	if openTimeout <= 0 || openTimeout > 10*time.Second {
		openTimeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, openTimeout)
	defer cancel()

	allocCtx, allocCancel, err := m.newAllocator(ctx)
	if err != nil {
		return "", err
	}
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	cookies := m.GetCookies(engine)
	actions := []chromedp.Action{}
	if len(cookies) > 0 && !m.isCDPMode() {
		actions = append(actions,
			chromedp.Navigate(searchURL),
			chromedp.ActionFunc(func(ctx context.Context) error {
				for _, cookie := range cookies {
					err := network.SetCookie(cookie.Name, cookie.Value).
						WithDomain(cookie.Domain).
						WithPath(cookie.Path).
						WithHTTPOnly(cookie.HTTPOnly).
						WithSecure(cookie.Secure).
						Do(ctx)
					if err != nil {
						logger.Warnf("Failed to set cookie %s: %v", cookie.Name, err)
					}
				}
				return nil
			}),
			chromedp.Navigate(searchURL),
		)
	} else {
		if m.isCDPMode() && len(cookies) > 0 {
			logger.Infof("Using CDP mode, skipping cookie setup (browser already logged in)")
		}
		actions = append(actions, chromedp.Navigate(searchURL))
	}

	actions = append(actions, chromedp.Sleep(m.waitTime))
	if err := chromedp.Run(browserCtx, actions...); err != nil {
		return "", fmt.Errorf("open search engine result failed: %w", err)
	}

	return searchURL, nil
}

// ValidateSearchEngineResult validates cookie/session behavior for CDP mode only.
// Extension mode should use bridge-side session verification paths.
func (m *Manager) ValidateSearchEngineResult(ctx context.Context, engine, query string, cookies []Cookie) (bool, string, string, error) {
	if strings.TrimSpace(query) == "" {
		return false, "", "empty query", fmt.Errorf("query cannot be empty")
	}

	searchURL := m.BuildSearchEngineURL(engine, query)
	if searchURL == "" {
		return false, "", "unsupported engine", fmt.Errorf("unsupported engine: %s", engine)
	}

	if len(cookies) == 0 && m.userDataDir == "" && m.remoteDebugURL == "" && os.Getenv("UNIMAP_CHROME_REMOTE_DEBUG_URL") == "" {
		return false, "", "cookie not set", nil
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	title := ""
	html := ""
	if err := m.loadPageContent(ctx, searchURL, cookies, &title, &html); err != nil {
		return false, title, "load failed", err
	}

	text := strings.ToLower(html)
	if strings.Contains(text, "login") || strings.Contains(text, "sign in") || strings.Contains(text, "\u767b\u5f55") || strings.Contains(text, "\u8bf7\u767b\u5f55") {
		return false, title, "login required", nil
	}

	if len(html) < 1500 {
		return false, title, "page content too short", nil
	}

	return true, title, "ok", nil
}

// isCDPMode 判断是否使用CDP远程调试模式
func (m *Manager) isCDPMode() bool {
	return m.remoteDebugURL != ""
}

// EngineLoginStatus represents the login status of a search engine.
type EngineLoginStatus struct {
	Engine   string `json:"engine"`
	LoggedIn bool   `json:"logged_in"`
	Reason   string `json:"reason"`    // "browser_session" / "cookie_configured" / "login_required" / "no_session"
	Title    string `json:"title"`     // page title if detected
	LoginURL string `json:"login_url"` // engine homepage for login redirect
	Error    string `json:"error,omitempty"`
}

// CheckEngineLoginStatus checks whether the user is logged in to the given
// search engine by opening its page and looking for login wall keywords.
// When CDP is connected, it uses the same browser session.
// When CDP is NOT connected, it falls back to checking if cookies are configured.
func (m *Manager) CheckEngineLoginStatus(ctx context.Context, engine, query string) (*EngineLoginStatus, error) {
	engine = strings.ToLower(strings.TrimSpace(engine))
	loginURL := m.EngineLoginURL(engine)
	if loginURL == "" {
		return &EngineLoginStatus{Engine: engine, LoggedIn: false, Reason: "unsupported_engine", LoginURL: ""}, nil
	}

	// Build a test search URL
	searchURL := m.BuildSearchEngineURL(engine, query)
	if searchURL == "" {
		searchURL = loginURL
	}

	if m.isCDPMode() {
		// CDP connected → open page in the same browser session, check for login wall
		ctx, cancel := context.WithTimeout(ctx, m.timeout)
		defer cancel()

		allocCtx, allocCancel, err := m.newAllocator(ctx)
		if err != nil {
			return &EngineLoginStatus{Engine: engine, LoggedIn: false, Reason: "no_session", LoginURL: loginURL, Error: err.Error()}, nil
		}
		defer allocCancel()

		ctx, taskCancel := chromedp.NewContext(allocCtx)
		defer taskCancel()

		title := ""
		html := ""
		if err := m.loadPageContent(ctx, searchURL, nil, &title, &html); err != nil {
			return &EngineLoginStatus{Engine: engine, LoggedIn: false, Reason: "load_failed", LoginURL: loginURL, Title: title, Error: err.Error()}, nil
		}

		text := strings.ToLower(html)
		if strings.Contains(text, "login") || strings.Contains(text, "sign in") ||
			strings.Contains(text, "\u767b\u5f55") || strings.Contains(text, "\u8bf7\u767b\u5f55") {
			metrics.IncBrowserLoginRequired(engine)
			return &EngineLoginStatus{Engine: engine, LoggedIn: false, Reason: "login_required", Title: title, LoginURL: loginURL}, nil
		}
		if len(html) < 500 {
			return &EngineLoginStatus{Engine: engine, LoggedIn: false, Reason: "page_too_short", Title: title, LoginURL: loginURL}, nil
		}
		return &EngineLoginStatus{Engine: engine, LoggedIn: true, Reason: "browser_session", Title: title, LoginURL: loginURL}, nil
	}

	// CDP NOT connected → check if cookies are configured
	cookies := m.GetCookies(engine)
	if len(cookies) > 0 {
		return &EngineLoginStatus{Engine: engine, LoggedIn: false, Reason: "cookie_configured", LoginURL: loginURL}, nil
	}
	return &EngineLoginStatus{Engine: engine, LoggedIn: false, Reason: "no_session", LoginURL: loginURL}, nil
}

// EngineLoginURL returns the homepage URL of a search engine for login redirect.
func (m *Manager) EngineLoginURL(engine string) string {
	switch strings.ToLower(engine) {
	case "fofa":
		return model.FOFAOfficialWebURL + "/"
	case "hunter":
		return "https://hunter.qianxin.com/"
	case "quake":
		return "https://quake.360.cn/"
	case "zoomeye":
		return "https://www.zoomeye.org/"
	default:
		return ""
	}
}

// CaptureSearchEngineResult 截图搜索引擎结果页面
func (m *Manager) CaptureSearchEngineResult(ctx context.Context, engine, query string, queryID string) (string, error) {
	return m.CaptureSearchEngineResultWithProxy(ctx, engine, query, queryID, "")
}

func (m *Manager) CaptureSearchEngineResultWithProxy(ctx context.Context, engine, query string, queryID string, proxy string) (string, error) {
	// 构建搜索引擎结果页面URL
	searchURL := m.BuildSearchEngineURL(engine, query)
	if searchURL == "" {
		return "", fmt.Errorf("unsupported engine: %s", engine)
	}

	// 创建目录
	_, searchEngineDir, _, err := m.CreateQueryDirectory(queryID)
	if err != nil {
		return "", err
	}

	// 生成文件名
	filename := m.generateSearchEngineFilename(engine, query)
	filepath := filepath.Join(searchEngineDir, filename)

	// 获取该引擎的Cookie
	cookies := m.GetCookies(engine)

	// 截图
	buf, err := m.CaptureScreenshotWithProxy(ctx, searchURL, cookies, proxy)
	if err != nil {
		return "", fmt.Errorf("failed to capture %s result page: %w", engine, err)
	}

	// 保存文件
	if err := os.WriteFile(filepath, buf, 0600); err != nil {
		return "", fmt.Errorf("failed to save screenshot: %w", err)
	}

	logger.CtxInfof(ctx, "Captured %s result page: %s", engine, filepath)
	return filepath, nil
}

// CaptureTargetWebsite 截图目标网站
func (m *Manager) CaptureTargetWebsite(ctx context.Context, targetURL, ip, port, protocol, queryID string) (string, error) {
	return m.CaptureTargetWebsiteWithProxy(ctx, targetURL, ip, port, protocol, queryID, "")
}

func (m *Manager) CaptureTargetWebsiteWithProxy(ctx context.Context, targetURL, ip, port, protocol, queryID, proxy string) (string, error) {
	// 构建目标URL
	if targetURL == "" {
		if ip == "" {
			return "", fmt.Errorf("target URL or IP is required")
		}
		proto := "http"
		if protocol != "" {
			proto = strings.ToLower(protocol)
		} else if port == "443" {
			proto = "https"
		}
		if port != "" && port != "80" && port != "443" {
			targetURL = fmt.Sprintf("%s://%s:%s", proto, ip, port)
		} else {
			targetURL = fmt.Sprintf("%s://%s", proto, ip)
		}
	}

	// 确保URL有scheme
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "http://" + targetURL
	}

	// 创建目录
	_, _, targetWebsiteDir, err := m.CreateQueryDirectory(queryID)
	if err != nil {
		return "", err
	}

	// 生成文件名
	filename := m.generateTargetWebsiteFilename(ip, port, protocol)
	filepath := filepath.Join(targetWebsiteDir, filename)

	// 截图（目标网站不需要Cookie）
	buf, err := m.CaptureScreenshotWithProxy(ctx, targetURL, nil, proxy)
	if err != nil {
		return "", fmt.Errorf("failed to capture target website: %w", err)
	}

	// 保存文件
	if err := os.WriteFile(filepath, buf, 0600); err != nil {
		return "", fmt.Errorf("failed to save screenshot: %w", err)
	}

	logger.CtxInfof(ctx, "Captured target website: %s", filepath)
	return filepath, nil
}

// BuildSearchEngineURL 构建搜索引擎结果页面URL
func (m *Manager) BuildSearchEngineURL(engine, query string) string {
	// Base64编码查询语句
	b64Query := base64.StdEncoding.EncodeToString([]byte(query))
	encodedB64 := url.QueryEscape(b64Query)
	encodedQuery := url.QueryEscape(query)

	switch strings.ToLower(engine) {
	case "fofa":
		return fmt.Sprintf("%s/result?qbase64=%s", model.FOFAOfficialWebURL, encodedB64)
	case "hunter":
		return fmt.Sprintf("https://hunter.qianxin.com/home/list?search=%s&conditions=", encodedB64)
	case "quake":
		return fmt.Sprintf("https://quake.360.net/quake/#/searchResult?searchVal=%s&selectIndex=quake_service&latest=true", encodedQuery)
	case "zoomeye":
		return fmt.Sprintf("https://www.zoomeye.org/searchResult?q=%s", encodedQuery)
	case "shodan":
		return fmt.Sprintf("https://www.shodan.io/search?query=%s", encodedQuery)
	default:
		return ""
	}
}

// generateSearchEngineFilename 生成搜索引擎截图文件名
func (m *Manager) generateSearchEngineFilename(engine, query string) string {
	// 清理查询语句，用于文件名
	cleanQuery := strings.ReplaceAll(query, " ", "_")
	cleanQuery = strings.ReplaceAll(cleanQuery, "/", "_")
	cleanQuery = strings.ReplaceAll(cleanQuery, "\\", "_")
	cleanQuery = strings.ReplaceAll(cleanQuery, ":", "_")
	cleanQuery = strings.ReplaceAll(cleanQuery, "*", "_")
	cleanQuery = strings.ReplaceAll(cleanQuery, "?", "_")
	cleanQuery = strings.ReplaceAll(cleanQuery, "\"", "_")
	cleanQuery = strings.ReplaceAll(cleanQuery, "<", "_")
	cleanQuery = strings.ReplaceAll(cleanQuery, ">", "_")
	cleanQuery = strings.ReplaceAll(cleanQuery, "|", "_")

	// 限制文件名长度
	if len(cleanQuery) > 50 {
		cleanQuery = cleanQuery[:50]
	}

	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%s_%s_%s.png", engine, cleanQuery, timestamp)
}

// generateTargetWebsiteFilename 生成目标网站截图文件名
func (m *Manager) generateTargetWebsiteFilename(ip, port, protocol string) string {
	proto := protocol
	if proto == "" {
		proto = "http"
	}

	if port == "" {
		return fmt.Sprintf("%s_%s.png", proto, ip)
	}
	return fmt.Sprintf("%s_%s_%s.png", proto, ip, port)
}

// BatchScreenshotResult 批量截图结果
type BatchScreenshotResult struct {
	URL          string         `json:"url"`
	Success      bool           `json:"success"`
	FilePath     string         `json:"file_path,omitempty"`
	Error        string         `json:"error,omitempty"`
	Timestamp    int64          `json:"timestamp"`
	TamperResult *TamperSummary `json:"tamper_result,omitempty"`
}

// TamperSummary 篡改检测摘要
type TamperSummary struct {
	Tampered         bool     `json:"tampered"`
	FullHash         string   `json:"full_hash"`
	BaselineHash     string   `json:"baseline_hash,omitempty"`
	TamperedSegments []string `json:"tampered_segments,omitempty"`
	HasBaseline      bool     `json:"has_baseline"`
}

// CaptureBatchURLs 批量截图URL列表
func (m *Manager) CaptureBatchURLs(ctx context.Context, urls []string, batchID string, concurrency int) ([]BatchScreenshotResult, error) {
	return m.CaptureBatchURLsWithTamper(ctx, urls, batchID, concurrency, false, nil)
}

// CaptureBatchURLsWithProgress 批量截图URL列表，并在单个 URL 完成时回调进度。
func (m *Manager) CaptureBatchURLsWithProgress(ctx context.Context, urls []string, batchID string, concurrency int, onResult func(BatchScreenshotResult)) ([]BatchScreenshotResult, error) {
	return m.captureBatchURLsWithTamper(ctx, urls, batchID, concurrency, false, nil, onResult)
}

// CaptureBatchURLsWithTamper 批量截图URL列表（带篡改检测）
func (m *Manager) CaptureBatchURLsWithTamper(ctx context.Context, urls []string, batchID string, concurrency int, enableTamper bool, tamperDetector interface{}) ([]BatchScreenshotResult, error) {
	return m.captureBatchURLsWithTamper(ctx, urls, batchID, concurrency, enableTamper, tamperDetector, nil)
}

func (m *Manager) captureBatchURLsWithTamper(ctx context.Context, urls []string, batchID string, concurrency int, enableTamper bool, tamperDetector interface{}, onResult func(BatchScreenshotResult)) ([]BatchScreenshotResult, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("no URLs provided")
	}

	if concurrency <= 0 {
		concurrency = 5
	}

	// 创建批量上传目录
	batchDir, err := m.CreateBatchUploadDirectory(batchID)
	if err != nil {
		return nil, err
	}

	results := make([]BatchScreenshotResult, len(urls))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	for i, targetURL := range urls {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := BatchScreenshotResult{
				URL:       url,
				Timestamp: time.Now().Unix(),
			}

			// 标准化URL
			normalizedURL := m.normalizeURL(url)
			if normalizedURL == "" {
				result.Success = false
				result.Error = "invalid URL"
				results[index] = result
				return
			}

			// 生成文件名
			filename := m.generateBatchFilename(url, index)
			filepath := filepath.Join(batchDir, filename)

			// 截图
			buf, err := m.CaptureScreenshot(ctx, normalizedURL, nil)
			if err != nil {
				result.Success = false
				result.Error = err.Error()
				logger.CtxWarnf(ctx, "Failed to capture screenshot for %s: %v", url, err)
			} else {
				// 保存文件
				if err := os.WriteFile(filepath, buf, 0600); err != nil {
					result.Success = false
					result.Error = fmt.Sprintf("failed to save file: %v", err)
				} else {
					result.Success = true
					result.FilePath = filepath
					logger.CtxInfof(ctx, "Captured screenshot for %s: %s", url, filepath)
				}
			}

			results[index] = result
			if onResult != nil {
				onResult(result)
			}
		}(i, targetURL)
	}

	wg.Wait()
	return results, nil
}

// normalizeURL 标准化URL
func (m *Manager) normalizeURL(targetURL string) string {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return ""
	}

	// 添加协议前缀
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "http://" + targetURL
	}

	// 验证URL格式
	u, err := url.Parse(targetURL)
	if err != nil || u.Host == "" {
		return ""
	}

	return targetURL
}

// generateBatchFilename 生成批量截图文件名
func (m *Manager) generateBatchFilename(targetURL string, index int) string {
	// 从URL提取主机名
	u, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Sprintf("url_%03d_%s.png", index, time.Now().Format("20060102_150405"))
	}

	host := u.Hostname()
	if host == "" {
		host = "unknown"
	}

	// 清理主机名中的非法字符
	host = strings.ReplaceAll(host, ":", "_")
	host = strings.ReplaceAll(host, "/", "_")
	host = strings.ReplaceAll(host, "\\", "_")

	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%03d_%s_%s.png", index, host, timestamp)
}
