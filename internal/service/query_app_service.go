package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/core/unimap"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/screenshot"
)

// BrowserQueryOutcome 封装浏览器联动查询的结果。
type BrowserQueryOutcome struct {
	Enabled            bool
	OpenedEngines      []string
	CollectedResults   []collection.CollectResult
	Errors             []string
	AutoCaptureEnabled bool
	AutoCaptureQueryID string
	AutoCapturedPaths  map[string]string
	AutoCaptureErrors  []string
}

// QueryAppService 封装查询应用层流程（引擎选择、核心查询、可选浏览器联动）。
type QueryAppService struct {
	unified      *UnifiedService
	orchestrator *adapter.EngineOrchestrator
}

func NewQueryAppService(unified *UnifiedService, orchestrator *adapter.EngineOrchestrator) *QueryAppService {
	return &QueryAppService{unified: unified, orchestrator: orchestrator}
}

// ResolveEngines 解析最终要使用的引擎列表。
func (s *QueryAppService) ResolveEngines(engines []string) []string {
	if len(engines) > 0 {
		return engines
	}
	if s.orchestrator == nil {
		return nil
	}
	defaults := s.orchestrator.ListAdapters()
	if len(defaults) == 0 {
		return nil
	}
	return []string{defaults[0]}
}

// ExecuteQuery 执行统一查询。
func (s *QueryAppService) ExecuteQuery(ctx context.Context, query string, engines []string, pageSize int) (*QueryResponse, error) {
	if s.unified == nil {
		return nil, fmt.Errorf("query service not initialized")
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	return s.unified.Query(ctx, QueryRequest{
		Query:       query,
		Engines:     engines,
		PageSize:    pageSize,
		ProcessData: true,
	})
}

func (s *QueryAppService) translateBrowserQuery(query, engine string) (string, error) {
	if s.orchestrator == nil {
		return query, nil
	}
	adapter, ok := s.orchestrator.GetAdapter(engine)
	if !ok {
		return "", fmt.Errorf("adapter %s not found", engine)
	}
	ast, err := unimap.NewUQLParser().Parse(query)
	if err != nil {
		return "", fmt.Errorf("parse browser query for %s: %w", engine, err)
	}
	translated, err := adapter.Translate(ast)
	if err != nil {
		return "", fmt.Errorf("translate browser query for %s: %w", engine, err)
	}
	if strings.TrimSpace(translated) == "" {
		return "", fmt.Errorf("translate browser query for %s returned empty query", engine)
	}
	return translated, nil
}

// RunBrowserQueryAsync 执行可选浏览器联动（打开结果页、截图、采集结构化结果）。
// progressCallback 在每个引擎阶段推进时被调用（progress 范围 0~100），可为 nil。
func (s *QueryAppService) RunBrowserQueryAsync(
	ctx context.Context,
	query string,
	engines []string,
	enabled bool,
	action string,
	queryID string,
	autoCaptureEnabled bool,
	screenshotApp *ScreenshotAppService,
	screenshotMgr *screenshot.Manager,
	previewURLBuilder func(string) string,
	browserRouter BrowserRouter,
	progress func(done, total int, engine string, err error),
) <-chan BrowserQueryOutcome {
	if !enabled {
		return nil
	}

	// Anti-corruption: old clients send browser_query=true without browser_action;
	// fallback to "collect" semantics (the previous default behavior).
	if action == "" {
		action = "collect"
	}

	// Backward compatibility: map old action names to the canonical ones.
	//   - Old "capture" (was: collect-only) → "collect"
	//   - Old "collect" with screenshot context (was: collect+截图) → "collect_and_capture"
	// Heuristic: autoCaptureEnabled serves as the "screenshot context" signal.
	switch action {
	case "capture":
		logger.CtxInfof(ctx, "legacy browser_action 'capture' mapped to 'collect'")
		action = "collect"
	case "collect":
		if autoCaptureEnabled {
			logger.CtxInfof(ctx, "legacy browser_action 'collect' with screenshot context mapped to 'collect_and_capture'")
			action = "collect_and_capture"
		}
	}

	resultCh := make(chan BrowserQueryOutcome, 1)
	go func() {
		defer close(resultCh)
		outcome := BrowserQueryOutcome{Enabled: true}

		if autoCaptureEnabled && (action == "collect" || action == "collect_and_capture") {
			if strings.TrimSpace(queryID) == "" {
				queryID = fmt.Sprintf("query_%d", time.Now().UnixNano())
			}
			outcome.AutoCaptureEnabled = true
			outcome.AutoCaptureQueryID = queryID
			outcome.AutoCapturedPaths = make(map[string]string)
		}

		captureAvailable := screenshotApp != nil && screenshotApp.IsCaptureAvailable(screenshotMgr)
		if outcome.AutoCaptureEnabled && !captureAvailable {
			outcome.AutoCaptureErrors = append(outcome.AutoCaptureErrors, "auto capture unavailable: screenshot engine not initialized")
		}

		total := len(engines)
		completed := 0
		for _, engine := range engines {
			func(engine string) {
				var engineErr error
				defer func() {
					completed++
					if progress != nil {
						progress(completed, total, engine, engineErr)
					}
				}()

				browserQuery, err := s.translateBrowserQuery(query, engine)
				if err != nil {
					engineErr = err
					outcome.Errors = append(outcome.Errors, err.Error())
					return
				}

				// Open search engine result page (always done for all actions)
				if browserRouter != nil {
					if _, err := browserRouter.OpenSearchEngineResult(ctx, engine, browserQuery); err != nil {
						engineErr = err
						outcome.Errors = append(outcome.Errors, fmt.Sprintf("browser query open failed for %s: %v", engine, err))
						return
					}
					outcome.OpenedEngines = append(outcome.OpenedEngines, engine)
				} else if screenshotMgr == nil {
					outcome.Errors = append(outcome.Errors, fmt.Sprintf("browser query open skipped for %s: no browser provider", engine))
					engineErr = fmt.Errorf("no browser provider")
					return
				} else if _, err := screenshotMgr.OpenSearchEngineResult(ctx, engine, browserQuery); err != nil {
					engineErr = err
					outcome.Errors = append(outcome.Errors, fmt.Sprintf("browser query open failed for %s: %v", engine, err))
					return
				} else {
					outcome.OpenedEngines = append(outcome.OpenedEngines, engine)
				}

				// Action-specific follow-up
				switch action {
				case "open":
					// Already opened above — nothing more to do.

				case "collect":
					// Collect structured asset data from DOM (no screenshot).
					if browserRouter != nil {
						collected, err := browserRouter.CollectSearchEngineResult(ctx, engine, browserQuery, queryID)
						if err != nil {
							engineErr = err
							outcome.Errors = append(outcome.Errors, fmt.Sprintf("browser collect failed for %s: %v", engine, err))
						} else {
							tagBrowserAssets(collected)
							outcome.CollectedResults = append(outcome.CollectedResults, collected...)
						}
					}

				case "collect_and_capture":
					// Collect structured data from DOM + take evidence screenshot.
					// 优先使用合并方法（单次导航），降级为分步调用。
					if combined, ok := browserRouter.(CombinedBrowserRouter); ok && captureAvailable {
						captureQueryID := queryID
						if captureQueryID == "" {
							captureQueryID = fmt.Sprintf("query_%d", time.Now().UnixNano())
						}
						collected, path, err := combined.CollectAndCaptureSearchEngineResult(ctx, engine, browserQuery, captureQueryID)
						if err != nil {
							engineErr = err
							outcome.Errors = append(outcome.Errors, fmt.Sprintf("browser collect+capture failed for %s: %v", engine, err))
						} else {
							tagBrowserAssets(collected)
							outcome.CollectedResults = append(outcome.CollectedResults, collected...)
							if path != "" && previewURLBuilder != nil {
								if previewURL := previewURLBuilder(path); previewURL != "" {
									outcome.AutoCapturedPaths[engine] = previewURL
								}
							}
						}
					} else {
						// 降级：分步调用
						if browserRouter != nil {
							collected, err := browserRouter.CollectSearchEngineResult(ctx, engine, browserQuery, queryID)
							if err != nil {
								engineErr = err
								outcome.Errors = append(outcome.Errors, fmt.Sprintf("browser collect failed for %s: %v", engine, err))
							} else {
								tagBrowserAssets(collected)
								outcome.CollectedResults = append(outcome.CollectedResults, collected...)
							}
						}
						if captureAvailable {
							captureQueryID := queryID
							if captureQueryID == "" {
								captureQueryID = fmt.Sprintf("query_%d", time.Now().UnixNano())
							}
							path, _, _, _, err := screenshotApp.CaptureSearchEngineResult(ctx, screenshotMgr, engine, browserQuery, captureQueryID)
							if err != nil {
								outcome.AutoCaptureErrors = append(outcome.AutoCaptureErrors, fmt.Sprintf("screenshot failed for %s: %v", engine, err))
							} else if previewURLBuilder != nil {
								if previewURL := previewURLBuilder(path); previewURL != "" {
									outcome.AutoCapturedPaths[engine] = previewURL
								}
							}
						}
					}
				}
			}(engine)
		}
		resultCh <- outcome
	}()

	return resultCh
}

// tagBrowserAssets marks every asset inside collected results as browser-sourced.
func tagBrowserAssets(collected []collection.CollectResult) {
	for i := range collected {
		for j := range collected[i].Assets {
			a := &collected[i].Assets[j]
			if a.Extra == nil {
				a.Extra = make(map[string]interface{})
			}
			a.Extra["collection_method"] = "browser"
		}
	}
}

// BrowserRouter is the minimal interface needed for browser query operations.
type BrowserRouter interface {
	OpenSearchEngineResult(ctx context.Context, engine, query string) (string, error)
	CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]collection.CollectResult, error)
}

// CombinedBrowserRouter extends BrowserRouter with a combined collect+capture operation.
type CombinedBrowserRouter interface {
	BrowserRouter
	CollectAndCaptureSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]collection.CollectResult, string, error)
}

func checkCDPStatus(ctx context.Context, baseURL string) (bool, map[string]interface{}, error) {
	baseURL = normalizeCDPBaseURL(baseURL)
	if baseURL == "" {
		return false, nil, fmt.Errorf("cdp url is empty")
	}

	statusURL := strings.TrimRight(baseURL, "/") + "/json/version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return false, nil, err
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return false, nil, err
	}

	return true, info, nil
}

func normalizeCDPBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	return strings.TrimRight(raw, "/")
}
