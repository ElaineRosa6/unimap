package screenshot

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
)

// CollectSearchEngineResult opens a search engine result page and extracts
// structured asset data from it using per-engine DOM selectors.
func (m *Manager) CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]CollectResult, error) {
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

	// Navigate to the search URL
	if err := chromedp.Run(browserCtx, chromedp.Navigate(searchURL)); err != nil {
		return nil, fmt.Errorf("navigate to search URL failed: %w", err)
	}

	// Wait for the page body to be ready
	if err := chromedp.Run(browserCtx, chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
		// Page may be slow for some engines; continue anyway
		logger.Warnf("wait for body failed on %s: %v", engine, err)
	}

	// Wait extra time for SPA rendering
	if err := chromedp.Run(browserCtx, chromedp.Sleep(3*time.Second)); err != nil {
		return nil, err
	}

	// Try engine-specific JS extraction
	sel := getSelectors(engine)
	var extracted string
	if sel != nil && sel.ExtractJS != "" {
		if err := chromedp.Run(browserCtx, chromedp.Evaluate(sel.ExtractJS, &extracted)); err != nil {
			logger.Warnf("engine-specific extraction failed for %s: %v", engine, err)
		}
	}

	// Extract page title as fallback
	title := ""
	if err := chromedp.Run(browserCtx, chromedp.Title(&title)); err != nil {
		logger.Warnf("failed to get page title: %v", err)
	}

	var assets []model.UnifiedAsset
	total := 0
	hasMore := false

	if extracted != "" {
		// Parse the JSON result from JS extraction
		var jsResult struct {
			Assets  []map[string]interface{} `json:"assets"`
			Total   int                      `json:"total"`
			HasMore bool                     `json:"hasMore"`
		}
		if err := json.Unmarshal([]byte(extracted), &jsResult); err != nil {
			logger.Warnf("failed to parse extracted JSON: %v", err)
		} else {
			assets = m.parseExtractedAssets(jsResult.Assets, engine)
			total = jsResult.Total
			hasMore = jsResult.HasMore
		}
	}

	result := CollectResult{
		Engine:    engine,
		Query:     query,
		RawURL:    searchURL,
		Title:     title,
		Timestamp: time.Now().Unix(),
		Assets:    assets,
		Total:     total,
		HasMore:   hasMore,
	}
	return []CollectResult{result}, nil
}

// parseExtractedAssets converts raw JS-extracted maps into UnifiedAsset structs.
func (m *Manager) parseExtractedAssets(raw []map[string]interface{}, engine string) []model.UnifiedAsset {
	assets := make([]model.UnifiedAsset, 0, len(raw))
	for _, row := range raw {
		a := model.UnifiedAsset{Source: engine}
		if v, ok := row["ip"].(string); ok {
			a.IP = v
		}
		if v, ok := row["port"].(float64); ok {
			a.Port = int(v)
		} else if v, ok := row["port"].(string); ok {
			if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				a.Port = p
			}
		}
		if v, ok := row["protocol"].(string); ok {
			a.Protocol = v
		}
		if v, ok := row["host"].(string); ok {
			a.Host = v
		}
		if v, ok := row["url"].(string); ok {
			a.URL = v
		}
		if v, ok := row["title"].(string); ok {
			a.Title = v
		}
		if v, ok := row["server"].(string); ok {
			a.Server = v
		}
		if v, ok := row["country"].(string); ok {
			a.CountryCode = v
		}
		if v, ok := row["region"].(string); ok {
			a.Region = v
		}
		if v, ok := row["city"].(string); ok {
			a.City = v
		}
		if v, ok := row["asn"].(string); ok {
			a.ASN = v
		}
		if v, ok := row["org"].(string); ok {
			a.Org = v
		}
		if v, ok := row["isp"].(string); ok {
			a.ISP = v
		}
		if v, ok := row["body_snippet"].(string); ok {
			a.BodySnippet = v
		}
		if v, ok := row["status_code"].(float64); ok {
			a.StatusCode = int(v)
		} else if v, ok := row["status_code"].(string); ok {
			if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				a.StatusCode = p
			}
		}
		if v, ok := row["source"].(string); ok {
			a.Source = v
		}
		// Skip empty assets (at least IP or host should be present)
		if a.IP == "" && a.Host == "" && a.URL == "" {
			continue
		}
		assets = append(assets, a)
	}
	return assets
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

