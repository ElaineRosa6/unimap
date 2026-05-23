package screenshot

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/unimap-icp-hunter/project/internal/model"
)

// ScreenshotMode represents the active screenshot capture mode.
type ScreenshotMode string

const (
	ModeCDP       ScreenshotMode = "cdp"
	ModeExtension ScreenshotMode = "extension"
	ModeAuto      ScreenshotMode = "auto"
)

// RouterConfig holds the routing configuration.
type RouterConfig struct {
	Priority      ScreenshotMode // Primary mode to prefer
	Fallback      bool           // Whether to fall back to the other mode on failure
	ProbeInterval time.Duration  // How often to run health checks
	ProbeTimeout  time.Duration  // Timeout per probe
}

// ScreenshotRouter routes screenshot requests between CDP and Extension modes
// with automatic health-based failover.
type ScreenshotRouter struct {
	cfg       RouterConfig
	cdp       Provider
	extBridge *BridgeService
	mgr       *Manager

	cdpChecker HealthChecker
	extChecker HealthChecker

	// Current active mode (atomic for lock-free reads)
	currentMode atomic.Value // ScreenshotMode

	// Per-mode health status
	cdpHealthy atomic.Bool
	extHealthy atomic.Bool

	// Probe goroutine lifecycle
	mu      sync.Mutex
	stopCh  chan struct{}
	stopped bool

	// Metrics hooks
	onModeSwitch  func(from, to ScreenshotMode)
	onHealthCheck func(mode string, healthy bool)
}

// SetMetricsHooks registers callback functions for Prometheus metrics.
func (r *ScreenshotRouter) SetMetricsHooks(onModeSwitch func(from, to ScreenshotMode), onHealthCheck func(mode string, healthy bool)) {
	r.onModeSwitch = onModeSwitch
	r.onHealthCheck = onHealthCheck
}

// NewScreenshotRouter creates a new ScreenshotRouter.
func NewScreenshotRouter(cfg RouterConfig, cdp Provider, extBridge *BridgeService, mgr *Manager) *ScreenshotRouter {
	if cfg.ProbeInterval <= 0 {
		cfg.ProbeInterval = 30 * time.Second
	}
	if cfg.ProbeTimeout <= 0 {
		cfg.ProbeTimeout = 5 * time.Second
	}

	r := &ScreenshotRouter{
		cfg:       cfg,
		cdp:       cdp,
		extBridge: extBridge,
		mgr:       mgr,
		stopCh:    make(chan struct{}),
	}

	// Initialize health checkers
	if cdp != nil {
		remoteURL := ""
		if mgr != nil {
			remoteURL = mgr.RemoteDebugURL()
		}
		r.cdpChecker = &CDPHealthChecker{RemoteDebugURL: remoteURL}
		r.cdpHealthy.Store(true) // CDP available
	} else {
		r.cdpChecker = &CDPHealthChecker{RemoteDebugURL: ""}
		r.cdpHealthy.Store(false)
	}

	isMock := extBridge != nil && isMockBridgeClient(extBridge)
	r.extChecker = &ExtensionHealthChecker{BridgeService: extBridge, IsMock: isMock}
	r.extHealthy.Store(extBridge != nil)

	// Set initial mode
	r.currentMode.Store(cfg.Priority)

	return r
}

// Start launches the health probe goroutine.
func (r *ScreenshotRouter) Start(ctx context.Context) {
	// Run initial probes synchronously
	r.runProbes(ctx)

	go r.probeLoop(ctx)
}

// Stop terminates the health probe goroutine.
func (r *ScreenshotRouter) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.stopped {
		r.stopped = true
		close(r.stopCh)
	}
}

func (r *ScreenshotRouter) probeLoop(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.runProbes(ctx)
		}
	}
}

func (r *ScreenshotRouter) runProbes(ctx context.Context) {
	probeCtx, cancel := context.WithTimeout(ctx, r.cfg.ProbeTimeout)
	defer cancel()

	cdpOK, _ := r.cdpChecker.Check(probeCtx)
	r.cdpHealthy.Store(cdpOK)

	extOK, _ := r.extChecker.Check(probeCtx)
	r.extHealthy.Store(extOK)

	// Metrics
	if r.onHealthCheck != nil {
		if cdpOK {
			r.onHealthCheck("cdp", true)
		} else {
			r.onHealthCheck("cdp", false)
		}
		if extOK {
			r.onHealthCheck("extension", true)
		} else {
			r.onHealthCheck("extension", false)
		}
	}

	// Determine best mode
	current := r.loadMode()
	best := r.determineBestMode(current, cdpOK, extOK)
	if best != current {
		r.currentMode.Store(best)
		if r.onModeSwitch != nil {
			r.onModeSwitch(current, best)
		}
	}
}

func (r *ScreenshotRouter) determineBestMode(current ScreenshotMode, cdpOK, extOK bool) ScreenshotMode {
	// Auto mode: pick the healthiest provider
	if current == ModeAuto {
		if cdpOK && extOK {
			return ModeCDP // prefer CDP when both healthy
		}
		if cdpOK {
			return ModeCDP
		}
		if extOK {
			return ModeExtension
		}
		return ModeAuto // neither healthy, stay auto
	}

	// Forced mode: respect the configured mode, fall back only if configured
	switch current {
	case ModeCDP:
		if cdpOK {
			return ModeCDP
		}
		if r.cfg.Fallback && extOK {
			return ModeExtension
		}
		return ModeCDP
	case ModeExtension:
		if extOK {
			return ModeExtension
		}
		if r.cfg.Fallback && cdpOK {
			return ModeCDP
		}
		return ModeExtension
	}
	return current
}

// loadMode safely reads the current screenshot mode from atomic.Value.
func (r *ScreenshotRouter) loadMode() ScreenshotMode {
	v := r.currentMode.Load()
	if v == nil {
		return ModeAuto
	}
	mode, ok := v.(ScreenshotMode)
	if !ok {
		return ModeAuto
	}
	return mode
}

// ActiveMode returns the current active screenshot mode.
func (r *ScreenshotRouter) ActiveMode() ScreenshotMode {
	return r.loadMode()
}

// HealthStatus returns the health status of both modes.
func (r *ScreenshotRouter) HealthStatus() (cdpHealthy, extHealthy bool) {
	return r.cdpHealthy.Load(), r.extHealthy.Load()
}

// Config returns the router configuration.
func (r *ScreenshotRouter) Config() RouterConfig {
	return r.cfg
}

// SetMode sets the active screenshot execution mode.
// ModeAuto delegates to the health probe loop to pick the best mode.
// ModeCDP and ModeExtension are forced modes — they will not auto-switch
// unless fallback is enabled and the forced provider is unhealthy.
func (r *ScreenshotRouter) SetMode(mode ScreenshotMode) {
	if mode != ModeCDP && mode != ModeExtension && mode != ModeAuto {
		return
	}
	old := r.loadMode()
	if old == mode {
		return
	}
	r.currentMode.Store(mode)
	if r.onModeSwitch != nil {
		r.onModeSwitch(old, mode)
	}
}

// CurrentMode returns the active screenshot execution mode.
func (r *ScreenshotRouter) CurrentMode() ScreenshotMode {
	return r.loadMode()
}

// CaptureSearchEngineResult captures a search engine result using the active mode.
func (r *ScreenshotRouter) CaptureSearchEngineResult(ctx context.Context, engine, query, queryID string) (string, error) {
	provider, err := r.resolveProvider(r.loadMode())
	if err != nil {
		return "", err
	}
	return provider.CaptureSearchEngineResult(ctx, engine, query, queryID)
}

// CaptureTargetWebsite captures a target website using the active mode.
func (r *ScreenshotRouter) CaptureTargetWebsite(ctx context.Context, targetURL, ip, port, protocol, queryID string) (string, error) {
	provider, err := r.resolveProvider(r.loadMode())
	if err != nil {
		return "", err
	}
	return provider.CaptureTargetWebsite(ctx, targetURL, ip, port, protocol, queryID)
}

// CaptureBatchURLs captures a batch of URLs using the active mode.
func (r *ScreenshotRouter) CaptureBatchURLs(ctx context.Context, urls []string, batchID string, concurrency int) ([]BatchScreenshotResult, error) {
	provider, err := r.resolveProvider(r.loadMode())
	if err != nil {
		return nil, err
	}
	return provider.CaptureBatchURLs(ctx, urls, batchID, concurrency)
}

// GetScreenshotDirectory returns the screenshot base directory.
func (r *ScreenshotRouter) GetScreenshotDirectory() string {
	if r.mgr != nil {
		return r.mgr.GetScreenshotDirectory()
	}
	return ""
}

// OpenSearchEngineResult opens a search engine result page using the active mode.
func (r *ScreenshotRouter) OpenSearchEngineResult(ctx context.Context, engine, query string) (string, error) {
	provider, err := r.resolveProvider(r.loadMode())
	if err != nil {
		return "", err
	}
	return provider.OpenSearchEngineResult(ctx, engine, query)
}

// CollectSearchEngineResult collects structured data from a search engine result using the active mode.
func (r *ScreenshotRouter) CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]CollectResult, error) {
	provider, err := r.resolveProvider(r.loadMode())
	if err != nil {
		return nil, err
	}
	return provider.CollectSearchEngineResult(ctx, engine, query, queryID)
}

// resolveProvider returns the best available Provider based on current health and fallback config.
func (r *ScreenshotRouter) resolveProvider(primaryMode ScreenshotMode) (Provider, error) {
	mode := r.determineBestMode(primaryMode, r.cdpHealthy.Load(), r.extHealthy.Load())

	// Try the determined mode first
	if provider := r.providerForMode(mode); provider != nil {
		return provider, nil
	}

	// Fallback to the other mode
	other := ModeCDP
	if mode == ModeCDP {
		other = ModeExtension
	}
	if r.cfg.Fallback {
		if provider := r.providerForMode(other); provider != nil {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("no screenshot provider available (cdp=%v, extension=%v, mode=%s, fallback=%v)",
		r.cdp != nil, r.extBridge != nil, mode, r.cfg.Fallback)
}

// providerForMode returns the Provider for the given mode, or nil if unavailable.
func (r *ScreenshotRouter) providerForMode(mode ScreenshotMode) Provider {
	switch mode {
	case ModeCDP:
		return r.cdp
	case ModeExtension:
		if r.extBridge == nil {
			return nil
		}
		return NewExtensionProvider(r.extBridge, r.mgr)
	default:
		return nil
	}
}

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

	collectResult := CollectResult{
		Engine:    engine,
		Query:     query,
		RawURL:    searchURL,
		Timestamp: time.Now().Unix(),
	}

	// Anti-corruption: prefer structured data, fall back to string payload.
	if len(result.StructuredCollectedData) > 0 {
		collectResult.Assets, collectResult.Total, collectResult.HasMore = parseStructuredCollectedData(result.StructuredCollectedData, engine)
		if title, ok := result.StructuredCollectedData["title"].(string); ok && title != "" {
			collectResult.Title = title
		}
		if lw, ok := result.StructuredCollectedData["is_login_wall"].(bool); ok && lw {
			collectResult.IsLoginWall = true
			collectResult.LoginRequired = true
		}
	} else if result.CollectedData != "" {
		collectResult.Title = result.CollectedData
	}

	return []CollectResult{collectResult}, nil
}

// buildSearchEngineURL builds a search engine result URL for bridge capture.
func buildSearchEngineURL(engine, query string) string {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "fofa":
		return fmt.Sprintf("%s/result?qbase64=%s", model.FOFAOfficialWebURL, urlBase64(query))
	case "hunter":
		return fmt.Sprintf("https://hunter.qianxin.com/list?searchValue=%s", urlBase64(query))
	case "quake":
		return fmt.Sprintf("https://quake.360.cn/quake/#/searchResult?searchVal=%s", url.QueryEscape(query))
	case "zoomeye":
		return fmt.Sprintf("https://www.zoomeye.org/searchResult?q=%s", url.QueryEscape(query))
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

// isMockBridgeClient detects if the BridgeService wraps a mock client.
// Since we cannot inspect the wrapped client type from the BridgeService,
// this is handled at creation time via the ExtensionHealthChecker.
func isMockBridgeClient(svc *BridgeService) bool {
	return false
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
		}
		if v, ok := item["protocol"].(string); ok {
			asset.Protocol = v
		}
		if v, ok := item["host"].(string); ok {
			asset.Host = v
		}
		if v, ok := item["body_snippet"].(string); ok {
			asset.BodySnippet = v
		}
		if v, ok := item["server"].(string); ok {
			asset.Server = v
		}
		if v, ok := item["status_code"].(float64); ok {
			asset.StatusCode = int(v)
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
			"server": true, "status_code": true, "country_code": true,
			"region": true, "city": true, "asn": true, "org": true, "isp": true,
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
