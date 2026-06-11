package screenshot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	allocCtx, allocCancel, err := m.newAllocator(ctx)
	if err != nil {
		return nil, err
	}
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	if err := chromedp.Run(browserCtx, chromedp.Navigate(searchURL)); err != nil {
		return nil, fmt.Errorf("navigate to search URL failed: %w", err)
	}
	if err := chromedp.Run(browserCtx, chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
		logger.Warnf("wait for body failed on %s: %v", engine, err)
	}
	if err := chromedp.Run(browserCtx, chromedp.Sleep(3*time.Second)); err != nil {
		return nil, err
	}

	sel := getSelectors(engine)
	var extracted string
	if sel != nil && sel.ExtractJS != "" {
		if err := chromedp.Run(browserCtx, chromedp.Evaluate(sel.ExtractJS, &extracted)); err != nil {
			logger.Warnf("engine-specific extraction failed for %s: %v", engine, err)
		}
	}

	title := ""
	if err := chromedp.Run(browserCtx, chromedp.Title(&title)); err != nil {
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

func collectViaNetworkOnContext(browserCtx context.Context, engine, query string) (*collection.CollectResult, chan struct{}) {
	engineKey := strings.ToLower(engine)
	apiConfig, ok := l1SearchAPIs[engineKey]
	if !ok {
		return nil, nil
	}

	var mu sync.Mutex
	captured := &networkResponse{}
	result := &collection.CollectResult{}
	respCh := make(chan struct{}, 1)

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
					*result = collection.CollectResult{
						Engine: engine, Query: query, RawURL: resp.URL,
						Title: fmt.Sprintf("L1 Network: %s", engine), Timestamp: time.Now().Unix(),
						Assets: assets, Total: total, HasMore: len(assets) < total,
					}
					select {
					case respCh <- struct{}{}:
					default:
					}
				}()
			}
		}
	})
	return result, respCh
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

	allocCtx, allocCancel, err := m.newAllocator(ctx)
	if err != nil {
		return nil, "", err
	}
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	l1Result, l1Ch := collectViaNetworkOnContext(browserCtx, engine, query)
	if l1Ch != nil {
		if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
			logger.Warnf("enable network failed on %s: %v", engine, err)
		}
	}

	// 单次导航
	if err := chromedp.Run(browserCtx, chromedp.Navigate(searchURL)); err != nil {
		return nil, "", fmt.Errorf("navigate to search URL failed: %w", err)
	}
	if err := chromedp.Run(browserCtx, chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
		logger.Warnf("wait for body failed on %s: %v", engine, err)
	}
	if err := chromedp.Run(browserCtx, chromedp.Sleep(3*time.Second)); err != nil {
		return nil, "", err
	}

	// 采集数据
	sel := getSelectors(engine)
	var extracted string
	if sel != nil && sel.ExtractJS != "" {
		if err := chromedp.Run(browserCtx, chromedp.Evaluate(sel.ExtractJS, &extracted)); err != nil {
			logger.Warnf("engine-specific extraction failed for %s: %v", engine, err)
		}
	}
	title := ""
	if err := chromedp.Run(browserCtx, chromedp.Title(&title)); err != nil {
		logger.Warnf("failed to get page title: %v", err)
	}

	collectResult := collection.CollectResult{
		Engine: engine, Query: query, RawURL: searchURL,
		Title: title, Timestamp: time.Now().Unix(),
	}
	l1Succeeded := false
	if l1Ch != nil {
		select {
		case <-l1Ch:
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
		}
		if l1Result != nil && len(l1Result.Assets) > 0 {
			collectResult = *l1Result
			l1Succeeded = true
		}
	}
	if !l1Succeeded && extracted != "" {
		var jsResult struct {
			Assets  []map[string]interface{} `json:"assets"`
			Total   int                      `json:"total"`
			HasMore bool                     `json:"hasMore"`
		}
		if err := json.Unmarshal([]byte(extracted), &jsResult); err != nil {
			logger.Warnf("failed to parse extracted JSON: %v", err)
		} else {
			collectResult.Assets = collection.ParseExtractedAssets(jsResult.Assets, engine)
			collectResult.Total = jsResult.Total
			collectResult.HasMore = jsResult.HasMore
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
	if err := chromedp.Run(browserCtx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return []collection.CollectResult{collectResult}, "", fmt.Errorf("screenshot failed: %w", err)
	}
	if err := os.WriteFile(screenshotPath, buf, 0600); err != nil {
		return []collection.CollectResult{collectResult}, "", fmt.Errorf("save screenshot failed: %w", err)
	}

	return []collection.CollectResult{collectResult}, screenshotPath, nil
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
