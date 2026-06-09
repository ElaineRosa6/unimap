package screenshot

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/unimap/project/internal/metrics"
	"github.com/unimap/project/internal/model"
)

// ExtensionProvider implements Provider using the Extension Bridge.
type ExtensionProvider struct {
	bridge *BridgeService
	mgr    *Manager
}

// NewExtensionProvider creates a Provider that routes through the Extension Bridge.
func NewExtensionProvider(bridge *BridgeService, mgr *Manager) *ExtensionProvider {
	return &ExtensionProvider{bridge: bridge, mgr: mgr}
}

func (p *ExtensionProvider) CaptureSearchEngineResult(ctx context.Context, engine, query, queryID string) (string, error) {
	if p == nil || p.bridge == nil {
		return "", fmt.Errorf("extension provider not initialized")
	}
	// Use the app's bridge capture method
	searchURL := ""
	if p.mgr != nil {
		searchURL = strings.TrimSpace(p.mgr.BuildSearchEngineURL(engine, query))
	}
	if searchURL == "" {
		searchURL = buildSearchEngineURL(engine, query)
	}
	if searchURL == "" {
		return "", fmt.Errorf("unsupported engine: %s", engine)
	}

	task := BridgeTask{
		RequestID:    fmt.Sprintf("router_search_%d", time.Now().UnixNano()),
		URL:          searchURL,
		BatchID:      queryID,
		WaitStrategy: "load",
	}
	result, err := p.bridge.Submit(ctx, task)
	if err != nil {
		return "", fmt.Errorf("extension bridge capture failed: %w", err)
	}
	if !result.Success {
		errMsg := strings.TrimSpace(result.Error)
		if errMsg == "" {
			errMsg = strings.TrimSpace(result.ErrorCode)
		}
		if errMsg == "" {
			errMsg = "unknown bridge error"
		}
		return "", fmt.Errorf("extension bridge capture failed: %s", errMsg)
	}
	if strings.TrimSpace(result.ImagePath) == "" {
		return "", fmt.Errorf("extension bridge capture missing image path")
	}
	return strings.TrimSpace(result.ImagePath), nil
}

func (p *ExtensionProvider) CaptureTargetWebsite(ctx context.Context, targetURL, ip, port, protocol, queryID string) (string, error) {
	if p == nil || p.bridge == nil {
		return "", fmt.Errorf("extension provider not initialized")
	}
	resolvedURL, err := p.buildTargetURL(targetURL, ip, port, protocol)
	if err != nil {
		return "", err
	}

	task := BridgeTask{
		RequestID:    fmt.Sprintf("router_target_%d", time.Now().UnixNano()),
		URL:          resolvedURL,
		BatchID:      queryID,
		WaitStrategy: "load",
	}
	result, err := p.bridge.Submit(ctx, task)
	if err != nil {
		return "", fmt.Errorf("extension bridge capture failed: %w", err)
	}
	if !result.Success {
		errMsg := strings.TrimSpace(result.Error)
		if errMsg == "" {
			errMsg = strings.TrimSpace(result.ErrorCode)
		}
		if errMsg == "" {
			errMsg = "unknown bridge error"
		}
		return "", fmt.Errorf("extension bridge capture failed: %s", errMsg)
	}
	if strings.TrimSpace(result.ImagePath) == "" {
		return "", fmt.Errorf("extension bridge capture missing image path")
	}
	return strings.TrimSpace(result.ImagePath), nil
}

func (p *ExtensionProvider) CaptureBatchURLs(ctx context.Context, urls []string, batchID string, concurrency int) ([]BatchScreenshotResult, error) {
	if p == nil || p.bridge == nil {
		return nil, fmt.Errorf("extension provider not initialized")
	}

	results := make([]BatchScreenshotResult, len(urls))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, rawURL := range urls {
		wg.Add(1)
		go func(idx int, inputURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			normalizedURL := normalizeURL(inputURL)
			result := BatchScreenshotResult{URL: inputURL, Timestamp: time.Now().Unix()}
			if normalizedURL == "" {
				result.Success = false
				result.Error = "invalid URL"
				results[idx] = result
				return
			}

			task := BridgeTask{
				RequestID:    fmt.Sprintf("router_batch_%d_%d", time.Now().UnixNano(), idx),
				URL:          normalizedURL,
				BatchID:      batchID,
				WaitStrategy: "load",
			}
			bridgeResult, err := p.bridge.Submit(ctx, task)
			if err != nil {
				result.Success = false
				result.Error = err.Error()
				results[idx] = result
				return
			}

			result.Success = bridgeResult.Success
			result.FilePath = bridgeResult.ImagePath
			if !bridgeResult.Success {
				if strings.TrimSpace(bridgeResult.Error) != "" {
					result.Error = bridgeResult.Error
				} else {
					result.Error = bridgeResult.ErrorCode
				}
			}
			results[idx] = result
		}(i, rawURL)
	}

	wg.Wait()
	return results, nil
}

func (p *ExtensionProvider) GetScreenshotDirectory() string {
	if p.mgr != nil {
		return p.mgr.GetScreenshotDirectory()
	}
	return ""
}

func (p *ExtensionProvider) OpenSearchEngineResult(ctx context.Context, engine, query string) (string, error) {
	if p == nil || p.bridge == nil {
		return "", fmt.Errorf("extension provider not initialized")
	}
	searchURL := ""
	if p.mgr != nil {
		searchURL = strings.TrimSpace(p.mgr.BuildSearchEngineURL(engine, query))
	}
	if searchURL == "" {
		searchURL = buildSearchEngineURL(engine, query)
	}
	if searchURL == "" {
		return "", fmt.Errorf("unsupported engine: %s", engine)
	}

	task := BridgeTask{
		RequestID:    fmt.Sprintf("router_open_%d", time.Now().UnixNano()),
		URL:          searchURL,
		WaitStrategy: "load",
		Timeout:      30 * time.Second,
		Action:       "open",
	}
	result, err := p.bridge.Submit(ctx, task)
	if err != nil {
		return "", fmt.Errorf("extension bridge open failed: %w", err)
	}
	if !result.Success {
		errMsg := strings.TrimSpace(result.Error)
		if errMsg == "" {
			errMsg = strings.TrimSpace(result.ErrorCode)
		}
		if errMsg == "" {
			errMsg = "extension reported open failure"
		}
		return "", fmt.Errorf("extension open failed for %s: %s", engine, errMsg)
	}
	return searchURL, nil
}

func (p *ExtensionProvider) CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]CollectResult, error) {
	if p == nil || p.bridge == nil {
		return nil, fmt.Errorf("extension provider not initialized")
	}
	searchURL := ""
	if p.mgr != nil {
		searchURL = strings.TrimSpace(p.mgr.BuildSearchEngineURL(engine, query))
	}
	if searchURL == "" {
		searchURL = buildSearchEngineURL(engine, query)
	}
	if searchURL == "" {
		return nil, fmt.Errorf("unsupported engine: %s", engine)
	}

	task := BridgeTask{
		RequestID:    fmt.Sprintf("router_collect_%d", time.Now().UnixNano()),
		URL:          searchURL,
		BatchID:      queryID,
		WaitStrategy: "load",
		Action:       "collect",
	}
	result, err := p.bridge.Submit(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("extension bridge collect failed: %w", err)
	}
	collectResult := CollectResult{
		Engine:    engine,
		Query:     query,
		RawURL:    searchURL,
		Timestamp: time.Now().Unix(),
	}

	// Check for login wall BEFORE treating success=false as an error.
	// The extension sets success=false + error_code="login_required" when a login wall
	// is detected. We must return a proper CollectResult with IsLoginWall=true rather
	// than an error, so the caller can distinguish "login required" from "actual failure".
	isLoginWall := false
	if len(result.StructuredCollectedData) > 0 {
		if lw, ok := result.StructuredCollectedData["is_login_wall"].(bool); ok && lw {
			isLoginWall = true
		}
	}
	if !isLoginWall && !result.Success {
		errCode := strings.TrimSpace(result.ErrorCode)
		if strings.Contains(strings.ToLower(errCode), "login") {
			isLoginWall = true
		}
	}

	if isLoginWall {
		collectResult.IsLoginWall = true
		collectResult.LoginRequired = true
		metrics.IncBrowserLoginRequired(engine)
		if len(result.StructuredCollectedData) > 0 {
			collectResult.Assets, collectResult.Total, collectResult.HasMore = parseStructuredCollectedData(result.StructuredCollectedData, engine)
			if title, ok := result.StructuredCollectedData["title"].(string); ok && title != "" {
				collectResult.Title = title
			}
		}
		return []CollectResult{collectResult}, nil
	}

	if !result.Success {
		errMsg := strings.TrimSpace(result.Error)
		if errMsg == "" {
			errMsg = strings.TrimSpace(result.ErrorCode)
		}
		if errMsg == "" {
			errMsg = "unknown bridge error"
		}
		return nil, fmt.Errorf("extension bridge collect failed: %s", errMsg)
	}

	// Anti-corruption: prefer structured data, fall back to string payload.
	if len(result.StructuredCollectedData) > 0 {
		collectResult.Assets, collectResult.Total, collectResult.HasMore = parseStructuredCollectedData(result.StructuredCollectedData, engine)
		if title, ok := result.StructuredCollectedData["title"].(string); ok && title != "" {
			collectResult.Title = title
		}
		// Collect diagnostic fields from extension
		if v, ok := result.StructuredCollectedData["extraction_method"].(string); ok {
			collectResult.ExtractionMethod = v
		}
		if v, ok := result.StructuredCollectedData["row_selector_used"].(string); ok {
			collectResult.RowSelectorUsed = v
		}
		if v, ok := result.StructuredCollectedData["rows_found"].(float64); ok {
			collectResult.RowsFound = int(v)
		}
		if v, ok := result.StructuredCollectedData["extraction_error"].(string); ok {
			collectResult.ExtractionError = v
		}
		// Also check login_required from structured data (second indicator from extension)
		if lr, ok := result.StructuredCollectedData["login_required"].(bool); ok && lr {
			collectResult.LoginRequired = true
			metrics.IncBrowserLoginRequired(engine)
		}
	} else if result.CollectedData != "" {
		collectResult.Title = result.CollectedData
	}

	return []CollectResult{collectResult}, nil
}

// buildSearchEngineURL builds a search engine result URL for bridge capture.
// Note: query should already be translated to engine-native syntax before calling this.
func buildSearchEngineURL(engine, query string) string {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "fofa":
		return fmt.Sprintf("%s/result?qbase64=%s", model.FOFAOfficialWebURL, urlBase64(query))
	case "hunter":
		return fmt.Sprintf("https://hunter.qianxin.com/home/list?search=%s&conditions=", urlBase64(query))
	case "quake":
		return fmt.Sprintf("https://quake.360.net/quake/#/searchResult?searchVal=%s&selectIndex=quake_service&latest=true", url.QueryEscape(query))
	case "zoomeye":
		return fmt.Sprintf("https://www.zoomeye.org/searchResult?q=%s", url.QueryEscape(query))
	case "shodan":
		return fmt.Sprintf("https://www.shodan.io/search?query=%s", url.QueryEscape(query))
	default:
		return ""
	}
}

// buildTargetURL builds a target website URL for bridge capture.
func (p *ExtensionProvider) buildTargetURL(targetURL, ip, port, protocol string) (string, error) {
	resolvedURL := strings.TrimSpace(targetURL)
	if resolvedURL == "" {
		resolvedIP := strings.TrimSpace(ip)
		if resolvedIP == "" {
			return "", fmt.Errorf("target URL or IP is required")
		}
		proto := "http"
		if p := strings.TrimSpace(protocol); p != "" {
			proto = strings.ToLower(p)
		} else if port == "443" {
			proto = "https"
		}
		resolvedPort := strings.TrimSpace(port)
		if resolvedPort != "" && resolvedPort != "80" && resolvedPort != "443" {
			resolvedURL = fmt.Sprintf("%s://%s:%s", proto, resolvedIP, resolvedPort)
		} else {
			resolvedURL = fmt.Sprintf("%s://%s", proto, resolvedIP)
		}
	}
	if !strings.HasPrefix(resolvedURL, "http://") && !strings.HasPrefix(resolvedURL, "https://") {
		resolvedURL = "http://" + resolvedURL
	}
	return resolvedURL, nil
}

// normalizeURL ensures a URL has a scheme prefix.
func normalizeURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "http://" + trimmed
	}
	return trimmed
}

// urlBase64 encodes a string as URL-safe base64.
func urlBase64(s string) string {
	return url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(s)))
}

// parseStructuredCollectedData extracts assets, total count, and has_more from
// the extension's structured collect payload. Anti-corruption: gracefully handles
// missing or malformed fields.
func parseStructuredCollectedData(data map[string]interface{}, engine string) ([]model.UnifiedAsset, int, bool) {
	assets := []model.UnifiedAsset{}
	total := 0
	hasMore := false

	if t, ok := data["total"].(float64); ok {
		total = int(t)
	}
	if hm, ok := data["has_more"].(bool); ok {
		hasMore = hm
	}

	rawItems, ok := data["items"]
	if !ok {
		return assets, total, hasMore
	}

	items, ok := rawItems.([]interface{})
	if !ok {
		return assets, total, hasMore
	}

	for _, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		asset := model.UnifiedAsset{
			Source: engine,
		}
		if v, ok := item["url"].(string); ok {
			asset.URL = v
		}
		if v, ok := item["title"].(string); ok {
			asset.Title = v
		}
		if v, ok := item["ip"].(string); ok {
			asset.IP = v
		}
		if v, ok := item["port"].(float64); ok {
			asset.Port = int(v)
		} else if v, ok := item["port"].(int); ok {
			asset.Port = v
		} else if v, ok := item["port"].(string); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				asset.Port = n
			}
		}
		if v, ok := item["protocol"].(string); ok {
			asset.Protocol = v
		}
		if v, ok := item["host"].(string); ok {
			asset.Host = v
		}
		if v, ok := item["body_snippet"].(string); ok && v != "" {
			asset.BodySnippet = v
		} else if v, ok := item["banner"].(string); ok && v != "" {
			asset.BodySnippet = v
		}
		if v, ok := item["server"].(string); ok {
			asset.Server = v
		}
		if v, ok := item["status_code"].(float64); ok {
			asset.StatusCode = int(v)
		} else if v, ok := item["status_code"].(string); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				asset.StatusCode = n
			}
		}
		if v, ok := item["country_code"].(string); ok {
			asset.CountryCode = v
		}
		if v, ok := item["region"].(string); ok {
			asset.Region = v
		}
		if v, ok := item["city"].(string); ok {
			asset.City = v
		}
		if v, ok := item["asn"].(string); ok {
			asset.ASN = v
		}
		if v, ok := item["org"].(string); ok {
			asset.Org = v
		}
		if v, ok := item["isp"].(string); ok {
			asset.ISP = v
		}
		// Store unrecognized engine-specific fields in Extra
		extra := make(map[string]interface{})
		known := map[string]bool{
			"url": true, "title": true, "ip": true, "port": true,
			"protocol": true, "host": true, "body_snippet": true,
			"banner": true, "server": true, "status_code": true,
			"country_code": true, "region": true, "city": true,
			"asn": true, "org": true, "isp": true, "os": true,
		}
		for k, v := range item {
			if !known[k] {
				extra[k] = v
			}
		}
		if len(extra) > 0 {
			asset.Extra = extra
		}
		assets = append(assets, asset)
	}

	return assets, total, hasMore
}
