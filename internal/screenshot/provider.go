package screenshot

import (
	"context"

	"github.com/unimap/project/internal/model"
)

// CollectResult holds structured data extracted from a search engine result page.
type CollectResult struct {
	Engine        string               `json:"engine"`
	Query         string               `json:"query"`
	RawURL        string               `json:"raw_url"`
	Title         string               `json:"title"`
	Timestamp     int64                `json:"timestamp"`
	Assets        []model.UnifiedAsset `json:"assets,omitempty"`
	Total         int                  `json:"total,omitempty"`
	HasMore       bool                 `json:"has_more,omitempty"`
	IsLoginWall   bool                 `json:"is_login_wall,omitempty"`
	LoginRequired bool                 `json:"login_required,omitempty"`
}

// Provider defines screenshot capabilities used by the app service layer.
type Provider interface {
	CaptureSearchEngineResult(ctx context.Context, engine, query, queryID string) (string, error)
	CaptureTargetWebsite(ctx context.Context, targetURL, ip, port, protocol, queryID string) (string, error)
	CaptureBatchURLs(ctx context.Context, urls []string, batchID string, concurrency int) ([]BatchScreenshotResult, error)
	GetScreenshotDirectory() string
	// OpenSearchEngineResult opens a search engine result page in the browser
	// without capturing a screenshot. Returns the opened URL.
	OpenSearchEngineResult(ctx context.Context, engine, query string) (string, error)
	// CollectSearchEngineResult opens a search engine result page and extracts
	// structured data from the page. Returns collected results.
	CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]CollectResult, error)
}
