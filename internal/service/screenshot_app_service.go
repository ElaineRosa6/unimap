package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/unimap/project/internal/metrics"
	"github.com/unimap/project/internal/screenshot"
)

// ScreenshotAppService 封装截图相关应用层流程。
type ScreenshotAppService struct {
	mu            sync.RWMutex
	baseDir       string
	provider      screenshot.Provider
	engine        string
	bridgeService *screenshot.BridgeService
	fallbackToCDP bool
}

type proxyAwareSearchProvider interface {
	CaptureSearchEngineResultWithProxy(ctx context.Context, engine, query, queryID, proxy string) (string, error)
}

type proxyAwareTargetProvider interface {
	CaptureTargetWebsiteWithProxy(ctx context.Context, targetURL, ip, port, protocol, queryID, proxy string) (string, error)
}

func NewScreenshotAppService(baseDir string) *ScreenshotAppService {
	return NewScreenshotAppServiceWithProvider(baseDir, nil)
}

func NewScreenshotAppServiceWithProvider(baseDir string, provider screenshot.Provider) *ScreenshotAppService {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "./screenshots"
	}
	return &ScreenshotAppService{baseDir: baseDir, provider: provider, engine: "cdp"}
}

func (s *ScreenshotAppService) SetEngine(engine string) {
	if s == nil {
		return
	}
	engine = strings.ToLower(strings.TrimSpace(engine))
	if engine == "" {
		engine = "cdp"
	}
	s.mu.Lock()
	s.engine = engine
	s.mu.Unlock()
}

func (s *ScreenshotAppService) SetBridgeService(bridge *screenshot.BridgeService) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.bridgeService = bridge
	s.mu.Unlock()
}

func (s *ScreenshotAppService) SetFallbackToCDP(enabled bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.fallbackToCDP = enabled
	s.mu.Unlock()
}

// SetMode updates the screenshot execution mode. It maps the mode to the
// engine field for the service's extension-first logic.
func (s *ScreenshotAppService) SetMode(mode string) {
	if s == nil {
		return
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "cdp" || mode == "extension" {
		s.mu.Lock()
		s.engine = mode
		s.mu.Unlock()
	}
	// "auto" leaves engine as-is; the router handles mode selection.
}

// appConfigSnapshot captures the mutable config fields under a single read lock.
type appConfigSnapshot struct {
	engine        string
	bridgeService *screenshot.BridgeService
	fallbackToCDP bool
}

func (s *ScreenshotAppService) configSnapshot() appConfigSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return appConfigSnapshot{
		engine:        s.engine,
		bridgeService: s.bridgeService,
		fallbackToCDP: s.fallbackToCDP,
	}
}

// IsCaptureAvailable reports whether screenshot capture can run with current dependencies.
func (s *ScreenshotAppService) IsCaptureAvailable(mgr *screenshot.Manager) bool {
	if s != nil && s.provider != nil {
		return true
	}
	return mgr != nil
}

// GetBaseDir 获取截图基础目录
func (s *ScreenshotAppService) GetBaseDir() string {
	return s.baseDir
}

type BatchScreenshotRequest struct {
	QueryID string
	Engines []struct {
		Engine string
		Query  string
	}
	Targets []struct {
		URL      string
		IP       string
		Port     string
		Protocol string
	}
}

type BatchScreenshotResponse struct {
	QueryID       string                   `json:"query_id"`
	SearchEngines []map[string]interface{} `json:"search_engines"`
	Targets       []map[string]interface{} `json:"targets"`
	Errors        []string                 `json:"errors"`
}

type BatchURLsRequest struct {
	URLs        []string
	BatchID     string
	Concurrency int
}

type BatchURLsResponse struct {
	BatchID       string                             `json:"batch_id"`
	Total         int                                `json:"total"`
	Success       int                                `json:"success"`
	Failed        int                                `json:"failed"`
	Results       []screenshot.BatchScreenshotResult `json:"results"`
	ScreenshotDir string                             `json:"screenshot_dir"`
}

func (s *ScreenshotAppService) CaptureSearchEngineResult(ctx context.Context, mgr *screenshot.Manager, engine, query, queryID string) (string, string, string, string, error) {
	return s.CaptureSearchEngineResultWithProxy(ctx, mgr, engine, query, queryID, "")
}

func (s *ScreenshotAppService) CaptureSearchEngineResultWithProxy(ctx context.Context, mgr *screenshot.Manager, engine, query, queryID, proxy string) (string, string, string, string, error) {
	engine = strings.TrimSpace(engine)
	query = strings.TrimSpace(query)
	if engine == "" || query == "" {
		return "", "", "", "", fmt.Errorf("missing engine or query parameter")
	}
	if strings.TrimSpace(queryID) == "" {
		queryID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	cfg := s.configSnapshot()
	if strings.EqualFold(cfg.engine, "extension") && cfg.bridgeService != nil {
		path, bridgeErr := s.captureSearchEngineWithBridge(ctx, mgr, engine, query, queryID)
		if bridgeErr == nil {
			return path, engine, query, queryID, nil
		}
		if !cfg.fallbackToCDP {
			return "", "", "", "", bridgeErr
		}
		metrics.IncBridgeFallback("extension_to_cdp")
	}

	provider, err := s.resolveProvider(mgr)
	if err != nil {
		return "", "", "", "", fmt.Errorf("screenshot manager not initialized")
	}
	path := ""
	if withProxy, ok := provider.(proxyAwareSearchProvider); ok && strings.TrimSpace(proxy) != "" {
		path, err = withProxy.CaptureSearchEngineResultWithProxy(ctx, engine, query, queryID, proxy)
	} else {
		path, err = provider.CaptureSearchEngineResult(ctx, engine, query, queryID)
	}
	if err != nil {
		return "", "", "", "", err
	}
	return path, engine, query, queryID, nil
}

func (s *ScreenshotAppService) CaptureTargetWebsite(ctx context.Context, mgr *screenshot.Manager, targetURL, ip, port, protocol, queryID string) (string, string, string, string, string, string, error) {
	return s.CaptureTargetWebsiteWithProxy(ctx, mgr, targetURL, ip, port, protocol, queryID, "")
}

func (s *ScreenshotAppService) CaptureTargetWebsiteWithProxy(ctx context.Context, mgr *screenshot.Manager, targetURL, ip, port, protocol, queryID, proxy string) (string, string, string, string, string, string, error) {
	targetURL = strings.TrimSpace(targetURL)
	ip = strings.TrimSpace(ip)
	port = strings.TrimSpace(port)
	protocol = strings.TrimSpace(protocol)
	queryID = strings.TrimSpace(queryID)
	if targetURL == "" && ip == "" {
		return "", "", "", "", "", "", fmt.Errorf("missing url or ip parameter")
	}
	if queryID == "" {
		queryID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	cfg := s.configSnapshot()
	if strings.EqualFold(cfg.engine, "extension") && cfg.bridgeService != nil {
		path, bridgeErr := s.captureTargetWithBridge(ctx, targetURL, ip, port, protocol, queryID)
		if bridgeErr == nil {
			return path, targetURL, ip, port, protocol, queryID, nil
		}
		if !cfg.fallbackToCDP {
			return "", "", "", "", "", "", bridgeErr
		}
		metrics.IncBridgeFallback("extension_to_cdp")
	}

	provider, err := s.resolveProvider(mgr)
	if err != nil {
		return "", "", "", "", "", "", fmt.Errorf("screenshot manager not initialized")
	}

	path := ""
	if withProxy, ok := provider.(proxyAwareTargetProvider); ok && strings.TrimSpace(proxy) != "" {
		path, err = withProxy.CaptureTargetWebsiteWithProxy(ctx, targetURL, ip, port, protocol, queryID, proxy)
	} else {
		path, err = provider.CaptureTargetWebsite(ctx, targetURL, ip, port, protocol, queryID)
	}
	if err != nil {
		return "", "", "", "", "", "", err
	}
	return path, targetURL, ip, port, protocol, queryID, nil
}

func (s *ScreenshotAppService) CaptureBatch(ctx context.Context, mgr *screenshot.Manager, req BatchScreenshotRequest) (*BatchScreenshotResponse, error) {
	provider, err := s.resolveProvider(mgr)
	if err != nil {
		return nil, fmt.Errorf("screenshot manager not initialized")
	}
	if strings.TrimSpace(req.QueryID) == "" {
		req.QueryID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	resp := &BatchScreenshotResponse{
		QueryID:       req.QueryID,
		SearchEngines: []map[string]interface{}{},
		Targets:       []map[string]interface{}{},
		Errors:        []string{},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, engine := range req.Engines {
		wg.Add(1)
		go func(engineName, query string) {
			defer wg.Done()
			path, err := provider.CaptureSearchEngineResult(ctx, engineName, query, req.QueryID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %v", engineName, err))
				return
			}
			resp.SearchEngines = append(resp.SearchEngines, map[string]interface{}{
				"engine": engineName,
				"query":  query,
				"path":   path,
			})
		}(engine.Engine, engine.Query)
	}

	for _, target := range req.Targets {
		wg.Add(1)
		go func(url, ip, port, protocol string) {
			defer wg.Done()
			path, err := provider.CaptureTargetWebsite(ctx, url, ip, port, protocol, req.QueryID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				resp.Errors = append(resp.Errors, fmt.Sprintf("%s:%s: %v", ip, port, err))
				return
			}
			resp.Targets = append(resp.Targets, map[string]interface{}{
				"url":      url,
				"ip":       ip,
				"port":     port,
				"protocol": protocol,
				"path":     path,
			})
		}(target.URL, target.IP, target.Port, target.Protocol)
	}

	wg.Wait()
	return resp, nil
}

func (s *ScreenshotAppService) CaptureBatchURLs(ctx context.Context, mgr *screenshot.Manager, req BatchURLsRequest) (*BatchURLsResponse, error) {
	if len(req.URLs) == 0 {
		return nil, fmt.Errorf("no URLs provided")
	}
	if len(req.URLs) > 100 {
		return nil, fmt.Errorf("too many URLs")
	}
	if strings.TrimSpace(req.BatchID) == "" {
		req.BatchID = fmt.Sprintf("batch_%d", time.Now().UnixNano())
	}
	if req.Concurrency <= 0 || req.Concurrency > 10 {
		req.Concurrency = 5
	}

	cfg := s.configSnapshot()
	if strings.EqualFold(cfg.engine, "extension") && cfg.bridgeService != nil {
		bridgeResp, bridgeErr := s.captureBatchURLsWithBridge(ctx, req)
		if bridgeErr == nil {
			return bridgeResp, nil
		}
		if !cfg.fallbackToCDP {
			return nil, bridgeErr
		}
		metrics.IncBridgeFallback("extension_to_cdp")
	}

	provider, err := s.resolveProvider(mgr)
	if err != nil {
		return nil, fmt.Errorf("screenshot manager not initialized")
	}

	results, err := provider.CaptureBatchURLs(ctx, req.URLs, req.BatchID, req.Concurrency)
	if err != nil {
		return nil, err
	}

	successCount := 0
	failCount := 0
	for _, item := range results {
		if item.Success {
			successCount++
		} else {
			failCount++
		}
	}

	return &BatchURLsResponse{
		BatchID:       req.BatchID,
		Total:         len(req.URLs),
		Success:       successCount,
		Failed:        failCount,
		Results:       results,
		ScreenshotDir: provider.GetScreenshotDirectory(),
	}, nil
}

func (s *ScreenshotAppService) captureBatchURLsWithBridge(ctx context.Context, req BatchURLsRequest) (*BatchURLsResponse, error) {
	cfg := s.configSnapshot()
	if cfg.bridgeService == nil {
		return nil, fmt.Errorf("bridge service not initialized")
	}

	results := make([]screenshot.BatchScreenshotResult, len(req.URLs))
	sem := make(chan struct{}, req.Concurrency)
	var wg sync.WaitGroup

	for i, rawURL := range req.URLs {
		wg.Add(1)
		go func(idx int, inputURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			startedAt := time.Now()

			normalizedURL := normalizeBridgeTargetURL(inputURL)
			result := screenshot.BatchScreenshotResult{URL: inputURL, Timestamp: time.Now().Unix()}
			if normalizedURL == "" {
				metrics.IncBridgeRequest("extension", "invalid_url")
				metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
				result.Success = false
				result.Error = "invalid URL"
				results[idx] = result
				return
			}

			task := screenshot.BridgeTask{
				RequestID:    fmt.Sprintf("bridge_%d_%d", time.Now().UnixNano(), idx),
				URL:          normalizedURL,
				BatchID:      req.BatchID,
				WaitStrategy: "load",
			}
			bridgeResult, err := cfg.bridgeService.Submit(ctx, task)
			if err != nil {
				metrics.IncBridgeRequest("extension", "submit_failed")
				metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
				result.Success = false
				result.Error = err.Error()
				results[idx] = result
				return
			}

			result.Success = bridgeResult.Success
			result.FilePath = bridgeResult.ImagePath
			if !bridgeResult.Success {
				metrics.IncBridgeRequest("extension", "result_failed")
				metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
				if strings.TrimSpace(bridgeResult.Error) != "" {
					result.Error = bridgeResult.Error
				} else {
					result.Error = bridgeResult.ErrorCode
				}
			} else {
				metrics.IncBridgeRequest("extension", "success")
				metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
			}
			results[idx] = result
		}(i, rawURL)
	}

	wg.Wait()

	successCount := 0
	failCount := 0
	for _, item := range results {
		if item.Success {
			successCount++
		} else {
			failCount++
		}
	}

	return &BatchURLsResponse{
		BatchID:       req.BatchID,
		Total:         len(req.URLs),
		Success:       successCount,
		Failed:        failCount,
		Results:       results,
		ScreenshotDir: s.baseDir,
	}, nil
}
