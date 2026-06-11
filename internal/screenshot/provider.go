package screenshot

import (
	"context"

	"github.com/unimap/project/internal/collection"
)

// Provider defines screenshot capabilities used by the app service layer.
type Provider interface {
	CaptureSearchEngineResult(ctx context.Context, engine, query, queryID string) (string, error)
	CaptureTargetWebsite(ctx context.Context, targetURL, ip, port, protocol, queryID string) (string, error)
	CaptureBatchURLs(ctx context.Context, urls []string, batchID string, concurrency int) ([]BatchScreenshotResult, error)
	GetScreenshotDirectory() string
	OpenSearchEngineResult(ctx context.Context, engine, query string) (string, error)
	CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]collection.CollectResult, error)
}

// BatchProgressProvider is implemented by providers that can report each batch
// item as it finishes.
type BatchProgressProvider interface {
	CaptureBatchURLsWithProgress(ctx context.Context, urls []string, batchID string, concurrency int, onResult func(BatchScreenshotResult)) ([]BatchScreenshotResult, error)
}
