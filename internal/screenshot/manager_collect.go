package screenshot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/logger"
)

// CollectSearchEngineResult opens a search engine result page and extracts
// structured asset data. Tries L1 Network first (for supported engines),
// falls back to L3 DOM extraction.
func (m *Manager) CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]collection.CollectResult, error) {
	// L1: 尝试 Network 拦截（仅支持 SPA 引擎）
	if IsL1Supported(engine) {
		results, err := m.CollectViaNetwork(ctx, engine, query, queryID)
		if err == nil && len(results) > 0 && len(results[0].Assets) > 0 {
			logger.Infof("L1 network collection succeeded for %s: %d assets", engine, len(results[0].Assets))
			return results, nil
		}
		logger.Warnf("L1 network collection failed for %s, falling back to L3 DOM: %v", engine, err)
	}

	// L3: DOM 解析（所有引擎）
	return m.collectViaDOM(ctx, engine, query, queryID)
}

// collectViaDOM 通过 L3 DOM 解析采集搜索结果
func (m *Manager) collectViaDOM(ctx context.Context, engine, query, queryID string) ([]collection.CollectResult, error) {
	searchURL := m.BuildSearchEngineURL(engine, query)
	if searchURL == "" {
		return nil, fmt.Errorf("unsupported engine: %s", engine)
	}

	collectTimeout := m.timeout
	if collectTimeout <= 0 || collectTimeout > 60*time.Second {
		collectTimeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, collectTimeout)
	defer cancel()

	session, err := m.newGuardedBrowserSession(ctx, searchURL, "")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	if cookies := m.GetCookies(engine); len(cookies) > 0 {
		if err := session.Run(setCookieActions(cookies, searchURL)...); err != nil {
			logger.Warnf("inject cookies failed for %s: %v", engine, err)
		}
	}

	if err := session.Run(chromedp.Navigate(searchURL)); err != nil {
		return nil, fmt.Errorf("navigate to search URL failed: %w", err)
	}
	if err := applyBrowserStorage(session, m.GetBrowserStorage(engine), searchURL); err != nil {
		return nil, fmt.Errorf("inject browser storage for %s: %w", engine, err)
	}
	if err := session.Run(chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
		logger.Warnf("wait for body failed on %s: %v", engine, err)
	}
	if err := prepareStatefulSearchPage(session, engine, query); err != nil {
		return nil, err
	}
	if err := session.Run(chromedp.Sleep(3 * time.Second)); err != nil {
		return nil, err
	}

	sel := getSelectors(engine)
	var extracted string
	if sel != nil && sel.ExtractJS != "" {
		if err := session.Run(chromedp.Evaluate(sel.ExtractJS, &extracted)); err != nil {
			logger.Warnf("engine-specific extraction failed for %s: %v", engine, err)
		}
	}

	title := ""
	if err := session.Run(chromedp.Title(&title)); err != nil {
		logger.Warnf("failed to get page title: %v", err)
	}

	result := collection.CollectResult{
		Engine: engine, Query: query, RawURL: searchURL,
		Title: title, Timestamp: time.Now().Unix(),
	}

	if extracted != "" {
		var jsResult struct {
			Assets  []map[string]interface{} `json:"assets"`
			Total   int                      `json:"total"`
			HasMore bool                     `json:"hasMore"`
		}
		if err := json.Unmarshal([]byte(extracted), &jsResult); err != nil {
			logger.Warnf("failed to parse extracted JSON: %v", err)
		} else {
			result.Assets = collection.ParseExtractedAssets(jsResult.Assets, engine)
			result.Total = jsResult.Total
			result.HasMore = jsResult.HasMore
		}
	}
	return []collection.CollectResult{result}, nil
}

func collectViaNetworkOnContext(browserCtx context.Context, engine, query string) chan collection.CollectResult {
	engineKey := strings.ToLower(engine)
	apiConfig, ok := l1SearchAPIs[engineKey]
	if !ok {
		return nil
	}

	var mu sync.Mutex
	captured := &networkResponse{}
	// FINDING-003 修复：仅通过 channel 传值，不共享 *result 指针。
	// goroutine 本地构造值后 send，调用方 receive 得到 happens-before；
	// 函数不返回可变指针，杜绝调用方在超时窗口内无同步读取 goroutine 的写入。
	resultCh := make(chan collection.CollectResult, 1)

	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			if strings.Contains(e.Response.URL, apiConfig.URLPattern) {
				mu.Lock()
				if captured.URL == "" {
					captured.URL = e.Response.URL
					captured.RequestID = e.RequestID
					captured.StatusCode = int(e.Response.Status)
				}
				mu.Unlock()
			}
		case *network.EventLoadingFinished:
			mu.Lock()
			needFetch := captured.URL != "" && captured.Body == nil && e.RequestID == captured.RequestID
			reqID := captured.RequestID
			mu.Unlock()
			if needFetch {
				go func() {
					var body []byte
					if err := chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
						b, err := network.GetResponseBody(reqID).Do(ctx)
						if err != nil {
							return err
						}
						body = b
						return nil
					})); err != nil {
						logger.Warnf("L1: failed to get response body: %v", err)
						return
					}

					mu.Lock()
					captured.Body = body
					resp := *captured
					mu.Unlock()

					if resp.StatusCode != http.StatusOK {
						logger.Warnf("L1: API returned status %d", resp.StatusCode)
						return
					}
					assets, total, err := apiConfig.ParseResponse(resp.Body)
					if err != nil {
						logger.Warnf("L1: failed to parse response: %v", err)
						return
					}
					res := collection.CollectResult{
						Engine: engine, Query: query, RawURL: resp.URL,
						Title: fmt.Sprintf("L1 Network: %s", engine), Timestamp: time.Now().Unix(),
						Assets: assets, Total: total, HasMore: len(assets) < total,
					}
					// 非阻塞传值；调用方可能已超时（channel 缓冲 1，不会阻塞本 goroutine）。
					select {
					case resultCh <- res:
					default:
					}
				}()
			}
		}
	})
	return resultCh
}

// CollectAndCaptureSearchEngineResult 在单次导航中同时完成数据采集和截图。
// 共享同一个 Chrome context，避免重复导航到同一 URL。
func (m *Manager) CollectAndCaptureSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]collection.CollectResult, string, error) {
	searchURL := m.BuildSearchEngineURL(engine, query)
	if searchURL == "" {
		return nil, "", fmt.Errorf("unsupported engine: %s", engine)
	}

	timeout := m.timeout
	if timeout <= 0 || timeout > 60*time.Second {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	session, err := m.newGuardedBrowserSession(ctx, searchURL, "")
	if err != nil {
		return nil, "", err
	}
	defer session.Close()
	browserCtx := session.Context()

	if cookies := m.GetCookies(engine); len(cookies) > 0 {
		if err := session.Run(setCookieActions(cookies, searchURL)...); err != nil {
			logger.Warnf("inject cookies failed for %s: %v", engine, err)
		}
	}

	// l1Result 仅由 channel receive 分支赋值，超时/ctx.Done 分支保持 nil，
	// 避免读取 goroutine 仍在写入的数据（FINDING-003 修复）。
	l1Ch := collectViaNetworkOnContext(browserCtx, engine, query)
	var l1Result *collection.CollectResult
	if l1Ch != nil {
		if enableErr := session.Run(network.Enable()); enableErr != nil {
			logger.Warnf("enable network failed on %s: %v", engine, enableErr)
		}
	}

	// 单次导航
	if navErr := session.Run(chromedp.Navigate(searchURL)); navErr != nil {
		return nil, "", fmt.Errorf("navigate to search URL failed: %w", navErr)
	}
	if waitErr := session.Run(chromedp.WaitReady("body", chromedp.ByQuery)); waitErr != nil {
		logger.Warnf("wait for body failed on %s: %v", engine, waitErr)
	}
	if storageErr := applyBrowserStorage(session, m.GetBrowserStorage(engine), searchURL); storageErr != nil {
		return nil, "", fmt.Errorf("inject browser storage for %s: %w", engine, storageErr)
	}
	if prepareErr := prepareStatefulSearchPage(session, engine, query); prepareErr != nil {
		return nil, "", prepareErr
	}
	if sleepErr := session.Run(chromedp.Sleep(3 * time.Second)); sleepErr != nil {
		return nil, "", sleepErr
	}
	if strings.EqualFold(strings.TrimSpace(engine), "daydaymap") {
		const rowsReady = `(() => {
			const selectors = ["[class*='result-item']", "[class*='result-card']", "[class*='result-list'] > div", "[class*='result'] > div", ".el-table__row", "table tbody tr", ".list_content > div"];
			return selectors.some((selector) => document.querySelectorAll(selector).length > 0);
		})()`
		if waitRowsErr := session.Run(chromedp.Poll(rowsReady, nil, chromedp.WithPollingInterval(500*time.Millisecond), chromedp.WithPollingTimeout(40*time.Second))); waitRowsErr != nil {
			logger.Warnf("DayDayMap result rows did not become ready before extraction: %v", waitRowsErr)
		}
	}

	// 采集数据
	sel := getSelectors(engine)
	var extracted string
	if sel != nil && sel.ExtractJS != "" {
		if evalErr := session.Run(chromedp.Evaluate(sel.ExtractJS, &extracted)); evalErr != nil {
			logger.Warnf("engine-specific extraction failed for %s: %v", engine, evalErr)
		}
	}
	title := ""
	if titleErr := session.Run(chromedp.Title(&title)); titleErr != nil {
		logger.Warnf("failed to get page title: %v", titleErr)
	}
	bodyText := ""
	if bodyErr := session.Run(chromedp.Text("body", &bodyText, chromedp.ByQuery)); bodyErr != nil {
		logger.Warnf("failed to read page body for challenge detection: %v", bodyErr)
	}

	collectResult := collection.CollectResult{
		Engine: engine, Query: query, RawURL: searchURL,
		Title: title, Timestamp: time.Now().Unix(),
	}
	if detectBrowserChallenge(title, bodyText) {
		collectResult.BrowserChallenge = true
		collectResult.ExtractionError = "browser_challenge"
	}
	l1Succeeded := false
	if l1Ch != nil {
		l1Wait := 2 * time.Second
		if strings.EqualFold(strings.TrimSpace(engine), "daydaymap") {
			l1Wait = 10 * time.Second
		}
		select {
		case res := <-l1Ch:
			l1Result = &res
		case <-time.After(l1Wait):
		case <-ctx.Done():
		}
		if l1Result != nil && len(l1Result.Assets) > 0 {
			collectResult = *l1Result
			l1Succeeded = true
		}
	}
	if !l1Succeeded && extracted != "" {
		var jsResult struct {
			Assets          []map[string]interface{} `json:"assets"`
			Total           int                      `json:"total"`
			HasMore         bool                     `json:"hasMore"`
			RowSelectorUsed string                   `json:"rowSelectorUsed"`
			RowsFound       int                      `json:"rowsFound"`
			ExtractionError string                   `json:"extractionError"`
		}
		if unmarshalErr := json.Unmarshal([]byte(extracted), &jsResult); unmarshalErr != nil {
			logger.Warnf("failed to parse extracted JSON: %v", unmarshalErr)
		} else {
			collectResult.Assets = collection.ParseExtractedAssets(jsResult.Assets, engine)
			collectResult.Total = jsResult.Total
			collectResult.HasMore = jsResult.HasMore
			collectResult.ExtractionMethod = "dom"
			collectResult.RowSelectorUsed = jsResult.RowSelectorUsed
			collectResult.RowsFound = jsResult.RowsFound
			collectResult.ExtractionError = jsResult.ExtractionError
			if collectResult.BrowserChallenge {
				collectResult.ExtractionError = "browser_challenge"
			}
		}
	}

	// 截图（复用同一页面）
	_, searchEngineDir, _, err := m.CreateQueryDirectory(queryID)
	if err != nil {
		return []collection.CollectResult{collectResult}, "", err
	}
	filename := m.generateSearchEngineFilename(engine, query)
	screenshotPath := filepath.Join(searchEngineDir, filename)

	var buf []byte
	if err := captureScreenshotWithFallback(browserCtx, &buf); err != nil {
		return []collection.CollectResult{collectResult}, "", fmt.Errorf("screenshot failed: %w", err)
	}
	if err := session.interceptor.Err(); err != nil {
		return []collection.CollectResult{collectResult}, "", err
	}
	if err := os.WriteFile(screenshotPath, buf, 0600); err != nil {
		return []collection.CollectResult{collectResult}, "", fmt.Errorf("save screenshot failed: %w", err)
	}

	return []collection.CollectResult{collectResult}, screenshotPath, nil
}

func prepareStatefulSearchPage(session *guardedBrowserSession, engine, query string) error {
	if session == nil || !strings.EqualFold(strings.TrimSpace(engine), "daydaymap") {
		return nil
	}
	var currentURL string
	if err := session.Run(chromedp.Location(&currentURL)); err == nil {
		if parsed, parseErr := url.Parse(currentURL); parseErr == nil && strings.Contains(parsed.Path, "/searchResult") && parsed.Query().Get("keyword") != "" {
			return nil
		}
	}
	encodedQuery, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("marshal DayDayMap browser query: %w", err)
	}
	script := fmt.Sprintf(`(() => {
		const nativeQuery = %s;
		const inputs = Array.from(document.querySelectorAll('input'));
		const input = inputs.find((element) => {
			const placeholder = (element.getAttribute('placeholder') || '').toLowerCase();
			return placeholder.includes('search') || placeholder.includes('检索') || placeholder.includes('关键词');
		}) || inputs.find((element) => element.type === 'text');
		if (!input) return 'search_input_missing';
		const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value');
		if (descriptor && descriptor.set) descriptor.set.call(input, nativeQuery); else input.value = nativeQuery;
		input.dispatchEvent(new InputEvent('input', {bubbles: true, inputType: 'insertText', data: nativeQuery}));
		input.dispatchEvent(new Event('change', {bubbles: true}));
		return 'ok';
	})()`, string(encodedQuery))
	var result string
	if err := session.Run(chromedp.Evaluate(script, &result)); err != nil {
		return fmt.Errorf("prepare DayDayMap search: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("prepare DayDayMap search: %s", result)
	}
	triggerScript := `(() => {
		const input = Array.from(document.querySelectorAll('input')).find((element) => ((element.getAttribute('placeholder') || '').toLowerCase()).includes('search'));
		const button = document.querySelector('.search-btn');
		if (button) button.click();
		else if (input && input.closest('form') && input.closest('form').requestSubmit) input.closest('form').requestSubmit();
		else if (input) input.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true}));
		else return 'search_trigger_missing';
		return 'ok';
	})()`
	var triggerResult string
	if err := session.Run(chromedp.Sleep(600*time.Millisecond), chromedp.Evaluate(triggerScript, &triggerResult)); err != nil {
		return fmt.Errorf("trigger DayDayMap search: %w", err)
	}
	if triggerResult != "ok" {
		return fmt.Errorf("trigger DayDayMap search: %s", triggerResult)
	}
	routeScript := `(() => {
		if (!location.pathname.includes('/searchResult')) {
			history.pushState({}, '', '/searchResult');
			window.dispatchEvent(new PopStateEvent('popstate'));
		}
		return location.pathname;
	})()`
	if err := session.Run(chromedp.Sleep(300*time.Millisecond), chromedp.Evaluate(routeScript, nil)); err != nil {
		return fmt.Errorf("route DayDayMap search: %w", err)
	}
	if err := session.Run(chromedp.Poll(`location.pathname.includes('/searchResult')`, nil, chromedp.WithPollingTimeout(15*time.Second))); err != nil {
		return fmt.Errorf("wait for DayDayMap search results: %w", err)
	}
	return nil
}

func applyBrowserStorage(session *guardedBrowserSession, storage BrowserStorage, targetURL string) error {
	if session == nil || (len(storage.Local) == 0 && len(storage.Session) == 0) {
		return nil
	}
	payload, err := json.Marshal(map[string]map[string]string{"local": storage.Local, "session": storage.Session})
	if err != nil {
		return fmt.Errorf("marshal browser storage: %w", err)
	}
	script := fmt.Sprintf(`((state) => {
		for (const [key, value] of Object.entries(state.local || {})) localStorage.setItem(key, value);
		for (const [key, value] of Object.entries(state.session || {})) sessionStorage.setItem(key, value);
		return true;
	})(%s)`, payload)
	var applied bool
	if err := session.Run(chromedp.Evaluate(script, &applied)); err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("browser rejected storage state")
	}
	return session.Run(chromedp.Navigate(targetURL), chromedp.WaitReady("body", chromedp.ByQuery))
}

// GetScreenshotDirectory 获取截图根目录
func (m *Manager) GetScreenshotDirectory() string {
	return m.baseDir
}

// SetChromePath 设置Chrome路径
func (m *Manager) SetChromePath(path string) {
	m.chromePath = path
}

// SetRemoteDebugURL 设置远程调试地址
func (m *Manager) SetRemoteDebugURL(remoteURL string) {
	m.remoteDebugURL = strings.TrimSpace(remoteURL)
}

// RemoteDebugURL returns the current remote debug URL.
func (m *Manager) RemoteDebugURL() string {
	return m.remoteDebugURL
}

// SetProxyServer 设置浏览器代理地址
func (m *Manager) SetProxyServer(proxy string) {
	m.proxyServer = strings.TrimSpace(proxy)
}
