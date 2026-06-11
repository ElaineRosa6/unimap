package screenshot

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/unimap/project/internal/collection"
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
	return p.CaptureBatchURLsWithProgress(ctx, urls, batchID, concurrency, nil)
}

func (p *ExtensionProvider) CaptureBatchURLsWithProgress(ctx context.Context, urls []string, batchID string, concurrency int, onResult func(BatchScreenshotResult)) ([]BatchScreenshotResult, error) {
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
				if onResult != nil {
					onResult(result)
				}
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
				if onResult != nil {
					onResult(result)
				}
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
			if onResult != nil {
				onResult(result)
			}
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

func (p *ExtensionProvider) CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]collection.CollectResult, error) {
	if p == nil || p.bridge == nil {
		return nil, fmt.Errorf("extension provider not initialized")
	}
	searchURL := p.resolveSearchURL(engine, query)
	if searchURL == "" {
		return nil, fmt.Errorf("unsupported engine: %s", engine)
	}

	result, err := p.submitCollectTask(ctx, searchURL, queryID)
	if err != nil {
		return nil, err
	}

	collectResult := collection.CollectResult{
		Engine: engine, Query: query, RawURL: searchURL, Timestamp: time.Now().Unix(),
	}

	if isLoginWallDetected(result) {
		return p.handleLoginWallResult(collectResult, result, engine), nil
	}
	if !result.Success {
		return nil, collectBridgeError(result)
	}

	p.populateCollectResultFromBridge(&collectResult, result, engine)
	return []collection.CollectResult{collectResult}, nil
}

// resolveSearchURL builds the search engine URL for collection.
func (p *ExtensionProvider) resolveSearchURL(engine, query string) string {
	if p.mgr != nil {
		if u := strings.TrimSpace(p.mgr.BuildSearchEngineURL(engine, query)); u != "" {
			return u
		}
	}
	return buildSearchEngineURL(engine, query)
}

// submitCollectTask submits a collect action to the bridge.
func (p *ExtensionProvider) submitCollectTask(ctx context.Context, searchURL, queryID string) (BridgeResult, error) {
	task := BridgeTask{
		RequestID:    fmt.Sprintf("router_collect_%d", time.Now().UnixNano()),
		URL:          searchURL,
		BatchID:      queryID,
		WaitStrategy: "load",
		Action:       "collect",
	}
	result, err := p.bridge.Submit(ctx, task)
	if err != nil {
		return result, fmt.Errorf("extension bridge collect failed: %w", err)
	}
	return result, nil
}

// isLoginWallDetected checks if the bridge result indicates a login wall.
func isLoginWallDetected(result BridgeResult) bool {
	if result.StructuredCollectedData != nil && result.StructuredCollectedData.Extra != nil {
		if lw, ok := result.StructuredCollectedData.Extra["is_login_wall"].(bool); ok && lw {
			return true
		}
	}
	if !result.Success {
		errCode := strings.TrimSpace(result.ErrorCode)
		if strings.Contains(strings.ToLower(errCode), "login") {
			return true
		}
	}
	return false
}

// handleLoginWallResult processes a login wall detection into a collection.CollectResult.
func (p *ExtensionProvider) handleLoginWallResult(cr collection.CollectResult, result BridgeResult, engine string) []collection.CollectResult {
	cr.IsLoginWall = true
	cr.LoginRequired = true
	metrics.IncBrowserLoginRequired(engine)
	if result.StructuredCollectedData != nil {
		cr.Assets, cr.Total, cr.HasMore = collection.ParseStructuredCollectedDataFromItems(result.StructuredCollectedData.Items, engine)
		if result.StructuredCollectedData.Extra != nil {
			if title, ok := result.StructuredCollectedData.Extra["title"].(string); ok && title != "" {
				cr.Title = title
			}
		}
	}
	return []collection.CollectResult{cr}
}

// collectBridgeError builds an error from a failed bridge result.
func collectBridgeError(result BridgeResult) error {
	errMsg := strings.TrimSpace(result.Error)
	if errMsg == "" {
		errMsg = strings.TrimSpace(result.ErrorCode)
	}
	if errMsg == "" {
		errMsg = "unknown bridge error"
	}
	return fmt.Errorf("extension bridge collect failed: %s", errMsg)
}

// populateCollectResultFromBridge fills a collection.CollectResult from bridge structured data.
func (p *ExtensionProvider) populateCollectResultFromBridge(cr *collection.CollectResult, result BridgeResult, engine string) {
	if result.StructuredCollectedData == nil {
		if result.CollectedData != "" {
			cr.Title = result.CollectedData
		}
		return
	}
	data := result.StructuredCollectedData
	cr.Assets, cr.Total, cr.HasMore = collection.ParseStructuredCollectedDataFromItems(data.Items, engine)
	if data.Extra != nil {
		if title, ok := data.Extra["title"].(string); ok && title != "" {
			cr.Title = title
		}
		if v, ok := data.Extra["extraction_method"].(string); ok {
			cr.ExtractionMethod = v
		}
		if v, ok := data.Extra["row_selector_used"].(string); ok {
			cr.RowSelectorUsed = v
		}
		if v, ok := data.Extra["rows_found"].(float64); ok {
			cr.RowsFound = int(v)
		}
		if v, ok := data.Extra["extraction_error"].(string); ok {
			cr.ExtractionError = v
		}
		if lr, ok := data.Extra["login_required"].(bool); ok && lr {
			cr.LoginRequired = true
			metrics.IncBrowserLoginRequired(engine)
		}
	}
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

// extractExtraFields collects unrecognized fields into an Extra map.
func extractExtraFields(item map[string]interface{}) map[string]interface{} {
	known := map[string]bool{
		"url": true, "title": true, "ip": true, "port": true,
		"protocol": true, "host": true, "body_snippet": true,
		"banner": true, "server": true, "status_code": true,
		"country_code": true, "region": true, "city": true,
		"asn": true, "org": true, "isp": true, "os": true,
	}
	extra := make(map[string]interface{})
	for k, v := range item {
		if !known[k] {
			extra[k] = v
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}
