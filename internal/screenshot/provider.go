package screenshot

import "context"

// Provider defines screenshot capabilities used by the app service layer.
type Provider interface {
	CaptureSearchEngineResult(ctx context.Context, engine, query, queryID string) (string, error)
	CaptureTargetWebsite(ctx context.Context, targetURL, ip, port, protocol, queryID string) (string, error)
	CaptureBatchURLs(ctx context.Context, urls []string, batchID string, concurrency int) ([]BatchScreenshotResult, error)
	GetScreenshotDirectory() string
	// OpenSearchEngineResult opens a search engine result page in the browser
	// without capturing a screenshot. Returns the opened URL.
	OpenSearchEngineResult(ctx context.Context, engine, query string) (string, error)
}
