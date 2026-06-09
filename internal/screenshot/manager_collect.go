package screenshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/logger"
)

// CollectSearchEngineResult opens a search engine result page and extracts
// structured asset data from it using per-engine DOM selectors.
func (m *Manager) CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]collection.CollectResult, error) {
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
	if extracted != "" {
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

