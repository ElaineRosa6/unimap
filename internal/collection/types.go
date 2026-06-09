package collection

import (
	"context"

	"github.com/unimap/project/internal/model"
)

// CollectResult holds structured data extracted from a search engine result page.
type CollectResult struct {
	Engine           string               `json:"engine"`
	Query            string               `json:"query"`
	RawURL           string               `json:"raw_url"`
	Title            string               `json:"title"`
	Timestamp        int64                `json:"timestamp"`
	Assets           []model.UnifiedAsset `json:"assets,omitempty"`
	Total            int                  `json:"total,omitempty"`
	HasMore          bool                 `json:"has_more,omitempty"`
	IsLoginWall      bool                 `json:"is_login_wall,omitempty"`
	LoginRequired    bool                 `json:"login_required,omitempty"`
	ExtractionMethod string               `json:"extraction_method,omitempty"`
	RowSelectorUsed  string               `json:"row_selector_used,omitempty"`
	RowsFound        int                  `json:"rows_found,omitempty"`
	ExtractionError  string               `json:"extraction_error,omitempty"`
}

// Collector defines the capability to collect structured data from search engine pages.
type Collector interface {
	CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]CollectResult, error)
}
