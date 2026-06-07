package adapter

import (
	"context"
	"fmt"

	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/screenshot"
)

// BrowserQueryBackend provides browser-based result collection for engines
// that don't have API credentials configured.
type BrowserQueryBackend interface {
	CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]screenshot.CollectResult, error)
}

// WebOnlyAdapterBase Web-only 适配器基类
type WebOnlyAdapterBase struct {
	adapter EngineAdapter
	name    string
	backend BrowserQueryBackend
}

// NewWebOnlyAdapterBase 创建 Web-only 适配器基类
func NewWebOnlyAdapterBase(adapter EngineAdapter, name string) *WebOnlyAdapterBase {
	return &WebOnlyAdapterBase{
		adapter: adapter,
		name:    name,
	}
}

// SetBrowserBackend sets the browser query backend for this adapter.
func (w *WebOnlyAdapterBase) SetBrowserBackend(backend BrowserQueryBackend) {
	w.backend = backend
}

// Name 获取引擎名称
func (w *WebOnlyAdapterBase) Name() string {
	return w.name
}

// Translate 翻译查询。Delegate to the underlying adapter which can translate
// UQL without needing API credentials.
func (w *WebOnlyAdapterBase) Translate(ast *model.UQLAST) (string, error) {
	if w.adapter != nil {
		return w.adapter.Translate(ast)
	}
	return "", fmt.Errorf("web-only mode: translation not supported")
}

// Search 搜索。If a browser backend is available, collect results via browser.
func (w *WebOnlyAdapterBase) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if w.backend == nil {
		return nil, fmt.Errorf("web-only mode: no browser backend configured for %s", w.name)
	}

	queryID := fmt.Sprintf("webonly_%s_%d_%d", w.name, page, pageSize)

	results, err := w.backend.CollectSearchEngineResult(ctx, w.name, query, queryID)
	if err != nil {
		return nil, fmt.Errorf("browser collection failed for %s: %w", w.name, err)
	}

	// Build EngineResult from collected assets
	engineResult := &model.EngineResult{
		EngineName: w.name,
		Total:      0,
		Page:       page,
		HasMore:    false,
	}

	var allAssets []model.UnifiedAsset
	for _, r := range results {
		allAssets = append(allAssets, r.Assets...)
		if r.Total > engineResult.Total {
			engineResult.Total = r.Total
		}
		if r.HasMore {
			engineResult.HasMore = true
		}
	}

	engineResult.NormalizedData = allAssets
	return engineResult, nil
}

// Normalize 标准化结果 — delegates to underlying adapter if available.
func (w *WebOnlyAdapterBase) Normalize(result *model.EngineResult) ([]model.UnifiedAsset, error) {
	if w.adapter != nil {
		return w.adapter.Normalize(result)
	}
	if result != nil {
		return result.NormalizedData, nil
	}
	return []model.UnifiedAsset{}, nil
}

// GetQuota 获取配额（Web-only 模式下返回空配额）
func (w *WebOnlyAdapterBase) GetQuota() (*model.QuotaInfo, error) {
	return nil, fmt.Errorf("web-only mode: quota not supported")
}

// IsWebOnly 检查是否为 Web-only 模式
func (w *WebOnlyAdapterBase) IsWebOnly() bool {
	return true
}
